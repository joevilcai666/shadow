package adapter

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joevilcai666/shadow/internal/storage"
)

// CopilotAdapter writes rules to GitHub Copilot's instruction files.
//
// GitHub Copilot reads instructions from `.github/copilot-instructions.md`
// in the project root (and from a per-user `~/.copilot/instructions.md`
// when configured). We follow the same scope convention as the other
// adapters: project-level rules go to the project file, global rules
// go to the user-level file.
type CopilotAdapter struct {
	mb *ManagedBlock
}

// NewCopilotAdapter creates a new Copilot adapter.
func NewCopilotAdapter(backupDir string) *CopilotAdapter {
	return &CopilotAdapter{
		mb: NewManagedBlock(backupDir),
	}
}

// Name returns the adapter identifier.
func (a *CopilotAdapter) Name() string { return "copilot" }

// IsInstalled reports whether the Copilot CLI or VS Code extension is
// detectable on this machine. Copilot has no single sentinel binary on
// disk, so we treat it as always-available — the same way the onboarding
// flow (daemon.go) and /api/adapters list (server.go) treat it.
func (a *CopilotAdapter) IsInstalled() bool {
	return true
}

// WriteRules writes the given rules to the Copilot context file for the
// requested scope/project. The actual file is managed via ManagedBlock so
// any hand-written content outside the shadow-managed region is preserved
// (MD5-verified, backed up, atomic-rename).
func (a *CopilotAdapter) WriteRules(rules []*storage.Rule, scope, projectPath string) error {
	targetPath := a.TargetPath(scope, projectPath)

	entries := rulesToEntries(rules)

	result, err := a.mb.Write(targetPath, entries)
	if err != nil {
		return fmt.Errorf("write copilot-instructions: %w", err)
	}

	slog.Info("wrote rules to copilot-instructions",
		"path", targetPath,
		"rules", len(rules),
		"verified", result.Verified,
	)
	return nil
}

// RemoveRules strips the managed block from the Copilot context file
// for the requested scope/project, leaving any hand-written content
// untouched.
func (a *CopilotAdapter) RemoveRules(scope, projectPath string) error {
	targetPath := a.TargetPath(scope, projectPath)
	return a.mb.Remove(targetPath)
}

// TargetPath returns the absolute file path Copilot will read for the
// given scope. For project rules, we use `.github/copilot-instructions.md`
// (the documented location). For global rules, we fall back to the
// per-user instruction file location under ~/.copilot/instructions.md
// which is the same place the official `gh copilot` CLI stores its
// generated user instructions.
func (a *CopilotAdapter) TargetPath(scope, projectPath string) string {
	if scope == "global" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".copilot", "instructions.md")
	}
	return filepath.Join(projectPath, ".github", "copilot-instructions.md")
}
