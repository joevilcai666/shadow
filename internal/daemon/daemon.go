package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/joevilcai666/shadow/internal/adapter"
	"github.com/joevilcai666/shadow/internal/capture"
	"github.com/joevilcai666/shadow/internal/config"
	"github.com/joevilcai666/shadow/internal/distill"
	apiserver "github.com/joevilcai666/shadow/internal/server"
	"github.com/joevilcai666/shadow/internal/storage"
)

// State represents the daemon's current state in its lifecycle.
type State string

const (
	StateIdle       State = "idle"
	StateCapturing  State = "capturing"
	StateDistilling State = "distilling"
	StateWriting    State = "writing"
	StateStopping   State = "stopping"
)

// Daemon is the core background service for Shadow.
type Daemon struct {
	mu       sync.RWMutex
	state    State
	version  string
	started  time.Time
	sockPath string
	pidPath  string
	logDir   string
	homeDir  string

	sockServer    *SocketServer
	httpServer    *http.Server
	db            *sql.DB
	captureEngine *capture.Engine
	distillEngine *distill.Engine
	adapters      []adapter.Adapter
	configMgr     *config.Manager

	cancel context.CancelFunc
	done   chan struct{}
}

// Config holds daemon configuration.
type Config struct {
	Version string
	HomeDir string // defaults to ~/.shadow
}

// New creates a new Daemon instance.
func New(cfg Config) (*Daemon, error) {
	home, err := resolveHome(cfg.HomeDir)
	if err != nil {
		return nil, err
	}

	d := &Daemon{
		state:    StateIdle,
		version:  cfg.Version,
		started:  time.Now().UTC(),
		sockPath: filepath.Join(home, "shadow.sock"),
		pidPath:  filepath.Join(home, "shadow.pid"),
		logDir:   filepath.Join(home, "logs"),
		homeDir:  home,
		done:     make(chan struct{}),
	}

	return d, nil
}

