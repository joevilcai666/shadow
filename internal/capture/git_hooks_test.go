package capture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHooksInstall(t *testing.T) {
	dir := t.TempDir()
	// Create fake git repo structure.
	hooksDir := filepath.Join(dir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	hooks := NewGitHooks(dir, filepath.Join(dir, "shadow.sock"))

	if err := hooks.Install(dir); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Check hooks exist.
	postCheckout := filepath.Join(hooksDir, "post-checkout")
	if _, err := os.Stat(postCheckout); os.IsNotExist(err) {
		t.Fatal("post-checkout hook should exist")
	}

	postRewrite := filepath.Join(hooksDir, "post-rewrite")
	if _, err := os.Stat(postRewrite); os.IsNotExist(err) {
		t.Fatal("post-rewrite hook should exist")
	}

	// Verify hook content has shadow marker.
	data, _ := os.ReadFile(postCheckout)
	if !strings.Contains(string(data), shadowMarker) {
		t.Error("hook should contain shadow marker")
	}
}

func TestGitHooksIdempotent(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	hooks := NewGitHooks(dir, filepath.Join(dir, "shadow.sock"))

	// Install twice.
	hooks.Install(dir)
	hooks.Install(dir)

	data, _ := os.ReadFile(filepath.Join(hooksDir, "post-checkout"))
	// Should only have one shadow marker block.
	count := strings.Count(string(data), shadowMarker)
	if count != 1 {
		t.Errorf("idempotent: expected 1 marker, got %d", count)
	}
}

func TestGitHooksUninstall(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	hooks := NewGitHooks(dir, filepath.Join(dir, "shadow.sock"))
	hooks.Install(dir)
	hooks.Uninstall(dir)

	data, _ := os.ReadFile(filepath.Join(hooksDir, "post-checkout"))
	if strings.Contains(string(data), shadowMarker) {
		t.Error("shadow marker should be removed after uninstall")
	}
}

func TestGitHooksPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)

	// Pre-existing hook content.
	existingHook := "#!/bin/sh\necho 'my custom hook'\n"
	os.WriteFile(filepath.Join(hooksDir, "post-checkout"), []byte(existingHook), 0755)

	hooks := NewGitHooks(dir, filepath.Join(dir, "shadow.sock"))
	hooks.Install(dir)

	data, _ := os.ReadFile(filepath.Join(hooksDir, "post-checkout"))
	content := string(data)
	if !strings.Contains(content, "my custom hook") {
		t.Error("existing hook content should be preserved")
	}
	if !strings.Contains(content, shadowMarker) {
		t.Error("shadow hook should be appended")
	}
}

func TestGitHooksNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	hooks := NewGitHooks(dir, filepath.Join(dir, "shadow.sock"))
	err := hooks.Install(dir)
	if err == nil {
		t.Error("should error when not a git repo")
	}
}
