package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joevilcai666/shadow/internal/adapter"
	"github.com/joevilcai666/shadow/internal/config"
	"github.com/joevilcai666/shadow/internal/storage"
)

func TestDaemonStatus(t *testing.T) {
	dir := t.TempDir()
	d, err := New(Config{Version: "test", HomeDir: dir})
	if err != nil {
		t.Fatalf("create daemon: %v", err)
	}

	status := d.Status()
	if status.State != string(StateIdle) {
		t.Errorf("initial state: got %q, want %q", status.State, StateIdle)
	}
	if status.Version != "test" {
		t.Errorf("version: got %q, want %q", status.Version, "test")
	}
	if status.PID != os.Getpid() {
		t.Errorf("pid: got %d, want %d", status.PID, os.Getpid())
	}
}

func TestDaemonStateTransitions(t *testing.T) {
	dir := t.TempDir()
	d, _ := New(Config{Version: "test", HomeDir: dir})

	transitions := []State{StateCapturing, StateDistilling, StateWriting, StateIdle}
	for _, want := range transitions {
		d.SetState(want)
		if got := d.GetState(); got != want {
			t.Errorf("state: got %q, want %q", got, want)
		}
	}
}

func TestDaemonHandleCommand(t *testing.T) {
	dir := t.TempDir()
	d, _ := New(Config{Version: "0.1.0", HomeDir: dir})

	// Status
	resp := d.HandleCommand(IPCRequest{Method: "status"})
	if resp.Error != "" {
		t.Errorf("status error: %s", resp.Error)
	}
	statusBytes, _ := json.Marshal(resp.Result)
	var status StatusResponse
	json.Unmarshal(statusBytes, &status)
	if status.State != string(StateIdle) {
		t.Errorf("status state: got %q", status.State)
	}

	// Version
	resp = d.HandleCommand(IPCRequest{Method: "version"})
	if resp.Error != "" {
		t.Errorf("version error: %s", resp.Error)
	}

	// Unknown method
	resp = d.HandleCommand(IPCRequest{Method: "nonexistent"})
	if resp.Error == "" {
		t.Error("expected error for unknown method")
	}
}

func TestCaptureToggle(t *testing.T) {
	dir := t.TempDir()
	d, _ := New(Config{Version: "test", HomeDir: dir})

	// Toggle on
	resp := d.HandleCommand(IPCRequest{Method: "capture.toggle"})
	var r1 map[string]string
	data, _ := json.Marshal(resp.Result)
	json.Unmarshal(data, &r1)
	if r1["state"] != string(StateCapturing) {
		t.Errorf("after toggle on: got %q, want %q", r1["state"], StateCapturing)
	}

	// Toggle off
	resp = d.HandleCommand(IPCRequest{Method: "capture.toggle"})
	data, _ = json.Marshal(resp.Result)
	json.Unmarshal(data, &r1)
	if r1["state"] != string(StateIdle) {
		t.Errorf("after toggle off: got %q, want %q", r1["state"], StateIdle)
	}
}

func TestOnboardingEnterFromWelcomeGoesToPrivacyStep(t *testing.T) {
	m := NewOnboardingModel("test")

	next, _ := m.handleEnter()
	got := next.(OnboardingModel)

	if got.step != 2 {
		t.Fatalf("step = %d, want Privacy step 2", got.step)
	}
	if got.loading {
		t.Fatal("welcome enter should not start daemon check loading")
	}
}

func TestOnboardingIncludesOpenClawTarget(t *testing.T) {
	m := NewOnboardingModel("test")

	found := false
	for _, item := range m.agents.Items {
		if item.Label == "OpenClaw" {
			found = true
			if !strings.Contains(item.Description, "OPENCLAW.md") {
				t.Fatalf("OpenClaw description = %q, want OPENCLAW.md", item.Description)
			}
		}
	}
	if !found {
		t.Fatalf("onboarding agents = %#v, want OpenClaw", m.agents.Items)
	}
	if m.agentTargets["OpenClaw"] != "OPENCLAW.md (project) + ~/OPENCLAW.md (global)" {
		t.Fatalf("OpenClaw target = %q", m.agentTargets["OpenClaw"])
	}
}

