package capture

import (
	"os"
	"testing"
)

// TestCodexParser_RealHistory is a smoke test against the user's actual
// ~/.codex/history.jsonl. Skipped if the file doesn't exist.
func TestCodexParser_RealHistory(t *testing.T) {
	home, _ := os.UserHomeDir()
	histPath := home + "/.codex/history.jsonl"
	if _, err := os.Stat(histPath); err != nil {
		t.Skipf("real history.jsonl not present at %s: %v", histPath, err)
	}

	parser := NewCodexParser()
	signals, _, err := parser.Parse(histPath, 0)
	if err != nil {
		t.Fatalf("parse real history: %v", err)
	}
	t.Logf("real ~/.codex/history.jsonl: %d signals", len(signals))
	strongCount := 0
	for _, s := range signals {
		if s.Strength == "strong" {
			strongCount++
		}
	}
	t.Logf("strong signals: %d", strongCount)
}
