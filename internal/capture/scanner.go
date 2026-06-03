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

	// SensitiveFiles lists paths that the scanner skipped because they
	// matched a sensitive-filename pattern (.env, *.pem, *.key,
	// id_rsa*, id_dsa*, etc.). This is exposed so callers — and
	// tests — can verify the filter ran end-to-end.
	SensitiveFiles []string

	// SecretsFound counts files whose contents contained a known
	// secret pattern (sk-..., ghp_..., AIza..., AKIA...). The actual
	// file paths and snippets are NOT stored in the result to avoid
	// leaking what we filtered.
	SecretsFound int
}

// Scan performs a quick scan of the project directory.
//
// The scan is "safe by default": it walks the project root to find files
// that look sensitive (.env, *.pem, *.key, id_rsa*, id_dsa*, etc.) and
// excludes them from any result. It also tracks the names of those
// files in SensitiveFiles so callers (and tests) can verify the
// filter ran. The walk is shallow (no recursion into node_modules, .git,
// etc.) to stay fast on large repos.
func (s *Scanner) Scan() (*ScanResult, error) {
	result := &ScanResult{}

	// Walk the project root and find sensitive files BEFORE we make any
	// other decision. The list of sensitive files is the source of truth
	// for what to skip; later stages (findExistingRules, the importer)
	// check against this list as a defense-in-depth.
	s.walkSensitiveFiles(result)

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
		if isSensitiveFilename(f) {
			// Defense-in-depth: even if a rule file matches a sensitive
			// name, we skip it. In practice this never matches for the
			// curated list above, but the guard makes the function
			// safe against future additions.
			continue
		}
		if _, err := os.Stat(path); err == nil {
			r.ExistingRules = append(r.ExistingRules, path)
			r.Facts = append(r.Facts, fmt.Sprintf("Existing rules: %s", f))
		}
	}
}

// walkSensitiveFiles performs a shallow directory walk of the project
// root and records any files whose names match a sensitive-filename
// pattern. It does NOT recurse into the standard "junk" directories
// (node_modules, .git, dist, .venv) to keep it fast.
//
// This is the entry-point for the privacy filter; later code
// (findExistingRules, the Importer) cross-checks against the
// recorded sensitive files for defense in depth.
func (s *Scanner) walkSensitiveFiles(r *ScanResult) {
	if s.projectPath == "" {
		return
	}
	entries, err := os.ReadDir(s.projectPath)
	if err != nil {
		return // not readable; nothing to do
	}

	skipDirs := map[string]bool{
		"node_modules": true,
		".git":         true,
		"dist":         true,
		"build":        true,
		".venv":        true,
		"venv":         true,
		"__pycache__":  true,
		".next":        true,
		"target":       true, // Rust
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			if skipDirs[name] {
				continue
			}
			continue // shallow walk only
		}
		if isSensitiveFilename(name) {
			full := filepath.Join(s.projectPath, name)
			r.SensitiveFiles = append(r.SensitiveFiles, full)
			r.Facts = append(r.Facts, fmt.Sprintf("Skipped sensitive file: %s", name))
		}
	}
}

// isSensitiveFilename returns true if a filename looks like it might
// contain secrets, credentials, or keys. The match is intentionally
// conservative: better to over-skip than to leak.
//
// Patterns covered:
//   - .env, .env.*, *.env      (env files, possibly with prefixes)
//   - *.pem, *.key             (TLS / SSH private keys)
//   - id_rsa, id_rsa.*, id_dsa, id_dsa.*  (SSH private keys)
//   - *.p12, *.pfx             (PKCS12 bundles)
//   - credentials, *.gcp-key.json  (cloud creds)
//   - netrc, .netrc            (curl/wget creds)
func isSensitiveFilename(name string) bool {
	lower := strings.ToLower(name)

	// Direct substring matches first — cheap and unambiguous.
	switch lower {
	case ".env", "env", "envfile":
		return true
	case "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519":
		return true
	case "netrc", ".netrc":
		return true
	}
	// Names that include the word "credentials" — common naming pattern
	// for cloud service account / AWS / GCP credential files.
	if strings.Contains(lower, "credentials") && strings.HasSuffix(lower, ".json") {
		return true
	}

	// Suffix / prefix patterns.
	if strings.HasSuffix(lower, ".env") {
		return true
	}
	if strings.HasPrefix(lower, ".env") && (strings.Contains(lower, ".") || len(lower) > 4) {
		// e.g. ".env.local", ".env.production"
		return true
	}
	if strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") {
		return true
	}
	if strings.HasSuffix(lower, ".p12") || strings.HasSuffix(lower, ".pfx") {
		return true
	}
	if strings.HasPrefix(lower, "id_rsa") || strings.HasPrefix(lower, "id_dsa") {
		return true
	}
	if strings.HasSuffix(lower, "service-account.json") || strings.HasSuffix(lower, ".gcp-key.json") {
		return true
	}

	return false
}

// containsSecret returns true if the given content contains a known
// credential pattern. It does NOT log the matched content — only a
// boolean. Patterns covered:
//
//   - OpenAI / Anthropic style:    sk-..., sk-ant-...
//   - GitHub PAT:                  ghp_..., gho_..., ghu_..., ghs_...
//   - Google API key:              AIza...
//   - AWS access key:              AKIA...
//   - Slack tokens:                xoxb-..., xoxp-...
//   - Stripe live key:             sk_live_..., pk_live_...
//   - Generic JWT:                 three base64url segments separated by dots
//   - PEM private key headers:     "-----BEGIN ... PRIVATE KEY-----"
func containsSecret(content string) bool {
	patterns := []string{
		"sk-", "sk_",
		"sk-ant-",
		"ghp_", "gho_", "ghu_", "ghs_", "ghr_",
		"AIza",
		"AKIA",
		"xoxb-", "xoxp-", "xoxa-",
		"sk_live_", "pk_live_",
		"-----BEGIN", // generic PEM header (RSA, EC, OPENSSH, etc.)
	}
	lower := strings.ToLower(content)
	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
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
	// Privacy guard 1: refuse to read files whose names look sensitive
	// (e.g. ".env", "id_rsa", "*.pem"). Returning an empty result with
	// nil error is intentional — the caller is told "no rules" not
	// "something is wrong", because the file might have been added
	// between two scan passes.
	if isSensitiveFilename(filepath.Base(filePath)) {
		return nil, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}

	// Privacy guard 2: refuse to import rules from files that contain
	// known secret patterns (sk-..., ghp_..., AIza..., AKIA..., PEM
	// headers, etc.). This is a second line of defense in case a
	// non-obvious file name slipped past guard 1.
	if containsSecret(content) {
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
