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

	sockServer   *SocketServer
	httpServer   *http.Server
	db           *sql.DB
	captureEngine *capture.Engine
	distillEngine *distill.Engine

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

	// Start HTTP API server.
	ruleRepo := storage.NewRuleRepo(db)
	mcpServer := adapter.NewMCPServer(ruleRepo)

	httpSrv := apiserver.New(
		ruleRepo,
		storage.NewSourceRepo(db),
		storage.NewVersionRepo(db),
		storage.NewConfigRepo(db),
		storage.NewProjectRepo(db),
		cfgMgr,
		config.ServerConfig{Port: 7878, Bind: "127.0.0.1"},
		mcpServer,
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
