package distill

import (
	"strings"
	"testing"

	"github.com/joevilcai666/shadow/internal/storage"
)

func TestLLMDistillerFallbackNoAPIKey(t *testing.T) {
	d := NewLLMDistiller("", "claude-sonnet-4-20250514")

	// Should fallback to rule-based when no API key.
	if d.apiKey != "" {
		t.Error("API key should be empty")
	}
}

func TestLLMDistillerDefaultModel(t *testing.T) {
	d := NewLLMDistiller("test-key", "")
	if d.model != "claude-sonnet-4-20250514" {
		t.Errorf("expected default model, got %s", d.model)
	}
}

func TestLLMDistillerDistillFallback(t *testing.T) {
	d := NewLLMDistiller("", "claude-sonnet-4-20250514")

	sources := testSources()
	result, err := d.Distill(sources)
	if err != nil {
		t.Fatalf("distill: %v", err)
	}
	if result == nil {
		t.Fatal("should return result via fallback")
	}
	if result.Content == "" {
		t.Error("content should not be empty")
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{`{"key": "value"}`, `{"key": "value"}`},
		{"```json\n{\"key\": \"value\"}\n```", `{"key": "value"}`},
		{"```\n{\"key\": \"value\"}\n```", `{"key": "value"}`},
		{`  {"key": "value"}  `, `{"key": "value"}`},
	}

	for _, tt := range tests {
		got := extractJSON(tt.input)
		if got != tt.expect {
			t.Errorf("extractJSON(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestParseDistillResponse(t *testing.T) {
	resp := `{"content": "Use pnpm instead of npm", "category": "toolchain", "trigger_context": "when setting up projects", "confidence": 0.9, "tags": ["tooling"]}`

	result, err := parseDistillResponse(resp, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Content != "Use pnpm instead of npm" {
		t.Errorf("content: %q", result.Content)
	}
	if result.Category != "toolchain" {
		t.Errorf("category: %q", result.Category)
	}
	if result.Confidence != 0.9 {
		t.Errorf("confidence: %f", result.Confidence)
	}
}

func TestParseSimilarityResponseNone(t *testing.T) {
	resp := `{"match_id": "none"}`
	result, err := parseSimilarityResponse(resp, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result != nil {
		t.Error("should return nil for no match")
	}
}

func TestParseConflictResponse(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		{`{"conflicts": true}`, true},
		{`{"conflicts": false}`, false},
		{`invalid json`, false},
	}

	for _, tt := range tests {
		got := parseConflictResponse(tt.input)
		if got != tt.expect {
			t.Errorf("parseConflictResponse(%q) = %v, want %v", tt.input, got, tt.expect)
		}
	}
}

func TestBuildDistillPrompt(t *testing.T) {
	sources := testSources()
	prompt := buildDistillPrompt(sources)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
	// Should contain signal content.
	if !contains(prompt, "test snippet") {
		t.Error("prompt should contain signal content")
	}
}

func testSources() []*storage.Source {
	return []*storage.Source{
		{
			ID:                    "src-1",
			SignalType:            "explicit_instruction",
			SignalStrength:        "strong",
			AgentName:             "claude-code",
			RawSnippet:            "test snippet about pnpm",
			ConfidenceContribution: 0.85,
		},
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
}
