package distill

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/joevilcai666/shadow/internal/storage"
)

// Distiller is the interface for rule distillation backends.
type Distiller interface {
	Distill(sources []*storage.Source) (*CandidateRule, error)
	CheckSimilarity(newContent string, existingRules []*storage.Rule) (*storage.Rule, error)
	DetectConflict(newContent string, existingRules []*storage.Rule) bool
}

// CandidateRule is the output of distillation.
type CandidateRule struct {
	Content        string   `json:"content"`
	Category       string   `json:"category"`
	TriggerContext string   `json:"trigger_context"`
	Confidence     float64  `json:"confidence"`
	Tags           []string `json:"tags"`
}

// Engine converts captured signals into candidate rules.
type Engine struct {
	mu       sync.Mutex
	distiller Distiller
	ruleDB    *storage.RuleRepo
	sourceDB  *storage.SourceRepo
	threshold string // "low", "medium", "high"
}

// NewEngine creates a new distillation engine.
func NewEngine(distiller Distiller, ruleDB *storage.RuleRepo, sourceDB *storage.SourceRepo, threshold string) *Engine {
	return &Engine{
		distiller: distiller,
		ruleDB:    ruleDB,
		sourceDB:  sourceDB,
		threshold: threshold,
	}
}

// ProcessSignals processes new signals and generates candidate rules if threshold met.
func (e *Engine) ProcessSignals(sources []*storage.Source) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(sources) == 0 {
		return nil
	}

	// Check if threshold is met.
	if !e.meetsThreshold(sources) {
		slog.Debug("signals below threshold, skipping distillation", "count", len(sources))
		return nil
	}

	// Distill into a candidate rule.
	candidate, err := e.distiller.Distill(sources)
	if err != nil {
		return fmt.Errorf("distill: %w", err)
	}

	if candidate == nil {
		return nil
	}

	// Check for similar existing rules.
	existing, err := e.ruleDB.List(storage.RuleFilter{Status: "active"})
	if err != nil {
		return fmt.Errorf("fetch existing rules: %w", err)
	}

	similar, err := e.distiller.CheckSimilarity(candidate.Content, existing)
	if err != nil {
		slog.Warn("similarity check failed", "error", err)
	}

	if similar != nil {
		// Merge: boost confidence, update the existing rule.
		slog.Info("merging with existing similar rule", "rule_id", similar.ID)
		similar.Confidence = min(similar.Confidence+0.1, 1.0)
		similar.UpdatedAt = storage.Now()
		return e.ruleDB.Update(similar, "auto", "merged similar signals")
	}

	// Check for conflicts.
	if e.distiller.DetectConflict(candidate.Content, existing) {
		slog.Info("new rule conflicts with existing", "content", candidate.Content)
		// Create as conflicted.
		return e.createRule(candidate, sources, "conflicted")
	}

	// Create as candidate.
	return e.createRule(candidate, sources, "candidate")
}

// ProcessExplicitSignal handles a strong explicit signal that should immediately create a rule.
func (e *Engine) ProcessExplicitSignal(source *storage.Source) error {
	sources := []*storage.Source{source}
	candidate, err := e.distiller.Distill(sources)
	if err != nil {
		// Fallback: use raw content as rule.
		candidate = &CandidateRule{
			Content:    source.RawSnippet,
			Category:   "uncategorized",
			Confidence: source.ConfidenceContribution,
			Tags:       []string{},
		}
	}

	return e.createRule(candidate, sources, "candidate")
}

func (e *Engine) createRule(candidate *CandidateRule, sources []*storage.Source, status string) error {
	rule := &storage.Rule{
		ID:             storage.NewID(),
		Content:        candidate.Content,
		Scope:          "global",
		Tags:           candidate.Tags,
		Category:       candidate.Category,
		TriggerContext: candidate.TriggerContext,
		Confidence:     candidate.Confidence,
		Status:         status,
		Version:        1,
		CreatedAt:      storage.Now(),
		UpdatedAt:      storage.Now(),
	}

	if rule.Tags == nil {
		rule.Tags = []string{}
	}

	// Determine scope from sources.
	for _, s := range sources {
		if s.ProjectPath != "" {
			rule.Scope = "project"
			rule.ProjectPath = s.ProjectPath
			break
		}
	}

	if err := e.ruleDB.Create(rule); err != nil {
		return fmt.Errorf("create rule: %w", err)
	}

	// Link sources to the rule.
	for _, s := range sources {
		if err := e.sourceDB.LinkToRule(s.ID, rule.ID); err != nil {
			slog.Warn("link source to rule failed", "source_id", s.ID, "rule_id", rule.ID, "error", err)
			continue
		}
		slog.Debug("linked source to rule", "source_id", s.ID, "rule_id", rule.ID)
	}

	slog.Info("created candidate rule",
		"rule_id", rule.ID,
		"content", truncate(rule.Content, 60),
		"status", status,
		"confidence", rule.Confidence,
	)

	return nil
}

func (e *Engine) meetsThreshold(sources []*storage.Source) bool {
	thresholdCount := map[string]int{
		"low":    1,
		"medium": 2,
		"high":   5,
	}

	required := thresholdCount[e.threshold]
	if required == 0 {
		required = 2
	}

	strongCount := 0
	for _, s := range sources {
		if s.SignalStrength == "strong" {
			strongCount++
		}
	}

	// Strong signals count double.
	effectiveCount := strongCount + (len(sources) - strongCount)
	return effectiveCount >= required || strongCount >= 1
}

