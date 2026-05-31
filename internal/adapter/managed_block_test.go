package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testManagedBlock(t *testing.T) (*ManagedBlock, string) {
	t.Helper()
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	mb := NewManagedBlock(backupDir)
	return mb, dir
}

func TestWriteToNewFile(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	rules := []RuleEntry{
		{Content: "Use pnpm, not npm", Confidence: 0.9},
		{Content: "Use tabs for indentation", Confidence: 0.7},
	}

	result, err := mb.Write(filePath, rules)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !result.Verified {
		t.Error("verification should pass")
	}

	content, _ := os.ReadFile(filePath)
	str := string(content)

	if !strings.Contains(str, blockBegin) {
		t.Error("should contain block begin marker")
	}
	if !strings.Contains(str, blockEnd) {
		t.Error("should contain block end marker")
	}
	if !strings.Contains(str, "Use pnpm, not npm") {
		t.Error("should contain first rule")
	}
	if !strings.Contains(str, "Use tabs for indentation") {
		t.Error("should contain second rule")
	}
}

func TestWriteToExistingFileWithHandwrittenContent(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	// Pre-existing handwritten content.
	os.WriteFile(filePath, []byte("# My Project\n\nThis is my custom content.\n"), 0644)

	rules := []RuleEntry{{Content: "Always write tests", Confidence: 0.85}}

	result, err := mb.Write(filePath, rules)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !result.Verified {
		t.Error("verification should pass")
	}

	content, _ := os.ReadFile(filePath)
	str := string(content)

	if !strings.Contains(str, "# My Project") {
		t.Error("handwritten content should be preserved")
	}
	if !strings.Contains(str, "This is my custom content.") {
		t.Error("handwritten content should be preserved")
	}
	if !strings.Contains(str, "Always write tests") {
		t.Error("managed block should contain the rule")
	}
}

func TestWriteToExistingFileWithManagedBlock(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	// File with existing managed block.
	initial := "# My Project\n\n" + blockBegin + "\n# Old rule\n" + blockEnd + "\n"
	os.WriteFile(filePath, []byte(initial), 0644)

	rules := []RuleEntry{{Content: "New rule", Confidence: 0.9}}

	result, err := mb.Write(filePath, rules)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !result.Verified {
		t.Error("verification should pass")
	}

	content, _ := os.ReadFile(filePath)
	str := string(content)

	if !strings.Contains(str, "# My Project") {
		t.Error("handwritten content should be preserved")
	}
	if strings.Contains(str, "Old rule") {
		t.Error("old rule should be replaced")
	}
	if !strings.Contains(str, "New rule") {
		t.Error("new rule should be written")
	}
}

func TestRemoveManagedBlock(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	content := "# My Project\n\nSome instructions.\n\n" + blockBegin + "\n# Rule here\n" + blockEnd + "\n"
	os.WriteFile(filePath, []byte(content), 0644)

	if err := mb.Remove(filePath); err != nil {
		t.Fatalf("remove: %v", err)
	}

	result, _ := os.ReadFile(filePath)
	str := string(result)

	if strings.Contains(str, blockBegin) {
		t.Error("managed block should be removed")
	}
	if !strings.Contains(str, "# My Project") {
		t.Error("handwritten content should remain")
	}
}

func TestRemoveManagedBlockFromManagedOnlyFile(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	// File with only managed block.
	content := blockBegin + "\n# Rule\n" + blockEnd + "\n"
	os.WriteFile(filePath, []byte(content), 0644)

	if err := mb.Remove(filePath); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file with only managed block should be deleted")
	}
}

func TestRulesSortedByConfidence(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	rules := []RuleEntry{
		{Content: "Low confidence rule", Confidence: 0.3},
		{Content: "High confidence rule", Confidence: 0.95},
		{Content: "Medium confidence rule", Confidence: 0.6},
	}

	result, err := mb.Write(filePath, rules)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !result.Verified {
		t.Error("verification should pass")
	}

	content, _ := os.ReadFile(filePath)
	str := string(content)

	// High should come before medium, medium before low.
	highIdx := strings.Index(str, "High confidence rule")
	medIdx := strings.Index(str, "Medium confidence rule")
	lowIdx := strings.Index(str, "Low confidence rule")

	if highIdx > medIdx {
		t.Error("high confidence rule should come before medium")
	}
	if medIdx > lowIdx {
		t.Error("medium confidence rule should come before low")
	}
}

func TestAtomicWriteInterruption(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	// Write initial content.
	os.WriteFile(filePath, []byte("# Original\n"), 0644)

	// Write managed block.
	_, err := mb.Write(filePath, []RuleEntry{{Content: "Rule 1", Confidence: 0.8}})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Verify temp file is cleaned up.
	tmpFile := filepath.Join(dir, ".shadow-tmp-CLAUDE.md")
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("temp file should be cleaned up after atomic write")
	}
}

func TestBackupCreated(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	os.WriteFile(filePath, []byte("# Existing content\n"), 0644)

	result, err := mb.Write(filePath, []RuleEntry{{Content: "Rule", Confidence: 0.8}})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	if result.BackupPath == "" {
		t.Error("backup should be created for existing files")
	}
	if _, err := os.Stat(result.BackupPath); os.IsNotExist(err) {
		t.Error("backup file should exist")
	}

	// Backup should contain original content.
	backup, _ := os.ReadFile(result.BackupPath)
	if !strings.Contains(string(backup), "# Existing content") {
		t.Error("backup should contain original content")
	}
}

func TestMalformedBlockEndBeforeBegin(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	// Malformed: end before begin.
	content := blockEnd + "\n# some content\n" + blockBegin + "\n"
	os.WriteFile(filePath, []byte(content), 0644)

	_, err := mb.Write(filePath, []RuleEntry{{Content: "Rule", Confidence: 0.8}})
	if err == nil {
		t.Error("should error on malformed block (end before begin)")
	}
}

func TestMissingEndMarker(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	content := blockBegin + "\n# some rule\n"
	os.WriteFile(filePath, []byte(content), 0644)

	_, err := mb.Write(filePath, []RuleEntry{{Content: "Rule", Confidence: 0.8}})
	if err == nil {
		t.Error("should error on missing end marker")
	}
}

func TestEmptyFileCreatesBlock(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	// Create empty file.
	os.WriteFile(filePath, []byte(""), 0644)

	result, err := mb.Write(filePath, []RuleEntry{{Content: "Rule 1", Confidence: 0.9}})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !result.Verified {
		t.Error("verification should pass")
	}

	content, _ := os.ReadFile(filePath)
	if !strings.Contains(string(content), "Rule 1") {
		t.Error("should contain rule in new file")
	}
}

func TestSpecialCharacters(t *testing.T) {
	mb, dir := testManagedBlock(t)
	filePath := filepath.Join(dir, "CLAUDE.md")

	os.WriteFile(filePath, []byte("# Project with 中文 and emoji 🚀\n"), 0644)

	rules := []RuleEntry{{Content: "Use 中文注释 when possible", Confidence: 0.9}}
	result, err := mb.Write(filePath, rules)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !result.Verified {
		t.Error("verification should pass with special chars")
	}

	content, _ := os.ReadFile(filePath)
	if !strings.Contains(string(content), "中文") {
		t.Error("special chars should be preserved")
	}
}