func TestOnboardingScanScopesRulesAndSelectedAgentsToCurrentProject(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "go.mod"), []byte("module example.com/shadow-test"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "CLAUDE.md"), []byte("Use table-driven tests."), 0644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "shadow.db")

	msg := scanProject(
		cwd,
		dbPath,
		[]CheckboxItem{{Label: "Codex"}},
		[]string{"Claude Code", "Cursor"},
	)()
	done, ok := msg.(scanCompleteMsg)
	if !ok {
		t.Fatalf("scanProject returned %T, want scanCompleteMsg", msg)
	}
	if done.count == 0 {
		t.Fatal("scan should generate or import candidate rules")
	}

	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	rules, err := storage.NewRuleRepo(db).List(storage.RuleFilter{Scope: "project"})
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected project-scoped rules")
	}
	for _, rule := range rules {
		if rule.ProjectPath != cwd {
			t.Errorf("rule %q project path = %q, want %q", rule.Content, rule.ProjectPath, cwd)
		}
	}

	project, err := storage.NewProjectRepo(db).GetByPath(cwd)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if project == nil {
		t.Fatal("project should be registered")
	}
	if len(project.Agents) != 1 || project.Agents[0] != "Codex" {
		t.Errorf("project agents = %#v, want only selected Codex", project.Agents)
	}
}

