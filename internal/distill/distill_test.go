package distill

import (
	"path/filepath"
	"testing"

	"github.com/joevilcai666/shadow/internal/storage"
)

func testDistillEngine(t *testing.T) (*Engine, *storage.RuleRepo, *storage.SourceRepo) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ruleDB := storage.NewRuleRepo(db)
	sourceDB := storage.NewSourceRepo(db)
	distiller := NewRuleBasedDistiller()

	engine := NewEngine(distiller, ruleDB, sourceDB, "medium")
	return engine, ruleDB, sourceDB
}

func TestDistillSignals(t *testing.T) {
	engine, ruleDB, _ := testDistillEngine(t)

	sources := []*storage.Source{
		{ID: storage.NewID(), SignalType: "explicit_instruction", SignalStrength: "strong", AgentName: "claude-code", ProjectPath: "/tmp/proj", RawSnippet: "不对，应该用 pnpm，不要用 npm", Timestamp: storage.Now(), ConfidenceContribution: 0.8},
		{ID: storage.NewID(), SignalType: "manual_edit", SignalStrength: "medium", AgentName: "cursor", ProjectPath: "/tmp/proj", RawSnippet: "use pnpm instead of npm", Timestamp: storage.Now(), ConfidenceContribution: 0.5},
	}

	if err := engine.ProcessSignals(sources); err != nil {
		t.Fatalf("process signals: %v", err)
	}

	// Should have created a candidate rule.
	rules, _ := ruleDB.List(storage.RuleFilter{})
	if len(rules) == 0 {
		t.Fatal("expected at least 1 rule to be created")
	}

	rule := rules[0]
	if rule.Content == "" {
		t.Error("rule should have content")
	}
	if rule.Category == "" {
		t.Error("rule should have category")
	}
	if rule.Status != "candidate" {
		t.Errorf("status: got %q, want candidate", rule.Status)
	}
}

func TestProcessExplicitSignal(t *testing.T) {
	engine, ruleDB, _ := testDistillEngine(t)

	source := &storage.Source{
		ID: storage.NewID(), SignalType: "manual_mark", SignalStrength: "strong",
		AgentName: "claude-code", RawSnippet: "记住：永远不要用 npm",
		Timestamp: storage.Now(), ConfidenceContribution: 0.95,
	}

	if err := engine.ProcessExplicitSignal(source); err != nil {
		t.Fatalf("process explicit: %v", err)
	}

	rules, _ := ruleDB.List(storage.RuleFilter{})
	if len(rules) == 0 {
		t.Fatal("explicit signal should immediately create a rule")
	}
}

func TestSimilarityDetection(t *testing.T) {
	distiller := NewRuleBasedDistiller()

	existing := []*storage.Rule{
		{ID: "r1", Content: "Use pnpm for package management, not npm", Status: "active"},
	}

	// Same content.
	similar, _ := distiller.CheckSimilarity("Use pnpm for package management, not npm", existing)
	if similar == nil {
		t.Error("identical content should be detected as similar")
	}

	// Different content.
	similar, _ = distiller.CheckSimilarity("Always write unit tests", existing)
	if similar != nil {
		t.Error("unrelated content should not be similar")
	}
}

func TestConflictDetection(t *testing.T) {
	distiller := NewRuleBasedDistiller()

	existing := []*storage.Rule{
		{ID: "r1", Content: "Use tabs for indentation", Status: "active"},
	}

	if !distiller.DetectConflict("Use spaces for indentation", existing) {
		t.Error("tabs vs spaces should conflict")
	}

	if distiller.DetectConflict("Always write tests", existing) {
		t.Error("tests vs tabs should not conflict")
	}
}

func TestCategorize(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"Use pnpm not npm", "toolchain"},
		{"Use 2-space indentation", "code-style"},
		{"Write unit tests for all functions", "testing"},
		{"Follow MVC architecture pattern", "architecture"},
		{"Be concise in comments", "general"},
	}

	for _, tt := range tests {
		got := categorize(tt.content)
		if got != tt.want {
			t.Errorf("categorize(%q) = %q, want %q", tt.content, got, tt.want)
		}
	}
}

func TestCleanContent(t *testing.T) {
	tests := []struct {
		input  string
		want   string
	}{
		{"不对，应该用 pnpm", "应该用 pnpm"},
		{"Don't use npm", "use npm"},
		{"Always write tests", "Always write tests"},
	}

	for _, tt := range tests {
		got := cleanContent(tt.input)
		if got != tt.want {
			t.Errorf("cleanContent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestThresholdCheck(t *testing.T) {
	engine, _, _ := testDistillEngine(t)
	engine.threshold = "medium"

	// Single medium signal should not meet medium threshold.
	mediumSources := []*storage.Source{
		{SignalStrength: "medium", ConfidenceContribution: 0.5},
	}
	if engine.meetsThreshold(mediumSources) {
		t.Error("1 medium signal should not meet medium threshold")
	}

	// Single strong signal should meet threshold.
	strongSources := []*storage.Source{
		{SignalStrength: "strong", ConfidenceContribution: 0.9},
	}
	if !engine.meetsThreshold(strongSources) {
		t.Error("1 strong signal should meet any threshold")
	}
}

func TestMergeSimilarRules(t *testing.T) {
	engine, ruleDB, _ := testDistillEngine(t)

	// Create existing rule.
	existing := &storage.Rule{
		ID: storage.NewID(), Content: "Use pnpm not npm", Scope: "global",
		Tags: []string{"toolchain"}, Category: "toolchain", Confidence: 0.8,
		Status: "active", Version: 1, CreatedAt: storage.Now(), UpdatedAt: storage.Now(),
	}
	ruleDB.Create(existing)

	// Process similar signal.
	sources := []*storage.Source{
		{ID: storage.NewID(), SignalType: "explicit_instruction", SignalStrength: "strong",
			AgentName: "cursor", RawSnippet: "Use pnpm not npm", Timestamp: storage.Now(), ConfidenceContribution: 0.7},
	}

	if err := engine.ProcessSignals(sources); err != nil {
		t.Fatalf("process: %v", err)
	}

	// Should merge, not create new.
	rules, _ := ruleDB.List(storage.RuleFilter{})
	if len(rules) != 1 {
		t.Errorf("should have 1 rule (merged), got %d", len(rules))
	}

	// Confidence should have increased.
	updated, _ := ruleDB.GetByID(existing.ID)
	if updated.Confidence <= 0.8 {
		t.Errorf("confidence should increase after merge: got %f", updated.Confidence)
	}
}