// Run starts the daemon and blocks until shutdown. This is the main entry point.
func (d *Daemon) Run(ctx context.Context) error {
	ctx, d.cancel = context.WithCancel(ctx)
	defer d.cancel()

	// Acquire single-instance lock.
	release, err := d.acquireLock()
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer release()

	// Ensure directories.
	if err := os.MkdirAll(d.logDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	// Write PID file.
	if err := d.writePID(); err != nil {
		return fmt.Errorf("write pid: %w", err)
	}
	defer os.Remove(d.pidPath)

	// Start IPC socket server.
	d.sockServer = NewSocketServer(d.sockPath, d)
	if err := d.sockServer.Start(); err != nil {
		return fmt.Errorf("start socket server: %w", err)
	}
	defer d.sockServer.Close()

	// Open database.
	dbPath := filepath.Join(d.homeDir, "shadow.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	d.db = db

	// Load config.
	cfgMgr := config.NewManager(d.homeDir)
	cfgMgr.LoadGlobal()
	d.configMgr = cfgMgr

	// Start HTTP API server.
	ruleRepo := storage.NewRuleRepo(db)
	mcpServer := adapter.NewMCPServer(ruleRepo)

	httpSrv := apiserver.New(
		ruleRepo,
		storage.NewSourceRepo(db),
		storage.NewEventRepo(db),
		storage.NewVersionRepo(db),
		storage.NewConfigRepo(db),
		storage.NewProjectRepo(db),
		cfgMgr,
		config.ServerConfig{Port: 7878, Bind: "127.0.0.1"},
		mcpServer,
	)
	httpSrv.SetControlHooks(
		func() error { return d.ToggleCapture(ctx, cfgMgr) },
		func() error {
			d.SyncAdapters()
			return nil
		},
	)

	d.httpServer = &http.Server{
		Addr:    "127.0.0.1:7878",
		Handler: httpSrv,
	}
	go func() {
		if err := d.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	// Start capture engine (reuses ruleRepo from HTTP server setup).
	sourceRepo := storage.NewSourceRepo(db)
	d.captureEngine = capture.NewEngine(cfgMgr, sourceRepo, ruleRepo, d.homeDir)
	d.captureEngine.RegisterParser(capture.NewClaudeCodeParser())
	d.captureEngine.RegisterParser(capture.NewCodexParser())
	d.captureEngine.RegisterParser(capture.NewCursorParser())

	// Start distill engine â use LLM if API key configured, else rule-based.
	var distiller distill.Distiller
	if apiKey := cfgMgr.Get().Distill.LLMAPIKey; apiKey != "" {
		slog.Info("using LLM distiller", "model", cfgMgr.Get().Distill.LLMModel)
		distiller = distill.NewLLMDistiller(apiKey, cfgMgr.Get().Distill.LLMModel)
	} else {
		slog.Info("using rule-based distiller (no LLM API key configured)")
		distiller = distill.NewRuleBasedDistiller()
	}
	d.distillEngine = distill.NewEngine(distiller, ruleRepo, sourceRepo, cfgMgr.Get().Distill.Threshold)

	if cfgMgr.Get().Capture.Enabled {
		if err := d.captureEngine.Start(ctx); err != nil {
			slog.Warn("capture engine start failed (non-fatal)", "error", err)
		} else {
			d.SetState(StateCapturing)
			slog.Info("capture engine started")
		}
	}

	// Initialize adapters for writing rules to agent context files.
	backupDir := filepath.Join(d.homeDir, "backups")
	d.adapters = []adapter.Adapter{
		adapter.NewClaudeCodeAdapter(backupDir),
		adapter.NewCursorAdapter(backupDir),
		adapter.NewCodexAdapter(backupDir),
		adapter.NewCopilotAdapter(backupDir),
	}
	slog.Info("adapters initialized", "count", len(d.adapters))

	// Start periodic distill loop: processes accumulated signals → candidate rules.
	go d.distillLoop(ctx, d.distillEngine, cfgMgr)

	// Start adapter sync loop: writes active rules to agent context files.
	go d.adapterSyncLoop(ctx)

	slog.Info("shadow daemon started", "version", d.version, "socket", d.sockPath, "http", "localhost:7878")

	// Handle signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	for {
		select {
		case <-ctx.Done():
			slog.Info("daemon shutting down via context")
			return d.shutdown()
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT:
				slog.Info("received shutdown signal", "signal", sig)
				return d.shutdown()
			case syscall.SIGHUP:
				slog.Info("received reload signal")
				d.handleReload()
			}
		case <-d.done:
			return nil
		}
	}
}

// Stop gracefully stops the daemon via IPC.
func (d *Daemon) Stop() {
	d.mu.Lock()
	d.state = StateStopping
	d.mu.Unlock()

	if d.cancel != nil {
		d.cancel()
	}
	close(d.done)
}

// Status returns the current daemon status.
func (d *Daemon) Status() StatusResponse {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return StatusResponse{
		State:     string(d.state),
		Version:   d.version,
		Uptime:    time.Since(d.started).String(),
		StartedAt: d.started.Format(time.RFC3339),
		Socket:    d.sockPath,
		PID:       os.Getpid(),
	}
}

// SetState transitions the daemon to a new state.
func (d *Daemon) SetState(s State) {
	d.mu.Lock()
	defer d.mu.Unlock()
	slog.Info("state transition", "from", d.state, "to", s)
	d.state = s
}

// GetState returns the current state.
func (d *Daemon) GetState() State {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.state
}

func (d *Daemon) shutdown() error {
	slog.Info("daemon shutting down gracefully")

	d.SetState(StateStopping)

	// Shutdown HTTP server.
	if d.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		d.httpServer.Shutdown(ctx)
	}

	// Give ongoing operations up to 10s to finish.
	timeout := time.After(10 * time.Second)
	done := make(chan struct{})

	go func() {
		// Stop capture engine.
		if d.captureEngine != nil {
			d.captureEngine.Stop()
			slog.Info("capture engine stopped")
		}
		close(done)
	}()

	select {
	case <-done:
		slog.Info("all operations completed")
	case <-timeout:
		slog.Warn("shutdown timeout, forcing exit")
	}

	slog.Info("shadow daemon stopped")
	return nil
}

func (d *Daemon) handleReload() {
	slog.Info("reloading configuration")
	// TODO: SHADOW-004 — reload config from disk
}

