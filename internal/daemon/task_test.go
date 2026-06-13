package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joevilcai666/shadow/internal/storage"
)

// newTaskCommandWithSeed opens a temp DB, seeds one active rule, and returns a
// TaskCommand ready to exercise RunTask / InjectIntoAgent.
func newTaskCommandWithSeed(t *testing.T) (*TaskCommand, *storage.Rule) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ruleRepo := storage.NewRuleRepo(db)
	rule := &storage.Rule{
		ID:         storage.NewID(),
		Content:    "Always strip whitespace from JWT before decoding",
		Scope:      "global",
		Tags:       []string{"auth", "security"},
		Category:   "security",
		Confidence: 0.9,
		DecayScore: 0.85,
		Status:     "active",
		Version:    1,
		Importance: 0.8,
		Author:     "user",
		CreatedAt:  storage.Now(),
		UpdatedAt:  storage.Now(),
	}
	if err := ruleRepo.Create(rule); err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	return NewTaskCommand(db, ruleRepo), rule
}

// TestRunTaskExtractsRelevantRules locks the contract that cmd/shadow's taskCmd
// depends on: RunTask returns the real ExtractedContext (ranked rules + metadata)
// for a task description. This is the SHADOW-032 wiring contract.
func TestRunTaskExtractsRelevantRules(t *testing.T) {
	tc, rule := newTaskCommandWithSeed(t)

	result, err := tc.RunTask("fix the auth login bug", "codex", "/tmp/proj")
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	if result == nil || result.ExtractedContext == nil {
		t.Fatal("expected non-nil ExtractedContext")
	}
	if len(result.ExtractedContext.Rules) == 0 {
		t.Fatal("expected at least 1 extracted rule")
	}
	// The seeded global auth rule must be among the extracted rules.
	found := false
	for _, r := range result.ExtractedContext.Rules {
		if r.ID == rule.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("seeded rule %s not in extracted set", rule.ID)
	}
}

// TestInjectIntoAgentWritesAgentFile proves the SHADOW-032/036 confirm step's
// real side effect: selected rules are written into the target agent's context
// file. Uses an isolated HOME so backups do not touch the real home directory.
func TestInjectIntoAgentWritesAgentFile(t *testing.T) {
	tc, rule := newTaskCommandWithSeed(t)

	// Isolate HOME so adapter backups land under temp, not the real home.
	home := t.TempDir()
	t.Setenv("HOME", home)
	projectDir := t.TempDir()

	rules := []*storage.Rule{rule}
	if err := tc.InjectIntoAgent("codex", rules, projectDir); err != nil {
		t.Fatalf("InjectIntoAgent: %v", err)
	}

	// Codex project-scoped adapter writes <project>/AGENTS.md.
	written, err := os.ReadFile(filepath.Join(projectDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("expected AGENTS.md to be written: %v", err)
	}
	if !strings.Contains(string(written), rule.Content) {
		t.Errorf("AGENTS.md does not contain rule content; got:\n%s", written)
	}
}

// TestInjectIntoAgentRejectsUnknownAgent ensures we don't silently write to a
// garbage path for an unsupported agent name.
func TestInjectIntoAgentRejectsUnknownAgent(t *testing.T) {
	tc, rule := newTaskCommandWithSeed(t)
	err := tc.InjectIntoAgent("no-such-agent", []*storage.Rule{rule}, "/tmp/proj")
	if err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
}
