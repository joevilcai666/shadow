package adapter

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joevilcai666/shadow/internal/storage"
)

// CodexAdapter writes rules to AGENTS.md files for OpenAI Codex.
type CodexAdapter struct {
	mb *ManagedBlock
}

// NewCodexAdapter creates a new Codex adapter.
func NewCodexAdapter(backupDir string) *CodexAdapter {
	return &CodexAdapter{mb: NewManagedBlock(backupDir)}
}

func (a *CodexAdapter) Name() string { return "codex" }

func (a *CodexAdapter) IsInstalled() bool {
	// Check for Codex CLI or AGENTS.md presence.
	home, _ := os.UserHomeDir()
	if _, err := os.Stat(filepath.Join(home, ".codex")); err == nil {
		return true
	}
	return false
}

func (a *CodexAdapter) WriteRules(rules []*storage.Rule, scope, projectPath string) error {
	targetPath := a.TargetPath(scope, projectPath)
	entries := rulesToEntries(rules)

	result, err := a.mb.Write(targetPath, entries)
	if err != nil {
		return fmt.Errorf("write AGENTS.md: %w", err)
	}

	slog.Info("wrote rules to AGENTS.md", "path", targetPath, "rules", len(rules), "verified", result.Verified)
	return nil
}

func (a *CodexAdapter) RemoveRules(scope, projectPath string) error {
	return a.mb.Remove(a.TargetPath(scope, projectPath))
}

func (a *CodexAdapter) TargetPath(scope, projectPath string) string {
	if scope == "global" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AGENTS.md")
	}
	return filepath.Join(projectPath, "AGENTS.md")
}