func (d *Daemon) writePID() error {
	return os.WriteFile(d.pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
}

// StatusResponse is the JSON response for status queries.
type StatusResponse struct {
	State     string `json:"state"`
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
	StartedAt string `json:"started_at"`
	Socket    string `json:"socket"`
	PID       int    `json:"pid"`
}

// IPCRequest represents an incoming IPC command.
type IPCRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// IPCResponse represents an IPC response.
type IPCResponse struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// HandleCommand processes an IPC command and returns a response.
func (d *Daemon) HandleCommand(req IPCRequest) IPCResponse {
	switch req.Method {
	case "status":
		return IPCResponse{Result: d.Status()}
	case "stop":
		go func() {
			time.Sleep(100 * time.Millisecond)
			d.Stop()
		}()
		return IPCResponse{Result: "stopping"}
	case "version":
		return IPCResponse{Result: map[string]string{"version": d.version}}
	case "state":
		return IPCResponse{Result: map[string]string{"state": string(d.GetState())}}
	case "capture.toggle":
		d.mu.Lock()
		if d.state == StateCapturing {
			d.state = StateIdle
		} else if d.state == StateIdle {
			d.state = StateCapturing
		}
		newState := d.state
		d.mu.Unlock()
		return IPCResponse{Result: map[string]string{"state": string(newState)}}
	default:
		return IPCResponse{Error: fmt.Sprintf("unknown method: %s", req.Method)}
	}
}

func resolveHome(homeDir string) (string, error) {
	if homeDir != "" {
		return homeDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".shadow"), nil
}

// distillLoop runs periodically to process accumulated signals into candidate rules.
func (d *Daemon) distillLoop(ctx context.Context, eng *distill.Engine, cfgMgr *config.Manager) {
	// Run every 60 seconds.
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	// Process immediately on startup for any backlog.
	d.runDistillCycle(eng)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.runDistillCycle(eng)
		}
	}
}

func (d *Daemon) runDistillCycle(eng *distill.Engine) {
	if d.db == nil {
		return
	}

	_ = storage.NewSourceRepo(d.db) // available for future use

	// Find unlinked sources (no rule_id) — these haven't been distilled yet.
	rows, err := d.db.Query(`
		SELECT id, signal_type, signal_strength, COALESCE(agent_name,''),
		       COALESCE(project_path,''), COALESCE(raw_snippet,''), timestamp, confidence_contribution
		FROM sources WHERE rule_id = '' OR rule_id IS NULL
		ORDER BY timestamp ASC LIMIT 50`)
	if err != nil {
		slog.Debug("distill: query unlinked sources", "error", err)
		return
	}
	defer rows.Close()

	var sources []*storage.Source
	for rows.Next() {
		s, err := scanSourceRow(rows)
		if err != nil {
			continue
		}
		sources = append(sources, s)
	}

	if len(sources) == 0 {
		return
	}

	slog.Info("distill: processing unlinked sources", "count", len(sources))

	// Group sources by similar content for batch distillation.
	// For simplicity, process strong signals individually and batch weak ones.
	var strongSignals []*storage.Source
	var weakSignals []*storage.Source

	for _, s := range sources {
		if s.SignalStrength == "strong" || s.SignalType == "manual_mark" {
			strongSignals = append(strongSignals, s)
		} else {
			weakSignals = append(weakSignals, s)
		}
	}

	// Process strong signals immediately.
	for _, s := range strongSignals {
		if err := eng.ProcessExplicitSignal(s); err != nil {
			slog.Warn("distill: process explicit signal", "error", err)
		}
	}

	// Process weak signals in batch if enough accumulated.
	if len(weakSignals) > 0 {
		if err := eng.ProcessSignals(weakSignals); err != nil {
			slog.Warn("distill: process signals batch", "error", err)
		}
	}

	d.SetState(StateDistilling)
	// Brief state to show activity, then back to capturing.
	time.AfterFunc(2*time.Second, func() { d.SetState(StateCapturing) })
}

// adapterSyncLoop periodically writes active rules to agent context files.
func (d *Daemon) adapterSyncLoop(ctx context.Context) {
	// Run every 2 minutes.
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	// Sync immediately on startup.
	d.syncAdapters()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.syncAdapters()
		}
	}
}

// SyncAdapters writes all active rules to connected agent context files.
func (d *Daemon) SyncAdapters() {
	d.syncAdapters()
}

// ToggleCapture flips capture on/off and persists the setting.
func (d *Daemon) ToggleCapture(ctx context.Context, cfgMgr *config.Manager) error {
	var enabled bool
	if err := cfgMgr.UpdateGlobal(func(cfg *config.Config) {
		cfg.Capture.Enabled = !cfg.Capture.Enabled
		enabled = cfg.Capture.Enabled
	}); err != nil {
		return err
	}

	if enabled {
		if d.captureEngine != nil && d.GetState() != StateCapturing {
			if err := d.captureEngine.Start(ctx); err != nil {
				return err
			}
		}
		d.SetState(StateCapturing)
		return nil
	}

	if d.captureEngine != nil {
		d.captureEngine.Stop()
	}
	d.SetState(StateIdle)
	return nil
}

