package distill

import (
	"path/filepath"
	"testing"

	"github.com/joevilcai666/shadow/internal/storage"
)

// testContextEngine builds an in-memory-backed ContextEngine over a temp DB,
// returning the engine and repo so tests can seed rules.
func testContextEngine(t *testing.T) (*ContextEngine, *storage.RuleRepo) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ruleRepo := storage.NewRuleRepo(db)
	return NewContextEngine(ruleRepo), ruleRepo
}

func mustCreateRule(t *testing.T, repo *storage.RuleRepo, r *storage.Rule) {
	t.Helper()
	r.ID = storage.NewID()
	r.Version = 1
	r.CreatedAt = storage.Now()
	r.UpdatedAt = storage.Now()
	if r.Status == "" {
		r.Status = "active"
	}
	if err := repo.Create(r); err != nil {
		t.Fatalf("create rule: %v", err)
	}
}

// TestExtractRanksByTagThenRecencyAndCapsAtMaxRules locks the SHADOW-033 contract
// that SHADOW-036's preview TUI depends on: tag hits rank highest, results are
// capped at MaxRules, TotalFound counts all matches before the cap, and tag /
// project metadata is surfaced.
func TestExtractRanksByTagThenRecencyAndCapsAtMaxRules(t *testing.T) {
	engine, repo := testContextEngine(t)

	// A: global, tag "auth", high decay → should rank #1.
	mustCreateRule(t, repo, &storage.Rule{
		Content: "always validate JWT expiry", Scope: "global",
		Tags: []string{"auth"}, DecayScore: 0.9, Confidence: 0.9,
	})
	// B: global, no tag hit, low confidence → ranks last among matches.
	mustCreateRule(t, repo, &storage.Rule{
		Content: "use conventional commits", Scope: "global",
		Tags: []string{"workflow"}, DecayScore: 0.5, Confidence: 0.5,
	})
	// C: project-scoped, matches project, tag "auth" → ranks #2.
	mustCreateRule(t, repo, &storage.Rule{
		Content: "project uses bcrypt hashing", Scope: "project",
		ProjectPath: "/tmp/proj", Tags: []string{"auth"},
		DecayScore: 0.8, Confidence: 0.8,
	})
	// D: project-scoped, DIFFERENT project → must be excluded.
	mustCreateRule(t, repo, &storage.Rule{
		Content: "other project rule", Scope: "project",
		ProjectPath: "/tmp/other", Tags: []string{"auth"},
		DecayScore: 0.99, Confidence: 0.99,
	})

	ctx, err := engine.Extract(TaskContextRequest{
		TaskDescription: "fix the auth login bug",
		ProjectPath:     "/tmp/proj",
		Tags:            []string{"auth"},
		MaxRules:        2,
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if got := len(ctx.Rules); got != 2 {
		t.Fatalf("expected 2 rules (capped at MaxRules), got %d", got)
	}
	// #1 must be the high-decay global auth rule.
	if ctx.Rules[0].Content != "always validate JWT expiry" {
		t.Errorf("rank #1 = %q, want the global high-decay auth rule", ctx.Rules[0].Content)
	}
	// #2 must be the matching project auth rule, not the off-project one.
	if ctx.Rules[1].Content != "project uses bcrypt hashing" {
		t.Errorf("rank #2 = %q, want the matching project auth rule", ctx.Rules[1].Content)
	}
	// Off-project rule D excluded; A,B,C matched → TotalFound == 3.
	if ctx.TotalFound != 3 {
		t.Errorf("TotalFound = %d, want 3", ctx.TotalFound)
	}
	if !ctx.ProjectMatched {
		t.Error("ProjectMatched = false, want true (a project rule matched)")
	}
	// Tag hit on "auth" surfaced.
	foundAuth := false
	for _, tag := range ctx.TagMatches {
		if tag == "auth" {
			foundAuth = true
		}
	}
	if !foundAuth {
		t.Errorf("TagMatches = %v, want it to contain %q", ctx.TagMatches, "auth")
	}
}

// TestExtractRespectsMaxRulesDefault ensures a zero MaxRules falls back to 5.
func TestExtractRespectsMaxRulesDefault(t *testing.T) {
	engine, repo := testContextEngine(t)
	for i := 0; i < 7; i++ {
		mustCreateRule(t, repo, &storage.Rule{
			Content: "global rule", Scope: "global", Confidence: 0.5,
		})
	}
	ctx, err := engine.Extract(TaskContextRequest{
		TaskDescription: "anything",
		ProjectPath:     "/tmp/proj",
		// MaxRules left 0 → defaults to 5.
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(ctx.Rules) != 5 {
		t.Errorf("expected default cap of 5 rules, got %d", len(ctx.Rules))
	}
	if ctx.TotalFound != 7 {
		t.Errorf("TotalFound = %d, want 7", ctx.TotalFound)
	}
}

func TestExtractKeepsStableOrderForEqualScores(t *testing.T) {
	engine, repo := testContextEngine(t)
	for _, content := range []string{"first equal rule", "second equal rule", "third equal rule"} {
		mustCreateRule(t, repo, &storage.Rule{
			Content: content, Scope: "global", Confidence: 0.5, DecayScore: 0.5,
		})
	}

	ctx, err := engine.Extract(TaskContextRequest{
		TaskDescription: "anything",
		MaxRules:        3,
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for i, want := range []string{"first equal rule", "second equal rule", "third equal rule"} {
		if ctx.Rules[i].Content != want {
			t.Fatalf("rank %d = %q, want %q", i+1, ctx.Rules[i].Content, want)
		}
	}
}

// TestExtractEmptyWhenNothingMatches: a request whose tags/scope hit nothing
// returns an empty rule set (the friendly "no matching rules" path).
func TestExtractEmptyWhenNothingMatches(t *testing.T) {
	engine, repo := testContextEngine(t)
	// Only an off-project rule exists.
	mustCreateRule(t, repo, &storage.Rule{
		Content: "off-project only", Scope: "project",
		ProjectPath: "/tmp/elsewhere", Confidence: 0.9,
	})
	ctx, err := engine.Extract(TaskContextRequest{
		TaskDescription: "unrelated task",
		ProjectPath:     "/tmp/proj",
		Tags:            []string{"nonexistent"},
		MaxRules:        5,
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(ctx.Rules) != 0 {
		t.Errorf("expected 0 rules for non-matching request, got %d", len(ctx.Rules))
	}
}
