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
	if !strings.Contains(global, ".claude/CLAUDE.md") {
		t.Errorf("global path: %q", global)
	}

	project := a.TargetPath("project", "/tmp/myproj")
	if project != "/tmp/myproj/CLAUDE.md" {
		t.Errorf("project path: %q", project)
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

	project := a.TargetPath("project", "/tmp/myproj")
	if project != "/tmp/myproj/.cursorrules" {
		t.Errorf("project path: %q", project)
	}
}

func TestCopilotAdapterTargetPath(t *testing.T) {
	a := &CopilotAdapter{mb: NewManagedBlock(filepath.Join(t.TempDir(), "backups"))}

	project := a.TargetPath("project", "/tmp/myproj")
	if project != "/tmp/myproj/.github/copilot-instructions.md" {
		t.Errorf("project path: %q, want /tmp/myproj/.github/copilot-instructions.md", project)
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
		{Content: "Active", Status: "active", Confidence: 0.9},
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
}
