package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joevilcai666/shadow/internal/storage"
)

func TestClaudeCodeAdapterTargetPath(t *testing.T) {
	home := t.TempDir()
	a := &ClaudeCodeAdapter{homeDir: home, mb: NewManagedBlock(filepath.Join(t.TempDir(), "backups"))}

	global := a.TargetPath("global", "")
	wantSuffix := filepath.Join(".claude", "CLAUDE.md")
	if !strings.Contains(global, wantSuffix) {
		t.Errorf("global path: %q, want suffix %q", global, wantSuffix)
	}

	projDir := filepath.Join(t.TempDir(), "myproj")
	project := a.TargetPath("project", projDir)
	if !strings.Contains(project, "CLAUDE.md") {
		t.Errorf("project path should contain CLAUDE.md: %q", project)
	}
}

func TestClaudeCodeAdapterWriteRules(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	a := &ClaudeCodeAdapter{homeDir: dir, mb: NewManagedBlock(backupDir)}

	// Create .claude directory.
	os.MkdirAll(filepath.Join(dir, ".claude"), 0755)

	rules := []*storage.Rule{
		{ID: "r1", Content: "Use pnpm not npm", Status: "active", Confidence: 0.9},
		{ID: "r2", Content: "Use tabs for indentation", Status: "active", Confidence: 0.7},
		{ID: "r3", Content: "Disabled rule", Status: "disabled", Confidence: 0.5},
	}

	err := a.WriteRules(rules, "global", "")
	if err != nil {
		t.Fatalf("write rules: %v", err)
	}

	target := a.TargetPath("global", "")
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	str := string(content)
	if !strings.Contains(str, "Use pnpm not npm") {
		t.Error("should contain active rule")
	}
	if !strings.Contains(str, "Use tabs for indentation") {
		t.Error("should contain second active rule")
	}
	if strings.Contains(str, "Disabled rule") {
		t.Error("should NOT contain disabled rule")
	}
}

func TestAdapterManagedBlockIncludesRuleMetadata(t *testing.T) {
	dir := t.TempDir()
	a := &ClaudeCodeAdapter{homeDir: dir, mb: NewManagedBlock(filepath.Join(dir, "backups"))}
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}

	rules := []*storage.Rule{
		{
			ID:             "rule_meta",
			Content:        "Use pnpm instead of npm",
			Status:         "active",
			Scope:          "project",
			Tags:           []string{"node", "tooling"},
			TriggerContext: "manual correction from Codex",
			Confidence:     0.9,
			Version:        3,
		},
	}

	if err := a.WriteRules(rules, "global", ""); err != nil {
		t.Fatalf("write rules: %v", err)
	}
	content, err := os.ReadFile(a.TargetPath("global", ""))
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	text := string(content)
	for _, want := range []string{
		"Rule: Use pnpm instead of npm",
		"Scope: project",
		"Tags: node, tooling",
		"Trigger: manual correction from Codex",
		"Confidence: 90%",
		"Version: 3",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("managed block missing %q:\n%s", want, text)
		}
	}
}

func TestClaudeCodeAdapterPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	a := &ClaudeCodeAdapter{homeDir: dir, mb: NewManagedBlock(backupDir)}
	os.MkdirAll(filepath.Join(dir, ".claude"), 0755)

	// Pre-existing CLAUDE.md with handwritten content.
	target := filepath.Join(dir, ".claude", "CLAUDE.md")
	os.WriteFile(target, []byte("# My Project\n\nCustom instructions here.\n"), 0644)

	rules := []*storage.Rule{
		{ID: "r1", Content: "Always write tests", Status: "active", Confidence: 0.8},
	}

	if err := a.WriteRules(rules, "global", ""); err != nil {
		t.Fatalf("write: %v", err)
	}

	content, _ := os.ReadFile(target)
	str := string(content)
	if !strings.Contains(str, "Custom instructions here.") {
		t.Error("handwritten content should be preserved")
	}
	if !strings.Contains(str, "Always write tests") {
		t.Error("managed rule should be added")
	}
}

func TestClaudeCodeAdapterRemoveRules(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	a := &ClaudeCodeAdapter{homeDir: dir, mb: NewManagedBlock(backupDir)}
	os.MkdirAll(filepath.Join(dir, ".claude"), 0755)

	// Write rules first.
	rules := []*storage.Rule{{ID: "r1", Content: "Rule", Status: "active", Confidence: 0.8}}
	a.WriteRules(rules, "global", "")

	// Remove.
	if err := a.RemoveRules("global", ""); err != nil {
		t.Fatalf("remove: %v", err)
	}

	target := a.TargetPath("global", "")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("file with only managed block should be removed")
	}
}

