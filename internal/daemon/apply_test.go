package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joevilcai666/shadow/internal/storage"
)

func TestApplyAgentsToAdapters(t *testing.T) {
	// Sandbox: use a temp dir as projectPath and a temp dir as $HOME so
	// the global adapter writes don't pollute the developer's actual home.
	projectDir := t.TempDir()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Pre-create a CLAUDE.md with handwritten content that the adapter
	// must preserve verbatim.
	handwritten := "# My Project\n\nHand-written instructions live here.\n"
	handwrittenPath := filepath.Join(projectDir, "CLAUDE.md")
	if err := os.WriteFile(handwrittenPath, []byte(handwritten), 0644); err != nil {
		t.Fatalf("write handwritten: %v", err)
	}

	// Build a couple of active rules.
	rules := []*storage.Rule{
		{
			ID:         storage.NewID(),
			Content:    "Use pnpm not npm",
			Status:     "active",
			Confidence: 0.9,
			Scope:      "project",
		},
		{
			ID:         storage.NewID(),
			Content:    "Always write tests for new functions",
			Status:     "active",
			Confidence: 0.85,
			Scope:      "project",
		},
		{
			ID:         storage.NewID(),
			Content:    "Disabled rule — should not be written",
			Status:     "disabled",
			Confidence: 0.5,
			Scope:      "project",
		},
	}

	applied, errs := ApplyAgentsToAdapters(
		[]string{"Claude Code", "Codex"},
		projectDir,
		rules,
	)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// We expect at least 2 writes per supported agent: global + project.
	if len(applied) < 2 {
		t.Fatalf("expected at least 2 applied files, got %d: %v", len(applied), applied)
	}

	// The handwritten content must still be on disk.
	content, err := os.ReadFile(handwrittenPath)
	if err != nil {
		t.Fatalf("read handwritten back: %v", err)
	}
	if !strings.Contains(string(content), "Hand-written instructions live here.") {
		t.Error("handwritten content lost after apply")
	}
	if !strings.Contains(string(content), "Use pnpm not npm") {
		t.Error("managed block missing the first rule")
	}
	if !strings.Contains(string(content), "Always write tests for new functions") {
		t.Error("managed block missing the second rule")
	}
	if strings.Contains(string(content), "Disabled rule") {
		t.Error("disabled rule leaked into managed block")
	}

	// Verify the global Claude Code target was created in the fake home.
	globalClaudePath := filepath.Join(fakeHome, ".claude", "CLAUDE.md")
	if _, err := os.Stat(globalClaudePath); err != nil {
		t.Errorf("global CLAUDE.md was not created: %v", err)
	} else {
		b, _ := os.ReadFile(globalClaudePath)
		if !strings.Contains(string(b), "Use pnpm not npm") {
			t.Error("global managed block missing the rule")
		}
	}

	// Verify the global Codex target (AGENTS.md in $HOME).
	globalCodexPath := filepath.Join(fakeHome, "AGENTS.md")
	if _, err := os.Stat(globalCodexPath); err != nil {
		t.Errorf("global AGENTS.md was not created: %v", err)
	}

	// Verify project Codex target (AGENTS.md in projectDir).
	projectCodexPath := filepath.Join(projectDir, "AGENTS.md")
	if _, err := os.Stat(projectCodexPath); err != nil {
		t.Errorf("project AGENTS.md was not created: %v", err)
	}
}

func TestApplyAgentsToAdapters_UnknownAgent(t *testing.T) {
	projectDir := t.TempDir()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	applied, errs := ApplyAgentsToAdapters(
		[]string{"Bogus Agent"},
		projectDir,
		[]*storage.Rule{{ID: "r1", Content: "X", Status: "active", Confidence: 0.9}},
	)
	if len(errs) == 0 {
		t.Fatal("expected an error for unknown agent")
	}
	if _, ok := errs["Bogus Agent"]; !ok {
		t.Errorf("expected error keyed by 'Bogus Agent', got %v", errs)
	}
	if len(applied) != 0 {
		t.Errorf("expected 0 applied files, got %d", len(applied))
	}
}

func TestApplyAgentsToAdapters_EmptyRules(t *testing.T) {
	projectDir := t.TempDir()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	applied, errs := ApplyAgentsToAdapters(
		[]string{"Claude Code"},
		projectDir,
		nil, // no rules at all
	)
	if len(errs) > 0 {
		t.Errorf("empty rules should not error, got %v", errs)
	}
	if len(applied) != 0 {
		t.Errorf("expected 0 applied files for empty rules, got %d", len(applied))
	}
}

func TestApplyAgentsToAdapters_OnlyDisabled(t *testing.T) {
	projectDir := t.TempDir()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	rules := []*storage.Rule{
		{ID: "r1", Content: "Disabled", Status: "disabled", Confidence: 0.5},
	}
	applied, errs := ApplyAgentsToAdapters(
		[]string{"Claude Code"},
		projectDir,
		rules,
	)
	if len(errs) > 0 {
		t.Errorf("only disabled rules should not error, got %v", errs)
	}
	if len(applied) != 0 {
		t.Errorf("expected 0 applied files when all rules disabled, got %d", len(applied))
	}
}
