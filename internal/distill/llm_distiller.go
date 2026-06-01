package distill

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/joevilcai666/shadow/internal/storage"
)

// LLMDistiller uses an LLM (Anthropic Claude API) to distill signals into rules.
type LLMDistiller struct {
	apiKey string
	model  string
	client *http.Client
}

// NewLLMDistiller creates an LLM-powered distiller.
// If apiKey is empty, callers should fall back to RuleBasedDistiller.
func NewLLMDistiller(apiKey, model string) *LLMDistiller {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &LLMDistiller{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

// Distill calls the LLM to extract a candidate rule from signals.
func (d *LLMDistiller) Distill(sources []*storage.Source) (*CandidateRule, error) {
	if len(sources) == 0 {
		return nil, nil
	}
	if d.apiKey == "" {
		// Fallback to rule-based if no key configured.
		return NewRuleBasedDistiller().Distill(sources)
	}

	prompt := buildDistillPrompt(sources)
	resp, err := d.callLLM(prompt)
	if err != nil {
		slog.Warn("LLM distill failed, falling back to rule-based", "error", err)
		return NewRuleBasedDistiller().Distill(sources)
	}

	return parseDistillResponse(resp, sources)
}

// CheckSimilarity uses the LLM to check if a new rule is similar to an existing one.
func (d *LLMDistiller) CheckSimilarity(newContent string, existingRules []*storage.Rule) (*storage.Rule, error) {
	if d.apiKey == "" || len(existingRules) == 0 {
		return NewRuleBasedDistiller().CheckSimilarity(newContent, existingRules)
	}

	prompt := buildSimilarityPrompt(newContent, existingRules)
	resp, err := d.callLLM(prompt)
	if err != nil {
		slog.Warn("LLM similarity check failed, falling back", "error", err)
		return NewRuleBasedDistiller().CheckSimilarity(newContent, existingRules)
	}

	return parseSimilarityResponse(resp, existingRules)
}

// DetectConflict uses the LLM to detect conflicting rules.
func (d *LLMDistiller) DetectConflict(newContent string, existingRules []*storage.Rule) bool {
	if d.apiKey == "" || len(existingRules) == 0 {
		return NewRuleBasedDistiller().DetectConflict(newContent, existingRules)
	}

	prompt := buildConflictPrompt(newContent, existingRules)
	resp, err := d.callLLM(prompt)
	if err != nil {
		slog.Warn("LLM conflict check failed, falling back", "error", err)
		return NewRuleBasedDistiller().DetectConflict(newContent, existingRules)
	}

	return parseConflictResponse(resp)
}

// --- Anthropic Messages API ---

type messageRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Messages  []messageItem `json:"messages"`
}

type messageItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messageResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

func (d *LLMDistiller) callLLM(prompt string) (string, error) {
	reqBody := messageRequest{
		Model:     d.model,
		MaxTokens: 1024,
		Messages: []messageItem{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", d.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var msgResp messageResponse
	if err := json.Unmarshal(respBody, &msgResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(msgResp.Content) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	return msgResp.Content[0].Text, nil
}

// --- Prompt builders ---

func buildDistillPrompt(sources []*storage.Source) string {
	var signals []string
	for _, s := range sources {
		signals = append(signals, fmt.Sprintf("- [%s/%s] %s: %s",
			s.AgentName, s.SignalStrength, s.SignalType, s.RawSnippet))
	}

	return fmt.Sprintf(`You are a rule distillation engine for a coding agent memory layer called Shadow.

Your task: analyze the following correction signals from a user interacting with coding agents, and distill them into a single, clear, actionable rule.

Signals:
%s

Respond in JSON only:
{
  "content": "The distilled rule as a clear, imperative statement",
  "category": "One of: toolchain, code-style, testing, architecture, general",
  "trigger_context": "When this rule should be applied",
  "confidence": 0.0 to 1.0,
  "tags": ["relevant", "tags"]
}`, strings.Join(signals, "\n"))
}

func buildSimilarityPrompt(newContent string, existingRules []*storage.Rule) string {
	var rules []string
	for _, r := range existingRules {
		rules = append(rules, fmt.Sprintf("- [id:%s] %s", r.ID, r.Content))
	}

	return fmt.Sprintf(`You are checking if a new rule is semantically similar to any existing rule.

New rule: %s

Existing rules:
%s

If the new rule is semantically similar to an existing one (same intent, possibly different wording), respond with the ID of the matching rule.
If no match, respond with "none".

Respond in JSON only:
{
  "match_id": "the-rule-id-or-none"
}`, newContent, strings.Join(rules, "\n"))
}

func buildConflictPrompt(newContent string, existingRules []*storage.Rule) string {
	var rules []string
	for _, r := range existingRules {
		rules = append(rules, fmt.Sprintf("- %s", r.Content))
	}

	return fmt.Sprintf(`You are checking if a new rule conflicts with existing rules.

New rule: %s

Existing rules:
%s

Does the new rule directly contradict any existing rule? For example, "use tabs" vs "use spaces".
Respond in JSON only:
{
  "conflicts": true or false
}`, newContent, strings.Join(rules, "\n"))
}

// --- Response parsers ---

func parseDistillResponse(resp string, sources []*storage.Source) (*CandidateRule, error) {
	// Extract JSON from possible markdown code blocks.
	resp = extractJSON(resp)

	var result struct {
		Content        string   `json:"content"`
		Category       string   `json:"category"`
		TriggerContext string   `json:"trigger_context"`
		Confidence     float64  `json:"confidence"`
		Tags           []string `json:"tags"`
	}

	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}

	if result.Content == "" {
		return nil, nil
	}

	if result.Tags == nil {
		result.Tags = []string{}
	}
	if result.Category == "" {
		result.Category = "general"
	}
	if result.Confidence == 0 {
		// Estimate from source confidence.
		var total float64
		for _, s := range sources {
			total += s.ConfidenceContribution
		}
		result.Confidence = total / float64(len(sources))
		if result.Confidence > 1.0 {
			result.Confidence = 1.0
		}
	}

	return &CandidateRule{
		Content:        result.Content,
		Category:       result.Category,
		TriggerContext: result.TriggerContext,
		Confidence:     result.Confidence,
		Tags:           result.Tags,
	}, nil
}

func parseSimilarityResponse(resp string, existingRules []*storage.Rule) (*storage.Rule, error) {
	resp = extractJSON(resp)

	var result struct {
		MatchID string `json:"match_id"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, nil
	}

	if result.MatchID == "" || result.MatchID == "none" {
		return nil, nil
	}

	for _, r := range existingRules {
		if r.ID == result.MatchID {
			return r, nil
		}
	}

	return nil, nil
}

func parseConflictResponse(resp string) bool {
	resp = extractJSON(resp)

	var result struct {
		Conflicts bool `json:"conflicts"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return false
	}
	return result.Conflicts
}

// extractJSON strips markdown code fences if present.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}