func TestCursorAdapterTargetPath(t *testing.T) {
	a := &CursorAdapter{mb: NewManagedBlock(filepath.Join(t.TempDir(), "backups"))}

	projDir := filepath.Join(t.TempDir(), "myproj")
	project := a.TargetPath("project", projDir)
	if !strings.Contains(project, ".cursorrules") {
		t.Errorf("project path should contain .cursorrules: %q", project)
	}
}

func TestOpenClawAdapterWriteAndRemove(t *testing.T) {
	dir := t.TempDir()
	a := NewOpenClawAdapter(filepath.Join(dir, "backups"))

	projectPath := filepath.Join(dir, "project")
	rules := []*storage.Rule{
		{ID: "r1", Content: "Use pnpm not npm", Status: "active", Confidence: 0.9},
		{ID: "r2", Content: "Disabled rule", Status: "disabled", Confidence: 0.4},
	}
	if err := a.WriteRules(rules, "project", projectPath); err != nil {
		t.Fatalf("write: %v", err)
	}

	target := filepath.Join(projectPath, "OPENCLAW.md")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read OPENCLAW.md: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "Use pnpm not npm") {
		t.Error("OPENCLAW.md should contain active rule")
	}
	if strings.Contains(text, "Disabled rule") {
		t.Error("OPENCLAW.md should not contain disabled rules")
	}

	if err := a.RemoveRules("project", projectPath); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("OPENCLAW.md with only managed block should be removed, stat err = %v", err)
	}
}

func TestCopilotAdapterTargetPath(t *testing.T) {
	a := &CopilotAdapter{mb: NewManagedBlock(filepath.Join(t.TempDir(), "backups"))}

	projDir := filepath.Join(t.TempDir(), "myproj")
	project := a.TargetPath("project", projDir)
	want := filepath.Join(".github", "copilot-instructions.md")
	if !strings.Contains(project, want) {
		t.Errorf("project path should contain %s: %q", want, project)
	}

	if !a.IsInstalled() {
		t.Error("Copilot IsInstalled should be true (treats Copilot as always-available)")
	}
}

func TestCopilotAdapterWriteAndRemove(t *testing.T) {
	dir := t.TempDir()
	a := NewCopilotAdapter(filepath.Join(dir, "backups"))

	rules := []*storage.Rule{
		{ID: "r1", Content: "Use pnpm not npm", Status: "active", Confidence: 0.9},
	}
	if err := a.WriteRules(rules, "project", dir); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Verify file exists with managed block markers.
	target := a.TargetPath("project", dir)
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "# >>> shadow managed >>>") {
		t.Error("managed block marker missing")
	}
	if !strings.Contains(string(data), "Use pnpm not npm") {
		t.Error("rule content missing")
	}

	// Remove and verify managed block is gone.
	if err := a.RemoveRules("project", dir); err != nil {
		t.Fatalf("remove: %v", err)
	}
	data, _ = os.ReadFile(target)
	if strings.Contains(string(data), "Use pnpm not npm") {
		t.Error("rule should be gone after RemoveRules")
	}
}

func TestRulesToEntries(t *testing.T) {
	rules := []*storage.Rule{
		{
			ID:             "active_rule",
			Content:        "Active",
			Scope:          "project",
			Tags:           []string{"one", "two"},
			TriggerContext: "manual correction",
			Status:         "active",
			Confidence:     0.9,
			Version:        2,
		},
		{Content: "Candidate", Status: "candidate", Confidence: 0.7},
		{Content: "Disabled", Status: "disabled", Confidence: 0.5},
	}

	entries := rulesToEntries(rules)
	if len(entries) != 1 {
		t.Errorf("only active rules, got %d entries", len(entries))
	}
	if entries[0].Content != "Active" {
		t.Errorf("entry content: %q", entries[0].Content)
	}
	if entries[0].ID != "active_rule" || entries[0].Version != 2 {
		t.Errorf("entry metadata: %#v", entries[0])
	}
	if strings.Join(entries[0].Tags, ",") != "one,two" {
		t.Errorf("entry tags: %#v", entries[0].Tags)
	}
	if entries[0].TriggerContext != "manual correction" {
		t.Errorf("entry trigger: %q", entries[0].TriggerContext)
	}
}
