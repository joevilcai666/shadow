//go:build darwin

package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

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