// --- Rule-based Distiller (MVP fallback, no LLM needed) ---

// RuleBasedDistiller is a simple rule-based distiller that works without LLM.
type RuleBasedDistiller struct{}

// NewRuleBasedDistiller creates a rule-based distiller.
func NewRuleBasedDistiller() *RuleBasedDistiller {
	return &RuleBasedDistiller{}
}

// Distill extracts a candidate rule from signals using rule-based logic.
func (d *RuleBasedDistiller) Distill(sources []*storage.Source) (*CandidateRule, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	// Aggregate content from all sources.
	var contents []string
	var totalConfidence float64
	agents := make(map[string]bool)

	for _, s := range sources {
		if s.RawSnippet != "" {
			contents = append(contents, s.RawSnippet)
		}
		totalConfidence += s.ConfidenceContribution
		agents[s.AgentName] = true
	}

	if len(contents) == 0 {
		return nil, nil
	}

	// Use the longest content as the base (most informative).
	longest := contents[0]
	for _, c := range contents {
		if len(c) > len(longest) {
			longest = c
		}
	}

	confidence := totalConfidence / float64(len(sources))
	if confidence > 1.0 {
		confidence = 1.0
	}

	category := categorize(longest)

	return &CandidateRule{
		Content:        cleanContent(longest),
		Category:       category,
		TriggerContext: fmt.Sprintf("auto-distilled from %d signals across %d agents", len(sources), len(agents)),
		Confidence:     confidence,
		Tags:           []string{category},
	}, nil
}

// CheckSimilarity checks if a new rule is similar to any existing rule.
func (d *RuleBasedDistiller) CheckSimilarity(newContent string, existingRules []*storage.Rule) (*storage.Rule, error) {
	newLower := strings.ToLower(strings.TrimSpace(newContent))

	for _, rule := range existingRules {
		existingLower := strings.ToLower(strings.TrimSpace(rule.Content))

		// Simple similarity: check if one contains the other (keyword overlap).
		if newLower == existingLower {
			return rule, nil
		}

		// Check significant keyword overlap.
		newWords := wordSet(newLower)
		existingWords := wordSet(existingLower)
		overlap := 0
		for w := range newWords {
			if existingWords[w] {
				overlap++
			}
		}

		totalWords := len(newWords) + len(existingWords) - overlap
		if totalWords > 0 && float64(overlap)/float64(totalWords) > 0.7 {
			return rule, nil
		}
	}

	return nil, nil
}

// DetectConflict checks if a new rule conflicts with existing active rules.
func (d *RuleBasedDistiller) DetectConflict(newContent string, existingRules []*storage.Rule) bool {
	newLower := strings.ToLower(newContent)

	// Simple conflict detection: opposite directives.
	// E.g., "use tabs" vs "use spaces", "use npm" vs "use pnpm"
	for _, rule := range existingRules {
		existingLower := strings.ToLower(rule.Content)
		if isConflicting(newLower, existingLower) {
			return true
		}
	}

	return false
}

func isConflicting(a, b string) bool {
	conflictPairs := [][2]string{
		{"use tabs", "use spaces"},
		{"use spaces", "use tabs"},
		{"use npm", "use pnpm"},
		{"use pnpm", "use npm"},
		{"use npm", "use yarn"},
		{"use yarn", "use npm"},
		{"use pnpm", "use yarn"},
		{"use yarn", "use pnpm"},
	}

	for _, pair := range conflictPairs {
		if (strings.Contains(a, pair[0]) && strings.Contains(b, pair[1])) ||
			(strings.Contains(b, pair[0]) && strings.Contains(a, pair[1])) {
			return true
		}
	}
	return false
}

func categorize(content string) string {
	lower := strings.ToLower(content)

	toolingKeywords := []string{"npm", "pnpm", "yarn", "bun", "webpack", "vite", "eslint", "prettier", "babel"}
	for _, kw := range toolingKeywords {
		if strings.Contains(lower, kw) {
			return "toolchain"
		}
	}

	styleKeywords := []string{"indent", "tab", "space", "format", "style", "naming", "camel", "snake", "pascal"}
	for _, kw := range styleKeywords {
		if strings.Contains(lower, kw) {
			return "code-style"
		}
	}

	testKeywords := []string{"test", "spec", "coverage", "unit test", "integration"}
	for _, kw := range testKeywords {
		if strings.Contains(lower, kw) {
			return "testing"
		}
	}

	archKeywords := []string{"architecture", "pattern", "module", "component", "service", "layer", "folder", "directory"}
	for _, kw := range archKeywords {
		if strings.Contains(lower, kw) {
			return "architecture"
		}
	}

	return "general"
}

func cleanContent(s string) string {
	// Remove common prefixes that aren't part of the rule itself.
	s = strings.TrimSpace(s)
	prefixes := []string{"不对，", "别这么写，", "记住：", "以后都要", "Don't ", "Stop "}
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			s = strings.TrimPrefix(s, p)
			break
		}
	}
	return strings.TrimSpace(s)
}

func wordSet(s string) map[string]bool {
	words := strings.Fields(s)
	set := make(map[string]bool, len(words))
	for _, w := range words {
		if len(w) > 2 { // Skip short words.
			set[w] = true
		}
	}
	return set
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