func (d *Daemon) syncAdapters() {
	if d.db == nil || len(d.adapters) == 0 {
		return
	}

	// Don't sync if daemon is stopping.
	if d.GetState() == StateStopping {
		return
	}

	ruleRepo := storage.NewRuleRepo(d.db)
	eventRepo := storage.NewEventRepo(d.db)

	// Get all active rules.
	globalRules, err := ruleRepo.List(storage.RuleFilter{Status: "active", Scope: "global"})
	if err != nil {
		slog.Warn("adapter sync: fetch global rules", "error", err)
		return
	}

	// Get all projects.
	projectRepo := storage.NewProjectRepo(d.db)
	projects, err := projectRepo.List()
	if err != nil {
		slog.Warn("adapter sync: fetch projects", "error", err)
		return
	}

	// Don't sync if daemon is stopping.
	if d.GetState() == StateStopping {
		return
	}

	d.SetState(StateWriting)

	cfg := (*config.Config)(nil)
	if d.configMgr != nil {
		cfg = d.configMgr.Get()
	}

	for _, a := range d.adapters {
		if cfg != nil && !config.AdapterEnabled(cfg, a.Name()) {
			d.removeAdapterBlocks(a, projects)
			continue
		}
		if !a.IsInstalled() {
			continue
		}

		// Write global rules.
		if len(globalRules) > 0 {
			if err := a.WriteRules(globalRules, "global", ""); err != nil {
				slog.Warn("adapter sync: write global rules", "adapter", a.Name(), "error", err)
				d.recordSyncEvent(eventRepo, a, "global", "", "sync_failure", err.Error())
			} else {
				slog.Debug("adapter sync: wrote global rules", "adapter", a.Name(), "count", len(globalRules))
				d.recordSyncEvent(eventRepo, a, "global", "", "sync_success", fmt.Sprintf("wrote %d active global rule(s)", len(globalRules)))
			}
		}

		// Write project-specific rules.
		for _, p := range projects {
			projectRules, err := ruleRepo.List(storage.RuleFilter{
				Status:      "active",
				Scope:       "project",
				ProjectPath: p.Path,
			})
			if err != nil {
				continue
			}
			if len(projectRules) > 0 {
				if err := a.WriteRules(projectRules, "project", p.Path); err != nil {
					slog.Warn("adapter sync: write project rules", "adapter", a.Name(), "project", p.Name, "error", err)
					d.recordSyncEvent(eventRepo, a, "project", p.Path, "sync_failure", err.Error())
				} else {
					d.recordSyncEvent(eventRepo, a, "project", p.Path, "sync_success", fmt.Sprintf("wrote %d active project rule(s)", len(projectRules)))
				}
			}
		}
	}

	// Brief state indicator, then back to capturing (if not stopping).
	time.AfterFunc(2*time.Second, func() {
		if d.GetState() != StateStopping {
			d.SetState(StateCapturing)
		}
	})
}

func (d *Daemon) removeAdapterBlocks(a adapter.Adapter, projects []*storage.Project) {
	if err := a.RemoveRules("global", ""); err != nil {
		slog.Warn("adapter sync: remove global rules", "adapter", a.Name(), "error", err)
		d.recordSyncEvent(storage.NewEventRepo(d.db), a, "global", "", "sync_failure", err.Error())
	} else {
		d.recordSyncEvent(storage.NewEventRepo(d.db), a, "global", "", "sync_success", "removed managed block for disabled adapter")
	}
	for _, p := range projects {
		if err := a.RemoveRules("project", p.Path); err != nil {
			slog.Warn("adapter sync: remove project rules", "adapter", a.Name(), "project", p.Name, "error", err)
			d.recordSyncEvent(storage.NewEventRepo(d.db), a, "project", p.Path, "sync_failure", err.Error())
		} else {
			d.recordSyncEvent(storage.NewEventRepo(d.db), a, "project", p.Path, "sync_success", "removed managed block for disabled adapter")
		}
	}
}

func (d *Daemon) recordSyncEvent(eventRepo *storage.EventRepo, a adapter.Adapter, scope, projectPath, eventType, details string) {
	if eventRepo == nil || d.db == nil {
		return
	}
	if err := eventRepo.Create(&storage.Event{
		ID:          storage.NewID(),
		EventType:   eventType,
		AgentName:   a.Name(),
		ProjectPath: projectPath,
		TargetPath:  a.TargetPath(scope, projectPath),
		Details:     details,
		Timestamp:   storage.Now(),
	}); err != nil {
		slog.Warn("adapter sync: record event", "adapter", a.Name(), "error", err)
	}
}

// scanSourceRow scans a source from a database row.
func scanSourceRow(rows *sql.Rows) (*storage.Source, error) {
	var s storage.Source
	err := rows.Scan(
		&s.ID, &s.SignalType, &s.SignalStrength,
		&s.AgentName, &s.ProjectPath, &s.RawSnippet,
		&s.Timestamp, &s.ConfidenceContribution,
	)
	return &s, err
}
