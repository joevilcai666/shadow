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

func TestScanner_FiltersDotEnv(t *testing.T) {
	dir := t.TempDir()

	// Create the "noise" — sensitive files that must be filtered.
	sensitiveFiles := []string{
		".env",
		".env.local",
		".env.production",
		"prod.env",
		"id_rsa",
		"id_rsa.pub",
		"server.pem",
		"tls.key",
		"service.p12",
		".netrc",
		"credentials.json",
	}
	for _, name := range sensitiveFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("placeholder"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Add a benign rule file the scanner SHOULD find.
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# My rules"), 0644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	s := NewScanner(dir)
	r, err := s.Scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	// Sensitive files must show up in the SensitiveFiles list — that's how
	// the host system (and tests) verify the filter ran end-to-end.
	if len(r.SensitiveFiles) < len(sensitiveFiles) {
		t.Errorf("expected to find %d sensitive files, got %d", len(sensitiveFiles), len(r.SensitiveFiles))
	}

	// And NONE of them should appear in ExistingRules.
	for _, sr := range r.ExistingRules {
		base := filepath.Base(sr)
		for _, sensitive := range sensitiveFiles {
			if base == sensitive {
				t.Errorf("sensitive file %s leaked into ExistingRules", base)
			}
		}
	}

	// The Importer should refuse to read these files even if asked.
	imp := NewImporter()
	for _, sensitive := range sensitiveFiles {
		path := filepath.Join(dir, sensitive)
		rules, _ := imp.ImportFile(path)
		if len(rules) > 0 {
			t.Errorf("importer should not import rules from %s, got %d", sensitive, len(rules))
		}
	}
}

func TestScanner_FiltersSecrets(t *testing.T) {
	dir := t.TempDir()

	// A benign CLAUDE.md that should be imported normally.
	benign := `# Project rules

Use pnpm for all package management.
Always write tests for new functions.
Follow the existing code style.
`
	benignPath := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(benignPath, []byte(benign), 0644); err != nil {
		t.Fatalf("write benign: %v", err)
	}

	// A CLAUDE.md seeded with secret patterns. None of these rules
	// should be imported — the content is poisoned.
	secretCases := map[string]string{
		"claude_with_openai.md": `# Rules
Never log OpenAI keys.
Production key is sk-proj-abc123def456ghi789jkl012mno345pqr
Follow the existing code style.
`,
		"claude_with_github_pat.md": `# Rules
The CI bot uses GITHUB_TOKEN=ghp_abc123def456ghi789jkl012mno345pqr
Use tabs.
`,
		"claude_with_google.md": `# Rules
Maps API key: AIzaSyAbc123def456ghi789jkl012mno345pqr
Use tabs.
`,
		"claude_with_aws.md": `# Rules
AWS_ACCESS_KEY_ID=AKIAABCDEFGHIJKLMNOP
Use tabs.
`,
		"claude_with_pem.md": `# Rules
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA...
-----END RSA PRIVATE KEY-----
Use tabs.
`,
	}
	for name, content := range secretCases {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// The benign file imports normally.
	imp := NewImporter()
	rules, err := imp.ImportFile(benignPath)
	if err != nil {
		t.Fatalf("import benign: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("benign CLAUDE.md should import at least one rule")
	}

	// Each poisoned file should produce ZERO rules.
	for name := range secretCases {
		path := filepath.Join(dir, name)
		rules, err := imp.ImportFile(path)
		if err != nil {
			t.Fatalf("import %s: %v", name, err)
		}
		if len(rules) > 0 {
			t.Errorf("%s should be filtered (contained a secret pattern), got %d rules", name, len(rules))
		}
	}

	// Sanity-check: containsSecret helper is doing what we think.
	if !containsSecret("hello sk-proj-abc123 world") {
		t.Error("containsSecret should detect sk-... pattern")
	}
	if containsSecret("Use tabs for indentation") {
		t.Error("containsSecret should NOT flag plain English")
	}
}
