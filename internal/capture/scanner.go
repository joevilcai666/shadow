package capture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joevilcai666/shadow/internal/storage"
)

// Scanner scans a project repository to infer initial rules.
type Scanner struct {
	projectPath string
}

// NewScanner creates a new project scanner.
func NewScanner(projectPath string) *Scanner {
	return &Scanner{projectPath: projectPath}
}

// ScanResult contains discovered project facts.
type ScanResult struct {
	PackageManager string
	TestFramework  string
	Language       string
	Framework      string
	ExistingRules  []string // Paths to existing rule files
	Facts          []string // Human-readable facts discovered
}

// Scan performs a quick scan of the project directory.
func (s *Scanner) Scan() (*ScanResult, error) {
	result := &ScanResult{}

	// Detect package manager.
	s.detectPackageManager(result)

	// Detect test framework.
	s.detectTestFramework(result)

	// Detect language/framework.
	s.detectLanguage(result)

	// Find existing rule files.
	s.findExistingRules(result)

	return result, nil
}

// ToRules converts scan results into candidate rules.
func (r *ScanResult) ToRules() []*storage.Rule {
	var rules []*storage.Rule

	if r.PackageManager != "" {
		rules = append(rules, &storage.Rule{
			ID:         storage.NewID(),
			Content:    fmt.Sprintf("This project uses %s for package management", r.PackageManager),
			Scope:      "project",
			ProjectPath: r.ProjectPath(),
			Tags:       []string{"toolchain", "auto-generated"},
			Category:   "toolchain",
			Confidence: 0.9,
			Status:     "candidate",
			Version:    1,
			CreatedAt:  storage.Now(),
			UpdatedAt:  storage.Now(),
		})
	}

	if r.TestFramework != "" {
		rules = append(rules, &storage.Rule{
			ID:         storage.NewID(),
			Content:    fmt.Sprintf("This project uses %s for testing", r.TestFramework),
			Scope:      "project",
			ProjectPath: r.ProjectPath(),
			Tags:       []string{"testing", "auto-generated"},
			Category:   "testing",
			Confidence: 0.85,
			Status:     "candidate",
			Version:    1,
			CreatedAt:  storage.Now(),
			UpdatedAt:  storage.Now(),
		})
	}

	if r.Language != "" {
		rules = append(rules, &storage.Rule{
			ID:         storage.NewID(),
			Content:    fmt.Sprintf("This project is written in %s", r.Language),
			Scope:      "project",
			ProjectPath: r.ProjectPath(),
			Tags:       []string{"language", "auto-generated"},
			Category:   "general",
			Confidence: 0.95,
			Status:     "candidate",
			Version:    1,
			CreatedAt:  storage.Now(),
			UpdatedAt:  storage.Now(),
		})
	}

	return rules
}

func (r *ScanResult) ProjectPath() string {
	// Use the first existing rules file's directory as a heuristic.
	// Or return empty for global scope.
	return ""
}

func (s *Scanner) detectPackageManager(r *ScanResult) {
	checks := map[string]string{
		"pnpm-lock.yaml":   "pnpm",
		"yarn.lock":        "yarn",
		"bun.lockb":        "bun",
		"package-lock.json": "npm",
	}

	for file, pm := range checks {
		if _, err := os.Stat(filepath.Join(s.projectPath, file)); err == nil {
			r.PackageManager = pm
			r.Facts = append(r.Facts, fmt.Sprintf("Package manager: %s (detected from %s)", pm, file))
			return
		}
	}
}

func (s *Scanner) detectTestFramework(r *ScanResult) {
	checks := map[string]string{
		"vitest.config.ts":  "Vitest",
		"vitest.config.js":  "Vitest",
		"jest.config.ts":    "Jest",
		"jest.config.js":    "Jest",
		"pytest.ini":        "pytest",
		"Cargo.toml":        "cargo test",
		"go.mod":            "go test",
	}

	for file, fw := range checks {
		if _, err := os.Stat(filepath.Join(s.projectPath, file)); err == nil {
			r.TestFramework = fw
			r.Facts = append(r.Facts, fmt.Sprintf("Test framework: %s (detected from %s)", fw, file))
			return
		}
	}
}

