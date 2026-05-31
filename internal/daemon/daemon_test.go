package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
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

func TestSocketIPC(t *testing.T) {
	dir := t.TempDir()
	d, _ := New(Config{Version: "0.1.0", HomeDir: dir})
	sockPath := filepath.Join(dir, "test.sock")

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

func TestDaemonGracefulShutdown(t *testing.T) {
	dir := t.TempDir()
	d, _ := New(Config{Version: "test", HomeDir: dir})

	var wg sync.WaitGroup
	wg.Add(1)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer wg.Done()
		d.Run(ctx)
	}()

	// Give daemon time to start.
	time.Sleep(100 * time.Millisecond)

	// Verify it's running via status.
	status := d.Status()
	if status.State == "" {
		t.Error("daemon should be running")
	}

	// Trigger shutdown.
	cancel()
	wg.Wait()

	if d.GetState() != StateStopping {
		t.Errorf("state after shutdown: got %q, want %q", d.GetState(), StateStopping)
	}
}

func TestLaunchdPlistGeneration(t *testing.T) {
	dir := t.TempDir()
	cfg := LaunchdConfig{
		Label:      "com.shadow.test",
		BinaryPath: "/usr/local/bin/shadow",
		LogDir:     filepath.Join(dir, "logs"),
		HomeDir:    dir,
	}

	if err := InstallLaunchd(cfg); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Note: plist goes to ~/Library/LaunchAgents, not temp dir.
	plistPath := LaunchdPlistPath("com.shadow.test")
	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("read plist: %v", err)
	}

	content := string(data)
	if !contains(content, "com.shadow.test") {
		t.Error("plist missing label")
	}
	if !contains(content, "/usr/local/bin/shadow") {
		t.Error("plist missing binary path")
	}
	if !contains(content, "KeepAlive") {
		t.Error("plist missing KeepAlive")
	}
	if !contains(content, "RunAtLoad") {
		t.Error("plist missing RunAtLoad")
	}

	// Clean up.
	UninstallLaunchd("com.shadow.test")
	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Error("plist should be removed after uninstall")
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
