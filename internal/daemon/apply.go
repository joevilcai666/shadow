package daemon

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/joevilcai666/shadow/internal/adapter"
	"github.com/joevilcai666/shadow/internal/storage"
)

// ApplyResult describes the outcome of a single adapter write during onboarding.
type ApplyResult struct {
	Agent     string
	Target    string
	OK        bool
	Err       error
	Files     []string // paths successfully written (one per scope)
}

// ApplyAgentsToAdapters writes the provided rules to the adapters backing each
// selected agent. It writes both global (user-home) and project (cwd) scopes by
// default — this is the safe default the onboarding wizard confirms.
//
// selectedAgentNames contains the user-facing labels from the TUI checkbox list
// (e.g. "Claude Code", "Cursor", "Codex"). Unknown names are skipped.
//
// On any per-adapter failure, the function records the error in errs and
// continues with the remaining adapters. It does NOT short-circuit the loop.
//
// Returned values:
//   - appliedFiles: every target path that was written successfully
//   - errs:         per-agent errors (keyed by display name) for any failures
func ApplyAgentsToAdapters(
	selectedAgentNames []string,
	projectPath string,
	rules []*storage.Rule,
) (appliedFiles []string, errs map[string]error) {
	errs = make(map[string]error)

	if projectPath == "" {
		// Fall back to cwd so callers don't have to pass it explicitly.
		if cwd, err := os.Getwd(); err == nil {
			projectPath = cwd
		}
	}

	home, _ := os.UserHomeDir()
	backupDir := filepath.Join(home, ".shadow", "backups")
	_ = os.MkdirAll(backupDir, 0o755)

	// Split rules by scope so we can pass only the relevant set to each adapter
	// call. The adapters themselves don't filter on scope; they just write
	// everything that is "active".
	active := filterActiveRules(rules)
	if len(active) == 0 {
		// Nothing to write but no error either — caller will report "0 rules".
		return appliedFiles, errs
	}

	for _, label := range selectedAgentNames {
		a, ok := adapterForLabel(label, backupDir)
		if !ok {
			errs[label] = fmt.Errorf("unknown agent: %q", label)
			continue
		}

		// Write to BOTH global and project scopes.
		scopes := []struct {
			scope string
			path  string
		}{
			{scope: "global", path: ""},
			{scope: "project", path: projectPath},
		}

		var firstErr error
		var wroteAny bool
		for _, s := range scopes {
			target := a.TargetPath(s.scope, s.path)
			if err := a.WriteRules(active, s.scope, s.path); err != nil {
				slog.Warn("onboarding: adapter write failed",
					"agent", a.Name(), "scope", s.scope, "target", target, "error", err)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			appliedFiles = append(appliedFiles, target)
			wroteAny = true
		}

		if firstErr != nil {
			errs[label] = firstErr
		}
		_ = wroteAny // Currently unused; kept for future metrics.
	}

	return appliedFiles, errs
}

// adapterForLabel returns the adapter corresponding to a TUI checkbox label.
// Currently supports the four agents exposed in NewOnboardingModel.
//
// GitHub Copilot does not have a dedicated adapter in the codebase yet, so it
// is intentionally skipped with a friendly error.
func adapterForLabel(label, backupDir string) (adapter.Adapter, bool) {
	switch strings.TrimSpace(label) {
	case "Claude Code":
		return adapter.NewClaudeCodeAdapter(backupDir), true
	case "Cursor":
		return adapter.NewCursorAdapter(backupDir), true
	case "Codex":
		return adapter.NewCodexAdapter(backupDir), true
	case "GitHub Copilot":
		// No dedicated adapter yet — caller will see a friendly error.
		return nil, false
	default:
		return nil, false
	}
}

// filterActiveRules keeps only rules that are eligible to be written to disk.
// The adapter layer applies the same filter, but we trim the list here so we
// don't have to allocate adapter-internal entries for rules that will be
// discarded anyway.
func filterActiveRules(rules []*storage.Rule) []*storage.Rule {
	out := make([]*storage.Rule, 0, len(rules))
	for _, r := range rules {
		if r == nil {
			continue
		}
		if r.Status != "active" {
			continue
		}
		out = append(out, r)
	}
	return out
}