func (s *Scanner) detectLanguage(r *ScanResult) {
	checks := map[string]string{
		"go.mod":         "Go",
		"Cargo.toml":     "Rust",
		"pyproject.toml": "Python",
		"Gemfile":        "Ruby",
		"package.json":   "TypeScript/JavaScript",
	}

	for file, lang := range checks {
		if _, err := os.Stat(filepath.Join(s.projectPath, file)); err == nil {
			r.Language = lang
			r.Facts = append(r.Facts, fmt.Sprintf("Language: %s (detected from %s)", lang, file))
			return
		}
	}
}

func (s *Scanner) findExistingRules(r *ScanResult) {
	ruleFiles := []string{
		"CLAUDE.md",
		".claude/CLAUDE.md",
		".cursorrules",
		".cursor/rules",
		"AGENTS.md",
		".github/copilot-instructions.md",
	}

	for _, f := range ruleFiles {
		path := filepath.Join(s.projectPath, f)
		if _, err := os.Stat(path); err == nil {
			r.ExistingRules = append(r.ExistingRules, path)
			r.Facts = append(r.Facts, fmt.Sprintf("Existing rules: %s", f))
		}
	}
}

// --- SHADOW-015: Rule File Importer ---

// Importer imports existing rule files into Shadow's rule system.
type Importer struct{}

// NewImporter creates a new rule importer.
func NewImporter() *Importer {
	return &Importer{}
}

// ImportFile reads a rule file and converts its content into candidate rules.
func (imp *Importer) ImportFile(filePath string) ([]*storage.Rule, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}

	// Parse the file content into rules.
	var rules []*storage.Rule
	lines := strings.Split(content, "\n")

	var currentRule strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and markdown headings.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") && !strings.Contains(trimmed, "shadow") {
			if currentRule.Len() > 0 {
				rule := imp.createRuleFromText(currentRule.String(), filePath)
				rules = append(rules, rule)
				currentRule.Reset()
			}
			continue
		}

		// Skip managed block markers.
		if strings.Contains(trimmed, "shadow managed") || strings.Contains(trimmed, "shadow hook") {
			continue
		}

		// Skip comment-only lines.
		cleaned := strings.TrimLeft(trimmed, "#- >")
		cleaned = strings.TrimSpace(cleaned)
		if cleaned == "" {
			continue
		}

		if currentRule.Len() > 0 {
			currentRule.WriteString(" ")
		}
		currentRule.WriteString(cleaned)
	}

	// Don't forget the last rule.
	if currentRule.Len() > 0 {
		rule := imp.createRuleFromText(currentRule.String(), filePath)
		rules = append(rules, rule)
	}

	return rules, nil
}

func (imp *Importer) createRuleFromText(text, sourceFile string) *storage.Rule {
	return &storage.Rule{
		ID:         storage.NewID(),
		Content:    text,
		Scope:      "project",
		Tags:       []string{"imported"},
		Category:   categorizeImported(text),
		Confidence: 1.0, // User has already verified this by having it in their file.
		Status:     "candidate",
		Version:    1,
		CreatedAt:  storage.Now(),
		UpdatedAt:  storage.Now(),
	}
}

func categorizeImported(text string) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "test") {
		return "testing"
	}
	if strings.Contains(lower, "style") || strings.Contains(lower, "format") || strings.Contains(lower, "indent") {
		return "code-style"
	}
	if strings.Contains(lower, "npm") || strings.Contains(lower, "pnpm") || strings.Contains(lower, "yarn") {
		return "toolchain"
	}
	if strings.Contains(lower, "architecture") || strings.Contains(lower, "pattern") {
		return "architecture"
	}
	return "general"
}
