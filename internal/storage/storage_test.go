package storage

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// openTestDB creates a temp SQLite database with migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenAndMigrate(t *testing.T) {
	db := openTestDB(t)

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("query migrations: %v", err)
	}
	if count == 0 {
		t.Error("expected at least 1 migration to be applied")
	}

	// Verify tables exist.
	tables := []string{"rules", "sources", "versions", "config", "projects", "events"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}

}

func TestEventRepoRecordsEffectivenessEvents(t *testing.T) {
	db := openTestDB(t)
	ruleRepo := NewRuleRepo(db)
	eventRepo := NewEventRepo(db)

	rule := &Rule{
		ID: NewID(), Content: "Use pnpm", Scope: "global",
		Tags: []string{"toolchain"}, Status: "active", Version: 1,
		CreatedAt: Now(), UpdatedAt: Now(),
	}
	if err := ruleRepo.Create(rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	for _, event := range []*Event{
		{
			ID: NewID(), RuleID: rule.ID, EventType: "rule_hit",
			AgentName: "codex", ProjectPath: "/tmp/app", TargetPath: "AGENTS.md",
			Details: "Aha demo memory hit", Timestamp: Now(),
		},
		{
			ID: NewID(), RuleID: rule.ID, EventType: "sync_success",
			AgentName: "codex", ProjectPath: "/tmp/app", TargetPath: "AGENTS.md",
			Details: "wrote managed block", Timestamp: Now(),
		},
	} {
		if err := eventRepo.Create(event); err != nil {
			t.Fatalf("create event: %v", err)
		}
	}

	counts, err := eventRepo.CountRuleHits()
	if err != nil {
		t.Fatalf("count rule hits: %v", err)
	}
	if counts[rule.ID] != 1 {
		t.Fatalf("rule hit count = %d, want 1", counts[rule.ID])
	}

	latest, err := eventRepo.LatestByAgentEvent("codex", "sync_success")
	if err != nil {
		t.Fatalf("latest by agent: %v", err)
	}
	if latest == nil || latest.TargetPath != "AGENTS.md" {
		t.Fatalf("latest sync event = %#v, want AGENTS.md target", latest)
	}
}

func TestRuleCRUD(t *testing.T) {
	db := openTestDB(t)
	repo := NewRuleRepo(db)

	rule := &Rule{
		ID:         NewID(),
		Content:    "This project uses pnpm, not npm",
		Scope:      "global",
		Tags:       []string{"tooling", "package-manager"},
		Category:   "toolchain",
		Confidence: 0.85,
		Status:     "candidate",
		Version:    1,
		CreatedAt:  Now(),
		UpdatedAt:  Now(),
	}

	// Create
	if err := repo.Create(rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// GetByID
	got, err := repo.GetByID(rule.ID)
	if err != nil {
		t.Fatalf("get rule: %v", err)
	}
	if got == nil {
		t.Fatal("expected rule, got nil")
	}
	if got.Content != rule.Content {
		t.Errorf("content mismatch: got %q, want %q", got.Content, rule.Content)
	}
	if got.Scope != "global" {
		t.Errorf("scope mismatch: got %q", got.Scope)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags length: got %d, want 2", len(got.Tags))
	}

	// Update
	got.Content = "Always use pnpm for this project"
	got.Status = "active"
	got.UpdatedAt = Now()
	if err := repo.Update(got, "user", "activated rule"); err != nil {
		t.Fatalf("update rule: %v", err)
	}

	updated, err := repo.GetByID(rule.ID)
	if err != nil {
		t.Fatalf("get updated rule: %v", err)
	}
	if updated.Content != "Always use pnpm for this project" {
		t.Errorf("updated content mismatch: %q", updated.Content)
	}
	if updated.Version != 2 {
		t.Errorf("version should be 2, got %d", updated.Version)
	}
	if updated.Status != "active" {
		t.Errorf("status should be active, got %q", updated.Status)
	}

	// List
	rules, err := repo.List(RuleFilter{Status: "active"})
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 active rule, got %d", len(rules))
	}

	// Count
	count, err := repo.Count(RuleFilter{Scope: "global"})
	if err != nil {
		t.Fatalf("count rules: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 global rule, got %d", count)
	}

	// Delete
	if err := repo.Delete(rule.ID); err != nil {
		t.Fatalf("delete rule: %v", err)
	}
	deleted, err := repo.GetByID(rule.ID)
	if err != nil {
		t.Fatalf("get deleted rule: %v", err)
	}
	if deleted != nil {
		t.Error("expected nil after delete")
	}
}

func TestRuleFilterSearch(t *testing.T) {
	db := openTestDB(t)
	repo := NewRuleRepo(db)

	rules := []*Rule{
		{ID: NewID(), Content: "Use pnpm not npm", Scope: "global", Tags: []string{}, Status: "active", Version: 1, CreatedAt: Now(), UpdatedAt: Now()},
		{ID: NewID(), Content: "Always write tests", Scope: "global", Tags: []string{}, Status: "active", Version: 1, CreatedAt: Now(), UpdatedAt: Now()},
		{ID: NewID(), Content: "Use tabs for indentation", Scope: "project", ProjectPath: "/tmp/demo", Tags: []string{}, Status: "candidate", Version: 1, CreatedAt: Now(), UpdatedAt: Now()},
	}
	for _, r := range rules {
		if err := repo.Create(r); err != nil {
			t.Fatalf("create rule: %v", err)
		}
	}

	// Search
	results, err := repo.List(RuleFilter{Search: "pnpm"})
	if err != nil {
		t.Fatalf("search rules: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("search 'pnpm': expected 1, got %d", len(results))
	}

	// Scope filter
	global, err := repo.List(RuleFilter{Scope: "global"})
	if err != nil {
		t.Fatalf("filter scope: %v", err)
	}
	if len(global) != 2 {
		t.Errorf("global rules: expected 2, got %d", len(global))
	}

	// Limit
	limited, err := repo.List(RuleFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list with limit: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("limit 1: expected 1, got %d", len(limited))
	}
}

func TestSourceTimeline(t *testing.T) {
	db := openTestDB(t)
	ruleRepo := NewRuleRepo(db)
	sourceRepo := NewSourceRepo(db)

	rule := &Rule{
		ID: NewID(), Content: "Test rule", Scope: "global",
		Tags: []string{}, Status: "active", Version: 1,
		CreatedAt: Now(), UpdatedAt: Now(),
	}
	if err := ruleRepo.Create(rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	sources := []*Source{
		{ID: NewID(), RuleID: rule.ID, SignalType: "explicit_instruction", SignalStrength: "strong", AgentName: "claude-code", ProjectPath: "/tmp/proj", Timestamp: Now(), ConfidenceContribution: 0.8},
		{ID: NewID(), RuleID: rule.ID, SignalType: "manual_edit", SignalStrength: "medium", AgentName: "cursor", ProjectPath: "/tmp/proj", Timestamp: Now(), ConfidenceContribution: 0.5},
	}
	for _, s := range sources {
		if err := sourceRepo.Create(s); err != nil {
			t.Fatalf("create source: %v", err)
		}
	}

	// List by rule
	timeline, err := sourceRepo.ListByRuleID(rule.ID)
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(timeline) != 2 {
		t.Errorf("expected 2 sources, got %d", len(timeline))
	}

	// Stats
	stats, err := sourceRepo.StatsBySignalType(rule.ID)
	if err != nil {
		t.Fatalf("source stats: %v", err)
	}
	if stats["explicit_instruction"] != 1 || stats["manual_edit"] != 1 {
		t.Errorf("stats mismatch: %v", stats)
	}
}

func TestSourceRepoEvidenceByRuleIDs(t *testing.T) {
	db := openTestDB(t)
	ruleRepo := NewRuleRepo(db)
	sourceRepo := NewSourceRepo(db)

	r1 := &Rule{ID: NewID(), Content: "r1", Scope: "global", Tags: []string{}, Status: "active", Version: 1, CreatedAt: Now(), UpdatedAt: Now()}
	r2 := &Rule{ID: NewID(), Content: "r2", Scope: "global", Tags: []string{}, Status: "active", Version: 1, CreatedAt: Now(), UpdatedAt: Now()}
	for _, rule := range []*Rule{r1, r2} {
		if err := ruleRepo.Create(rule); err != nil {
			t.Fatalf("create rule: %v", err)
		}
	}
	for _, source := range []*Source{
		{ID: NewID(), RuleID: r1.ID, SignalType: "explicit_instruction", SignalStrength: "strong", AgentName: "claude-code", RawSnippet: "first snippet", Timestamp: "2026-01-01T00:00:00Z"},
		{ID: NewID(), RuleID: r1.ID, SignalType: "manual_edit", SignalStrength: "medium", AgentName: "codex", RawSnippet: "second snippet", Timestamp: "2026-01-02T00:00:00Z"},
		{ID: NewID(), RuleID: r2.ID, SignalType: "manual_mark", SignalStrength: "strong", AgentName: "cursor", RawSnippet: "other snippet", Timestamp: "2026-01-01T00:00:00Z"},
	} {
		if err := sourceRepo.Create(source); err != nil {
			t.Fatalf("create source: %v", err)
		}
	}

	evidence, err := sourceRepo.EvidenceByRuleIDs([]string{r1.ID, r2.ID})
	if err != nil {
		t.Fatalf("evidence: %v", err)
	}
	if evidence[r1.ID].FirstSnippet != "first snippet" {
		t.Fatalf("r1 first snippet = %q, want first snippet", evidence[r1.ID].FirstSnippet)
	}
	if !evidence[r1.ID].Agents["claude-code"] || !evidence[r1.ID].Agents["codex"] {
		t.Fatalf("r1 agents = %#v, want claude-code and codex", evidence[r1.ID].Agents)
	}
	if evidence[r2.ID].FirstSnippet != "other snippet" {
		t.Fatalf("r2 first snippet = %q, want other snippet", evidence[r2.ID].FirstSnippet)
	}
}

func TestEventRepoAgentsByRuleIDs(t *testing.T) {
	db := openTestDB(t)
	ruleRepo := NewRuleRepo(db)
	eventRepo := NewEventRepo(db)

	r1 := &Rule{ID: NewID(), Content: "r1", Scope: "global", Tags: []string{}, Status: "active", Version: 1, CreatedAt: Now(), UpdatedAt: Now()}
	r2 := &Rule{ID: NewID(), Content: "r2", Scope: "global", Tags: []string{}, Status: "active", Version: 1, CreatedAt: Now(), UpdatedAt: Now()}
	for _, rule := range []*Rule{r1, r2} {
		if err := ruleRepo.Create(rule); err != nil {
			t.Fatalf("create rule: %v", err)
		}
	}
	for _, event := range []*Event{
		{ID: NewID(), RuleID: r1.ID, EventType: "rule_hit", AgentName: "codex", Timestamp: Now()},
		{ID: NewID(), RuleID: r1.ID, EventType: "rule_hit", AgentName: "claude-code", Timestamp: Now()},
		{ID: NewID(), RuleID: r2.ID, EventType: "rule_hit", AgentName: "cursor", Timestamp: Now()},
		{ID: NewID(), EventType: "sync_success", AgentName: "codex", Timestamp: Now()},
	} {
		if err := eventRepo.Create(event); err != nil {
			t.Fatalf("create event: %v", err)
		}
	}

	agents, err := eventRepo.AgentsByRuleIDs([]string{r1.ID, r2.ID})
	if err != nil {
		t.Fatalf("agents: %v", err)
	}
	if !agents[r1.ID]["codex"] || !agents[r1.ID]["claude-code"] {
		t.Fatalf("r1 agents = %#v, want codex and claude-code", agents[r1.ID])
	}
	if !agents[r2.ID]["cursor"] {
		t.Fatalf("r2 agents = %#v, want cursor", agents[r2.ID])
	}
}

func TestVersionRollback(t *testing.T) {
	db := openTestDB(t)
	ruleRepo := NewRuleRepo(db)
	versionRepo := NewVersionRepo(db)

	rule := &Rule{
		ID: NewID(), Content: "Version 1", Scope: "global",
		Tags: []string{}, Status: "active", Version: 1,
		CreatedAt: Now(), UpdatedAt: Now(),
	}
	if err := ruleRepo.Create(rule); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Update to v2
	rule.Content = "Version 2"
	rule.UpdatedAt = Now()
	if err := ruleRepo.Update(rule, "user", "update content"); err != nil {
		t.Fatalf("update: %v", err)
	}

	// Update to v3
	rule.Content = "Version 3"
	rule.UpdatedAt = Now()
	if err := ruleRepo.Update(rule, "auto", "auto refinement"); err != nil {
		t.Fatalf("update 2: %v", err)
	}

	// Check versions
	versions, err := versionRepo.ListByRuleID(rule.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}

	// Rollback to v1
	if err := versionRepo.Rollback(rule.ID, 1, "revert mistake"); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// Verify rule content restored
	restored, err := ruleRepo.GetByID(rule.ID)
	if err != nil {
		t.Fatalf("get restored: %v", err)
	}
	if restored.Content != "Version 1" {
		t.Errorf("after rollback content: got %q, want %q", restored.Content, "Version 1")
	}
	if restored.Version != 4 {
		t.Errorf("after rollback version: got %d, want 4", restored.Version)
	}
}

func TestConfigWithScopeFallback(t *testing.T) {
	db := openTestDB(t)
	repo := NewConfigRepo(db)

	// Set global config
	if err := repo.Set("theme", "dark", "global"); err != nil {
		t.Fatalf("set global: %v", err)
	}

	// Override for project
	if err := repo.Set("theme", "light", "/tmp/project"); err != nil {
		t.Fatalf("set project: %v", err)
	}

	// Project scope returns project value
	entry, err := repo.Get("theme", "/tmp/project")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if entry.Value != "light" {
		t.Errorf("project value: got %q, want %q", entry.Value, "light")
	}

	// Other scope falls back to global
	entry, err = repo.Get("theme", "/tmp/other")
	if err != nil {
		t.Fatalf("get fallback: %v", err)
	}
	if entry.Value != "dark" {
		t.Errorf("fallback value: got %q, want %q", entry.Value, "dark")
	}

	// List by scope
	global, err := repo.ListByScope("global")
	if err != nil {
		t.Fatalf("list global: %v", err)
	}
	if len(global) != 1 {
		t.Errorf("global configs: expected 1, got %d", len(global))
	}

	// Delete
	if err := repo.Delete("theme", "global"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	deleted, err := repo.Get("theme", "global")
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if deleted != nil {
		t.Error("expected nil after delete")
	}
}

func TestProjectCRUD(t *testing.T) {
	db := openTestDB(t)
	repo := NewProjectRepo(db)

	project := &Project{
		ID:        NewID(),
		Path:      "/Users/dev/myproject",
		Name:      "myproject",
		Agents:    []string{"claude-code", "cursor"},
		CreatedAt: Now(),
	}

	// Create
	if err := repo.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	// GetByID
	got, err := repo.GetByID(project.ID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if got == nil {
		t.Fatal("expected project, got nil")
	}
	if got.Name != "myproject" {
		t.Errorf("name: got %q", got.Name)
	}
	if len(got.Agents) != 2 {
		t.Errorf("agents: got %d", len(got.Agents))
	}

	// GetByPath
	byPath, err := repo.GetByPath("/Users/dev/myproject")
	if err != nil {
		t.Fatalf("get by path: %v", err)
	}
	if byPath.ID != project.ID {
		t.Error("path lookup mismatch")
	}

	// List
	projects, err := repo.List()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}

	// Update
	now := Now()
	got.Agents = append(got.Agents, "codex")
	got.LastScanAt = &now
	if err := repo.Update(got); err != nil {
		t.Fatalf("update project: %v", err)
	}

	updated, _ := repo.GetByID(project.ID)
	if len(updated.Agents) != 3 {
		t.Errorf("agents after update: got %d", len(updated.Agents))
	}

	// Delete
	if err := repo.Delete(project.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	deleted, _ := repo.GetByID(project.ID)
	if deleted != nil {
		t.Error("expected nil after delete")
	}
}

func TestComputeDiff(t *testing.T) {
	// Identical inputs return empty.
	if got := computeDiff("same", "same"); got != "" {
		t.Errorf("identical: got %q, want empty", got)
	}

	// Pure addition: 'a' is common so it appears as context, 'b' is new
	// so it appears as +b.
	got := computeDiff("a", "a\nb")
	if !strings.Contains(got, " a\n") {
		t.Errorf("addition diff missing shared context: %q", got)
	}
	if !strings.Contains(got, "+b") {
		t.Errorf("addition diff missing +b marker: %q", got)
	}

	// Pure change.
	got = computeDiff("foo\nbar", "foo\nbaz")
	if !strings.Contains(got, "-bar") || !strings.Contains(got, "+baz") {
		t.Errorf("change diff missing markers: %q", got)
	}
	if !strings.Contains(got, " foo") {
		t.Errorf("context line missing: %q", got)
	}

	// Multi-line common prefix/suffix preserved as context.
	got = computeDiff("a\nb\nc", "a\nX\nc")
	if !strings.Contains(got, " a") || !strings.Contains(got, " c") {
		t.Errorf("context not preserved on multi-line diff: %q", got)
	}
}

// daysAgoRFC3339 lives in event_repo.go (shared with production code).

func TestUserMemoryCRUD(t *testing.T) {
	db := openTestDB(t)
	repo := NewUserMemoryRepo(db)

	m := &UserMemory{
		ID:       NewID(),
		UserID:   "local",
		Content:  "Use Conventional Commits for all commit messages",
		Category: "convention",
		Tags:     []string{"git", "workflow"},
	}
	if err := repo.Create(m); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Content != m.Content || got.Category != "convention" {
		t.Fatalf("get = %#v, want content %q / category convention", got, m.Content)
	}
	if len(got.Tags) != 2 {
		t.Fatalf("tags = %v, want 2", got.Tags)
	}

	listed, err := repo.List("local", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list len = %d, want 1", len(listed))
	}

	if err := repo.Delete(m.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	gone, _ := repo.GetByID(m.ID)
	if gone != nil {
		t.Fatalf("after delete, get = %#v, want nil", gone)
	}
}

func TestRuleTouchHit(t *testing.T) {
	db := openTestDB(t)
	ruleRepo := NewRuleRepo(db)
	versionRepo := NewVersionRepo(db)

	rule := &Rule{
		ID: NewID(), Content: "Use pnpm", Scope: "global",
		Confidence: 0.9, Status: "active", Version: 1,
		CreatedAt: Now(), UpdatedAt: Now(),
	}
	if err := ruleRepo.Create(rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	now := Now()
	score := ComputeDecayScore(rule.Confidence, now)
	if err := ruleRepo.TouchHit(rule.ID, now, score); err != nil {
		t.Fatalf("touch hit: %v", err)
	}

	refreshed, err := ruleRepo.GetByID(rule.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if refreshed.LastHitAt != now {
		t.Errorf("last_hit_at = %q, want %q", refreshed.LastHitAt, now)
	}
	// A freshly-hit rule should be at (near) full confidence.
	if refreshed.DecayScore < rule.Confidence*0.99 {
		t.Errorf("decay_score = %f, want ~%f (full confidence after hit)", refreshed.DecayScore, rule.Confidence)
	}
	// A hit must NOT create a version snapshot. Create seeds the initial
	// version (v1), so the count must stay the same after TouchHit.
	beforeVersions, err := versionRepo.ListByRuleID(rule.ID)
	if err != nil {
		t.Fatalf("list versions before: %v", err)
	}
	// re-touch to be sure repeated hits don't churn versions either.
	if err := ruleRepo.TouchHit(rule.ID, now, score); err != nil {
		t.Fatalf("touch hit #2: %v", err)
	}
	afterVersions, err := versionRepo.ListByRuleID(rule.ID)
	if err != nil {
		t.Fatalf("list versions after: %v", err)
	}
	if len(afterVersions) != len(beforeVersions) {
		t.Errorf("versions after hit = %d, want %d (a hit is not an edit)", len(afterVersions), len(beforeVersions))
	}
}

func TestEventHitRateAggregation(t *testing.T) {
	db := openTestDB(t)
	ruleRepo := NewRuleRepo(db)
	eventRepo := NewEventRepo(db)

	rule := &Rule{
		ID: NewID(), Content: "Use pnpm", Scope: "global",
		Status: "active", Version: 1, CreatedAt: Now(), UpdatedAt: Now(),
	}
	if err := ruleRepo.Create(rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// Seed rule_hit events at known ages: today (2x), 3 days ago (1x), 10 days ago (1x).
	for _, ts := range []string{Now(), Now(), daysAgoRFC3339(3), daysAgoRFC3339(10)} {
		if err := eventRepo.Create(&Event{
			ID: NewID(), RuleID: rule.ID, EventType: "rule_hit",
			AgentName: "claude-code", Timestamp: ts,
		}); err != nil {
			t.Fatalf("create event: %v", err)
		}
	}

	// Total hits = 4.
	total, err := eventRepo.CountRuleHitsLastDays(365)
	if err != nil {
		t.Fatalf("count last 365: %v", err)
	}
	if total != 4 {
		t.Errorf("total hits = %d, want 4", total)
	}

	// Last 7 days: today (2) + 3 days (1) = 3 (10 days excluded).
	last7, err := eventRepo.CountRuleHitsLastDays(7)
	if err != nil {
		t.Fatalf("last 7: %v", err)
	}
	if last7 != 3 {
		t.Errorf("last 7 days = %d, want 3", last7)
	}

	// Last 14 days: today (2) + 3 days (1) + 10 days (1) = 4.
	// last-week (computed by subtraction) = last14 − last7 = 1.
	last14, err := eventRepo.CountRuleHitsLastDays(14)
	if err != nil {
		t.Fatalf("last 14: %v", err)
	}
	if last14 != 4 {
		t.Errorf("last 14 days = %d, want 4", last14)
	}
	if got := last14 - last7; got != 1 {
		t.Errorf("last-week (last14-last7) = %d, want 1", got)
	}

	// Distinct rules hit in 7 days = 1.
	distinct, err := eventRepo.DistinctHitRulesLastDays(7)
	if err != nil {
		t.Fatalf("distinct: %v", err)
	}
	if distinct != 1 {
		t.Errorf("distinct hit rules = %d, want 1", distinct)
	}

	// Latest hit exists.
	latest, err := eventRepo.LatestRuleHit()
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest == nil || latest.RuleID != rule.ID {
		t.Fatalf("latest = %#v, want rule %s", latest, rule.ID)
	}
}