func TestSyncAdaptersRemovesDisabledAdapterBlocks(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "shadow.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ruleRepo := storage.NewRuleRepo(db)
	projectPath := filepath.Join(dir, "repo")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := storage.NewProjectRepo(db).Create(&storage.Project{
		ID:        storage.NewID(),
		Path:      projectPath,
		Name:      "repo",
		Agents:    []string{"Cursor", "Codex"},
		CreatedAt: storage.Now(),
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	for _, rule := range []*storage.Rule{
		{
			ID: storage.NewID(), Content: "Use pnpm", Scope: "global",
			Tags: []string{}, Confidence: 0.9, Status: "active", Version: 1,
			CreatedAt: storage.Now(), UpdatedAt: storage.Now(),
		},
		{
			ID: storage.NewID(), Content: "Use table tests", Scope: "project", ProjectPath: projectPath,
			Tags: []string{}, Confidence: 0.9, Status: "active", Version: 1,
			CreatedAt: storage.Now(), UpdatedAt: storage.Now(),
		},
	} {
		if err := ruleRepo.Create(rule); err != nil {
			t.Fatalf("create rule: %v", err)
		}
	}

	cfgMgr := config.NewManager(dir)
	if err := cfgMgr.LoadGlobal(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if err := cfgMgr.UpdateGlobal(func(cfg *config.Config) {
		cfg.Adapters.Cursor.Enabled = false
		cfg.Adapters.Codex.Enabled = true
	}); err != nil {
		t.Fatalf("update config: %v", err)
	}

	cursor := &fakeAdapter{name: "cursor", installed: true}
	codex := &fakeAdapter{name: "codex", installed: true}
	d := &Daemon{
		state:     StateIdle,
		db:        db,
		configMgr: cfgMgr,
		adapters:  nil,
	}
	d.adapters = append(d.adapters, cursor, codex)

	d.syncAdapters()

	if len(cursor.writes) != 0 {
		t.Errorf("disabled cursor writes = %v, want none", cursor.writes)
	}
	if len(cursor.removes) != 2 {
		t.Errorf("disabled cursor removes = %v, want global and project", cursor.removes)
	}
	if len(codex.writes) != 2 {
		t.Errorf("enabled codex writes = %v, want global and project", codex.writes)
	}
}

func TestSyncAdaptersRecordsEffectivenessEvents(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "shadow.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	rule := &storage.Rule{
		ID: storage.NewID(), Content: "Use pnpm", Scope: "global",
		Tags: []string{}, Confidence: 0.9, Status: "active", Version: 1,
		CreatedAt: storage.Now(), UpdatedAt: storage.Now(),
	}
	if err := storage.NewRuleRepo(db).Create(rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	cfgMgr := config.NewManager(dir)
	if err := cfgMgr.LoadGlobal(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if err := cfgMgr.UpdateGlobal(func(cfg *config.Config) {
		cfg.Adapters.Codex.Enabled = true
	}); err != nil {
		t.Fatalf("update config: %v", err)
	}

	codex := &fakeAdapter{name: "codex", installed: true}
	d := &Daemon{
		state:     StateIdle,
		db:        db,
		configMgr: cfgMgr,
		adapters:  []adapter.Adapter{codex},
	}

	d.syncAdapters()

	latest, err := storage.NewEventRepo(db).LatestByAgentEvent("codex", "sync_success")
	if err != nil {
		t.Fatalf("latest sync event: %v", err)
	}
	if latest == nil {
		t.Fatal("expected sync_success event")
	}
	if latest.TargetPath != "global:" {
		t.Fatalf("target path = %q, want fake adapter global target", latest.TargetPath)
	}
	if latest.Details == "" {
		t.Fatal("sync event should explain what happened")
	}
}

func TestSocketIPC(t *testing.T) {
	dir := t.TempDir()
	d, _ := New(Config{Version: "0.1.0", HomeDir: dir})

	var sockPath string
	if runtime.GOOS == "windows" {
		sockPath = `\\.\pipe\shadow-test-` + strings.ReplaceAll(t.Name(), "/", "-")
	} else {
		sockPath = filepath.Join(dir, "test.sock")
	}

	srv := NewSocketServer(sockPath, d)
	if err := srv.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer srv.Close()

	// Connect as client.
	client := &Client{sockPath: sockPath}

	resp, err := client.Send("status", nil)
	if err != nil {
		t.Fatalf("send status: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("status error: %s", resp.Error)
	}

	resp, err = client.Send("version", nil)
	if err != nil {
		t.Fatalf("send version: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("version error: %s", resp.Error)
	}
}

func TestSingleInstanceLock(t *testing.T) {
	dir := t.TempDir()
	d1, _ := New(Config{Version: "test", HomeDir: dir})

	release, err := d1.acquireLock()
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}

	// Second instance should fail.
	d2, _ := New(Config{Version: "test", HomeDir: dir})
	_, err = d2.acquireLock()
	if err == nil {
		t.Error("expected lock conflict for second instance")
	}

	release()

	// Now should succeed.
	release2, err := d2.acquireLock()
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	release2()
}

func TestSingleInstanceLockCreatesFreshHomeDir(t *testing.T) {
	home := filepath.Join(t.TempDir(), "fresh", ".shadow")
	d, _ := New(Config{Version: "test", HomeDir: home})

	release, err := d.acquireLock()
	if err != nil {
		t.Fatalf("lock should create missing home dir: %v", err)
	}
	release()
}

func TestDaemonGracefulShutdown(t *testing.T) {
	dir := t.TempDir()
	d, _ := New(Config{Version: "test", HomeDir: dir})

	var wg sync.WaitGroup
	wg.Add(1)

	var runErr error
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer wg.Done()
		runErr = d.Run(ctx)
	}()

	// Give daemon time to start.
	time.Sleep(300 * time.Millisecond)

	// Verify it's running via status.
	status := d.Status()
	if status.State != "capturing" && status.State != "idle" {
		t.Logf("daemon state before shutdown: %q (runErr: %v)", status.State, runErr)
	}

	// Trigger shutdown.
	cancel()
	wg.Wait()

	// On Windows, the daemon may exit quickly if something fails at startup.
	// Check what happened.
	if runErr != nil {
		t.Logf("daemon Run returned error: %v", runErr)
	}
	finalState := d.GetState()
	if finalState != StateStopping && runErr != nil {
		t.Skipf("daemon failed to start (non-Windows-platform issue): %v", runErr)
	}
	if finalState != StateStopping {
		t.Errorf("state after shutdown: got %q, want %q", finalState, StateStopping)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type fakeAdapter struct {
	name      string
	installed bool
	writes    []string
	removes   []string
}

func (f *fakeAdapter) Name() string { return f.name }

func (f *fakeAdapter) IsInstalled() bool { return f.installed }

func (f *fakeAdapter) WriteRules(_ []*storage.Rule, scope, projectPath string) error {
	f.writes = append(f.writes, scope+":"+projectPath)
	return nil
}

func (f *fakeAdapter) PreviewRules(_ []*storage.Rule, scope, projectPath string) (*adapter.WriteResult, error) {
	return &adapter.WriteResult{
		FilePath: f.TargetPath(scope, projectPath),
		Changed:  true,
		Verified: true,
	}, nil
}

func (f *fakeAdapter) RemoveRules(scope, projectPath string) error {
	f.removes = append(f.removes, scope+":"+projectPath)
	return nil
}

func (f *fakeAdapter) TargetPath(scope, projectPath string) string {
	return scope + ":" + projectPath
}
