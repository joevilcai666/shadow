package adapter

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joevilcai666/shadow/internal/storage"
)

// Adapter writes Shadow rules into an agent's native context files.
type Adapter interface {
	Name() string
	IsInstalled() bool
	WriteRules(rules []*storage.Rule, scope, projectPath string) error
	RemoveRules(scope, projectPath string) error
	TargetPath(scope, projectPath string) string
}

// --- Claude Code Adapter ---

// ClaudeCodeAdapter writes rules to CLAUDE.md files.
type ClaudeCodeAdapter struct {
	mb      *ManagedBlock
	homeDir string
}

// NewClaudeCodeAdapter creates a new Claude Code adapter.
func NewClaudeCodeAdapter(backupDir string) *ClaudeCodeAdapter {
	home, _ := os.UserHomeDir()
	return &ClaudeCodeAdapter{
		mb:      NewManagedBlock(backupDir),
		homeDir: home,
	}
}

func (a *ClaudeCodeAdapter) Name() string { return "claude_code" }

func (a *ClaudeCodeAdapter) IsInstalled() bool {
	claudeDir := filepath.Join(a.homeDir, ".claude")
	_, err := os.Stat(claudeDir)
	return err == nil
}

func (a *ClaudeCodeAdapter) WriteRules(rules []*storage.Rule, scope, projectPath string) error {
	targetPath := a.TargetPath(scope, projectPath)

	entries := rulesToEntries(rules)

	result, err := a.mb.Write(targetPath, entries)
	if err != nil {
		return fmt.Errorf("write CLAUDE.md: %w", err)
	}

	slog.Info("wrote rules to CLAUDE.md",
		"path", targetPath,
		"rules", len(rules),
		"verified", result.Verified,
	)
	return nil
}

func (a *ClaudeCodeAdapter) RemoveRules(scope, projectPath string) error {
	targetPath := a.TargetPath(scope, projectPath)
	return a.mb.Remove(targetPath)
}

func (a *ClaudeCodeAdapter) TargetPath(scope, projectPath string) string {
	if scope == "global" {
		return filepath.Join(a.homeDir, ".claude", "CLAUDE.md")
	}
	return filepath.Join(projectPath, "CLAUDE.md")
}

// --- Cursor Adapter ---

// CursorAdapter writes rules to .cursorrules files.
type CursorAdapter struct {
	mb      *ManagedBlock
	homeDir string
}

// NewCursorAdapter creates a new Cursor adapter.
func NewCursorAdapter(backupDir string) *CursorAdapter {
	home, _ := os.UserHomeDir()
	return &CursorAdapter{
		mb:      NewManagedBlock(backupDir),
		homeDir: home,
	}
}

func (a *CursorAdapter) Name() string { return "cursor" }

// IsInstalled reports whether a Cursor install can be found on this
// machine. The check is platform-aware:
//   - macOS: /Applications/Cursor.app
//   - Linux: ~/.local/share/cursor (Cursor's XDG_STATE_HOME)
//   - Windows: %LOCALAPPDATA%\Programs\Cursor
// The legacy "always-installed" Cursor that wrote ai/ JSONL under
// ~/Library/Application Support/Cursor is still detected by the
// cursor.go parser's path probe, so users with the older install get
// capture either way.
func (a *CursorAdapter) IsInstalled() bool {
	candidates := []string{
		"/Applications/Cursor.app", // macOS
		filepath.Join(a.homeDir, ".local", "share", "cursor"),  // Linux
		filepath.Join(a.homeDir, "AppData", "Local", "Programs", "Cursor"), // Windows
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	// Fallback: ~/.cursor directory exists (used by some user-managed
	// installs and the Cursor CLI on every platform).
	if _, err := os.Stat(filepath.Join(a.homeDir, ".cursor")); err == nil {
		return true
	}
	return false
}

func (a *CursorAdapter) WriteRules(rules []*storage.Rule, scope, projectPath string) error {
	targetPath := a.TargetPath(scope, projectPath)

	entries := rulesToEntries(rules)

	result, err := a.mb.Write(targetPath, entries)
	if err != nil {
		return fmt.Errorf("write cursorrules: %w", err)
	}

	slog.Info("wrote rules to cursorrules",
		"path", targetPath,
		"rules", len(rules),
		"verified", result.Verified,
	)
	return nil
}

func (a *CursorAdapter) RemoveRules(scope, projectPath string) error {
	targetPath := a.TargetPath(scope, projectPath)
	return a.mb.Remove(targetPath)
}

func (a *CursorAdapter) TargetPath(scope, projectPath string) string {
	if scope == "global" {
		// Cursor global config — not well-defined, use home dir.
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".cursorrules")
	}
	return filepath.Join(projectPath, ".cursorrules")
}

// --- Helpers ---

func rulesToEntries(rules []*storage.Rule) []RuleEntry {
	entries := make([]RuleEntry, 0, len(rules))
	for _, r := range rules {
		if r.Status == "active" {
			entries = append(entries, RuleEntry{
				Content:    r.Content,
				Confidence: r.Confidence,
			})
		}
	}
	return entries
}
