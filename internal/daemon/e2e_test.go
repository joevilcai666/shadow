package daemon

// E2E_TEST: end-to-end verification that ApplyAgentsToAdapters produces
// a real, on-disk CLAUDE.md/cursorrules/AGENTS.md change set, that
// handwritten content is preserved, and that the managed block markers
// are present.
//
// This is the test the spec asks for as "E2E 验证":
//
//   1. create tmp project dir + hand-write a CLAUDE.md
//   2. create db + 1 active rule
//   3. call ApplyAgentsToAdapters
//   4. verify: hand-written content preserved + managed block present
//
// Run with:
//
//   go test -run TestE2E_ApplyAgentsToAdapters -v ./internal/daemon/...
//
// Or directly:
//
//   go test -v -run TestE2E ./...

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joevilcai666/shadow/internal/storage"
)

func TestE2E_ApplyAgentsToAdapters(t *testing.T) {
	// --- Step 1: create tmp project + handwritten CLAUDE.md -----------
	projectDir := t.TempDir()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	handwritten := "# My Project\n\n" +
		"This is hand-written project context that MUST be preserved.\n" +
		"It contains specific instructions for my team.\n"
	handwrittenPath := filepath.Join(projectDir, "CLAUDE.md")
	if err := os.WriteFile(handwrittenPath, []byte(handwritten), 0o644); err != nil {
		t.Fatalf("write handwritten CLAUDE.md: %v", err)
	}

	// --- Step 2: create db + 1 active rule -----------------------------
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "shadow.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ruleRepo := storage.NewRuleRepo(db)
	rule := &storage.Rule{
		ID:         storage.NewID(),
		Content:    "Always run `go test ./...` before committing",
		Status:     "active",
		Confidence: 0.92,
		Scope:      "project",
		Version:    1,
		CreatedAt:  storage.Now(),
		UpdatedAt:  storage.Now(),
	}
	if err := ruleRepo.Create(rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// Read it back to make sure it's there.
	active, err := ruleRepo.List(storage.RuleFilter{Status: "active"})
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active rule, got %d", len(active))
	}
	if active[0].Content != rule.Content {
		t.Fatalf("rule content mismatch: got %q want %q", active[0].Content, rule.Content)
	}

	// --- Step 3: call ApplyAgentsToAdapters ---------------------------
	applied, errs := ApplyAgentsToAdapters(
		[]string{"Claude Code", "Cursor", "Codex"},
		projectDir,
		active,
	)
	if len(errs) > 0 {
		t.Fatalf("ApplyAgentsToAdapters errors: %v", errs)
	}

	// We expect 2 writes per supported agent: global + project scope.
	// That's 6 paths for {Claude Code, Cursor, Codex}.
	if len(applied) < 6 {
		t.Fatalf("expected at least 6 applied paths, got %d: %v", len(applied), applied)
	}

	// --- Step 4: verify outcomes --------------------------------------
	// 4a. Handwritten content preserved.
	got, err := os.ReadFile(handwrittenPath)
	if err != nil {
		t.Fatalf("read CLAUDE.md back: %v", err)
	}
	gs := string(got)
	if !strings.Contains(gs, "hand-written project context") {
		t.Error("handwritten content was lost")
	}
	if !strings.Contains(gs, "specific instructions for my team") {
		t.Error("handwritten content was lost (line 2)")
	}

	// 4b. Managed block present.
	if !strings.Contains(gs, "# >>> shadow managed >>>") {
		t.Error("managed block begin marker missing")
	}
	if !strings.Contains(gs, "# <<< shadow managed <<<") {
		t.Error("managed block end marker missing")
	}
	if !strings.Contains(gs, "Always run `go test ./...` before committing") {
		t.Error("active rule did not make it into the managed block")
	}

	// 4c. Cursor + Codex project targets also got written.
	if _, err := os.Stat(filepath.Join(projectDir, ".cursorrules")); err != nil {
		t.Errorf("project .cursorrules not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "AGENTS.md")); err != nil {
		t.Errorf("project AGENTS.md not written: %v", err)
	}

	// 4d. Global targets were created in the fake home.
	for _, p := range []string{
		filepath.Join(fakeHome, ".claude", "CLAUDE.md"),
		filepath.Join(fakeHome, ".cursorrules"),
		filepath.Join(fakeHome, "AGENTS.md"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("global target %s not written: %v", p, err)
		}
	}

	// 4e. No managed block contains the disabled rules.
	// (None were added in this test, so we just confirm the active
	// rule is the only one inside the block — i.e. no leakage.)
	block := extractManagedBlock(gs)
	if strings.Count(block, "\n") < 3 {
		t.Errorf("managed block looks empty: %q", block)
	}
	if strings.Contains(block, "disabled") {
		t.Error("disabled marker leaked into managed block")
	}
}

// extractManagedBlock returns the contents between the begin and end
// markers, with markers stripped. Used by the E2E test to assert the
// block actually has the rules we expected.
func extractManagedBlock(content string) string {
	const begin = "# >>> shadow managed >>>"
	const end = "# <<< shadow managed <<<"
	i := strings.Index(content, begin)
	if i < 0 {
		return ""
	}
	j := strings.Index(content, end)
	if j < 0 {
		return ""
	}
	return content[i+len(begin) : j]
}
