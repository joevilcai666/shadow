package capture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScannerDetectPackageManager(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
		{"bun.lockb", "bun"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, tt.filename), []byte(""), 0644)
			s := NewScanner(dir)
			r, _ := s.Scan()
			if r.PackageManager != tt.want {
				t.Errorf("got %q, want %q", r.PackageManager, tt.want)
			}
		})
	}
}

func TestScannerDetectTestFramework(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "vitest.config.ts"), []byte("export default {}"), 0644)

	s := NewScanner(dir)
	r, _ := s.Scan()
	if r.TestFramework != "Vitest" {
		t.Errorf("got %q, want Vitest", r.TestFramework)
	}
}

func TestScannerDetectLanguage(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test"), 0644)

	s := NewScanner(dir)
	r, _ := s.Scan()
	if r.Language != "Go" {
		t.Errorf("got %q, want Go", r.Language)
	}
}

func TestScannerFindExistingRules(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Rules\nUse pnpm"), 0644)
	os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("Use tabs"), 0644)

	s := NewScanner(dir)
	r, _ := s.Scan()
	if len(r.ExistingRules) != 2 {
		t.Errorf("expected 2 existing rule files, got %d", len(r.ExistingRules))
	}
}

func TestScannerEmptyDir(t *testing.T) {
	dir := t.TempDir()
	s := NewScanner(dir)
	r, err := s.Scan()
	if err != nil {
		t.Fatalf("empty dir scan: %v", err)
	}
	if r.PackageManager != "" {
		t.Error("empty dir should have no package manager")
	}
}

func TestScanResultToRules(t *testing.T) {
	r := &ScanResult{
		PackageManager: "pnpm",
		TestFramework:  "Vitest",
		Language:       "TypeScript/JavaScript",
	}
	rules := r.ToRules()
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	for _, rule := range rules {
		if rule.Status != "candidate" {
			t.Errorf("rule status: got %q", rule.Status)
		}
		if rule.Confidence <= 0 {
			t.Error("confidence should be > 0")
		}
	}
}

func TestImporterImportFile(t *testing.T) {
	dir := t.TempDir()
	content := `# My Project Rules

Use pnpm for all package management.

Always write tests for new functions.

Follow the existing code style.

## Architecture

Keep controllers thin, logic in services.
`
	filePath := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(filePath, []byte(content), 0644)

	imp := NewImporter()
	rules, err := imp.ImportFile(filePath)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if len(rules) == 0 {
		t.Fatal("should import at least 1 rule")
	}

	// Verify imported rules have proper metadata.
	for _, rule := range rules {
		if rule.Content == "" {
			t.Error("rule should have content")
		}
		if rule.Confidence != 1.0 {
			t.Errorf("imported rule confidence should be 1.0, got %f", rule.Confidence)
		}
		if rule.Status != "candidate" {
			t.Errorf("status should be candidate, got %q", rule.Status)
		}
	}
}

func TestImporterEmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.md")
	os.WriteFile(filePath, []byte(""), 0644)

	imp := NewImporter()
	rules, err := imp.ImportFile(filePath)
	if err != nil {
		t.Fatalf("import empty: %v", err)
	}
	if rules != nil {
		t.Error("empty file should return nil rules")
	}
}

func TestImporterSkipsManagedBlock(t *testing.T) {
	dir := t.TempDir()
	content := `# My Rules

My custom rule here.

# >>> shadow managed >>>
# Auto-managed rule
# <<< shadow managed <<<

Another hand-written rule.
`
	filePath := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(filePath, []byte(content), 0644)

	imp := NewImporter()
	rules, _ := imp.ImportFile(filePath)

	for _, rule := range rules {
		if rule.Content == "Auto-managed rule" {
			t.Error("should skip managed block content")
		}
	}
}
