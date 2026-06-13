package distill

import (
	"fmt"
	"strings"

	"github.com/joevilcai666/shadow/internal/storage"
)

// ContextEngine extracts relevant rules for a given task.
type ContextEngine struct {
	ruleRepo *storage.RuleRepo
}

// NewContextEngine creates a new ContextEngine.
func NewContextEngine(ruleRepo *storage.RuleRepo) *ContextEngine {
	return &ContextEngine{ruleRepo: ruleRepo}
}

// TaskContextRequest describes the context needed to extract rules for a task.
type TaskContextRequest struct {
	TaskDescription string   // free-text task description
	ProjectPath    string   // current project path
	AgentName      string   // target agent (codex, claude-code, cursor, copilot)
	Tags           []string // explicitly requested tags (from structured params)
	MaxRules       int      // max rules to inject (default 5)
}

// ExtractedContext is the result of context extraction.
type ExtractedContext struct {
	Rules          []*storage.Rule `json:"rules"`           // selected rules
	TotalFound     int             `json:"total_found"`     // total matching before limit
	ProjectMatched bool            `json:"project_matched"`
	TagMatches     []string        `json:"tag_matches"`     // tags that had exact hits
}

// Extract extracts relevant rules for the given task request.
// Rules are selected by: exact tag match → scope hit (global+project) → project match,
// then sorted by decay_score (recency-adjusted confidence), capped at MaxRules.
func (e *ContextEngine) Extract(req TaskContextRequest) (*ExtractedContext, error) {
	if req.MaxRules <= 0 {
		req.MaxRules = 5
	}

	// Get all active rules.
	allRules, err := e.ruleRepo.List(storage.RuleFilter{Status: "active", Limit: 500})
	if err != nil {
		return nil, err
	}

	// Score and filter rules.
	type scoredRule struct {
		rule       *storage.Rule
		score      float64
		tagHit     bool
		projectHit bool
	}

	var scored []scoredRule
	for _, r := range allRules {
		s, tagHit, projectHit := e.scoreRule(r, req)
		if s > 0 {
			scored = append(scored, scoredRule{rule: r, score: s, tagHit: tagHit, projectHit: projectHit})
		}
	}

	// Sort by score descending (simple bubble sort, small list).
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Apply limit.
	result := &ExtractedContext{
		TotalFound: len(scored),
	}
	for i := 0; i < len(scored) && i < req.MaxRules; i++ {
		result.Rules = append(result.Rules, scored[i].rule)
		if scored[i].tagHit {
			result.TagMatches = append(result.TagMatches, scored[i].rule.Tags...)
		}
		if scored[i].projectHit {
			result.ProjectMatched = true
		}
	}

	return result, nil
}

// scoreRule returns a relevance score (0 = not relevant) plus the dimensions
// that hit, so callers can surface match metadata. Higher score = more relevant.
// Based on:
//   - Tag exact match: +0.5
//   - Same project: +0.3
//   - Recency (30-day half-life): decay_score factor
//   - Confidence: base confidence
func (e *ContextEngine) scoreRule(r *storage.Rule, req TaskContextRequest) (score float64, tagHit bool, projectHit bool) {
	// Tag exact match check.
	if len(req.Tags) > 0 {
		for _, wantTag := range req.Tags {
			for _, ruleTag := range r.Tags {
				if strings.EqualFold(wantTag, ruleTag) {
					tagHit = true
					break
				}
			}
			if tagHit {
				break
			}
		}
		if tagHit {
			score += 0.5
		}
	}

	// Scope: global always matches; project-scoped must match project path.
	if r.Scope == "global" {
		score += 0.2
	} else if r.ProjectPath != "" && req.ProjectPath != "" {
		if strings.HasPrefix(req.ProjectPath, r.ProjectPath) ||
			strings.HasPrefix(r.ProjectPath, req.ProjectPath) ||
			strings.EqualFold(r.ProjectPath, req.ProjectPath) {
			score += 0.3
			projectHit = true
		} else {
			return 0, false, false // project rule but different project
		}
	}

	// If no tag hit and scope doesn't match, skip.
	if score == 0 && r.Scope != "global" {
		return 0, false, false
	}

	// Recency factor: use decay_score (already computed) or fall back to confidence.
	recencyFactor := r.DecayScore
	if recencyFactor == 0 {
		recencyFactor = r.Confidence
	}

	// Importance weight (0.5 to 2.0).
	importance := r.Importance
	if importance == 0 {
		importance = 0.5
	}

	// Final score: confidence * importance * recency_factor.
	// Add small tag bonus on top.
	finalScore := recencyFactor * importance * 2.0
	if tagHit {
		finalScore += 0.1
	}

	return finalScore, tagHit, projectHit
}

// ExtractTagsFromText extracts potential tags from free-text task description.
// Simple keyword-based extraction (no embedding).
func ExtractTagsFromText(text string) []string {
	// Common project/topic keywords to look for.
	keywords := []string{
		"deploy", "deployment", "staging", "production",
		"auth", "authentication", "jwt", "oauth", "login",
		"api", "rest", "graphql", "endpoint",
		"database", "migration", "schema", "sql",
		"test", "testing", "unit", "integration",
		"security", "vulnerability", "xss", "sql-injection",
		"performance", "optimization", "cache",
		"refactor", "cleanup", "debt",
		"bug", "fix", "hotfix",
		"feature", "feature-flags",
		"docker", "kubernetes", "ci", "cd",
		"config", "configuration", "env",
		"docs", "documentation",
	}

	text = strings.ToLower(text)
	var tags []string
	seen := make(map[string]bool)

	for _, kw := range keywords {
		if strings.Contains(text, kw) && !seen[kw] {
			tags = append(tags, kw)
			seen[kw] = true
		}
	}

	return tags
}

// ExtractForAgent returns the context formatted for a specific agent.
// Returns the rules as a markdown-formatted string suitable for injecting
// into the agent's context file.
func (e *ContextEngine) ExtractForAgent(req TaskContextRequest) (string, error) {
	ctx, err := e.Extract(req)
	if err != nil {
		return "", err
	}

	if len(ctx.Rules) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("## Shadow Memory — Relevant Rules\n\n")

	for i, r := range ctx.Rules {
		sb.WriteString(fmt.Sprintf("### [%d] %s\n", i+1, r.Category))
		sb.WriteString(r.Content + "\n")
		if len(r.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(r.Tags, ", ")))
		}
		if r.Scope == "project" {
			sb.WriteString(fmt.Sprintf("Scope: project (%s)\n", r.ProjectPath))
		}
		if r.DecayScore > 0 {
			sb.WriteString(fmt.Sprintf("Confidence: %.0f%% (last hit: %s)\n",
				r.DecayScore*100, r.LastHitAt))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}
