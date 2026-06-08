package capture

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCodexParser_Parse_HistoryFile verifies Codex history.jsonl parsing.
// Real-world format (from ~/.codex/history.jsonl on macOS):
//
//	{"session_id":"<uuid>","ts":<unix-seconds>,"text":"<user message>"}
func TestCodexParser_Parse_HistoryFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "history.jsonl")

	content := `{"session_id":"abc-123","ts":1772165893,"text":"不对，应该用 pnpm，不要用 npm"}
{"session_id":"abc-123","ts":1772165894,"text":"Please add a comment"}
{"session_id":"def-456","ts":1772165895,"text":"记住这条：永远不要用 catch-all"}
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	parser := NewCodexParser()
	signals, newOffset, err := parser.Parse(logFile, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(signals) != 3 {
		t.Fatalf("expected 3 signals (one per line), got %d: %+v", len(signals), signals)
	}

	// First signal: Chinese negation → explicit_instruction / strong.
	if signals[0].Type != "explicit_instruction" {
		t.Errorf("signal[0].Type = %q, want explicit_instruction", signals[0].Type)
	}
	if signals[0].Strength != "strong" {
		t.Errorf("signal[0].Strength = %q, want strong", signals[0].Strength)
	}
	if signals[0].AgentName != "codex" {
		t.Errorf("signal[0].AgentName = %q, want codex", signals[0].AgentName)
	}
	if signals[0].SourceLineNum != 1 {
		t.Errorf("signal[0].SourceLineNum = %d, want 1", signals[0].SourceLineNum)
	}

	// Second signal: neutral → medium.
	if signals[1].Strength != "medium" {
		t.Errorf("signal[1].Strength = %q, want medium", signals[1].Strength)
	}

	// Third signal: "记住" → manual_mark / strong.
	if signals[2].Type != "manual_mark" {
		t.Errorf("signal[2].Type = %q, want manual_mark", signals[2].Type)
	}
	if signals[2].Strength != "strong" {
		t.Errorf("signal[2].Strength = %q, want strong", signals[2].Strength)
	}

	if newOffset == 0 {
		t.Error("newOffset should advance after parsing")
	}
}

// TestCodexParser_Parse_RolloutFile verifies parsing of rollout-*.jsonl files.
// Real-world format (from ~/.codex/sessions/YYYY/MM/DD/rollout-<uuid>.jsonl):
//
//	{"timestamp":"...","type":"session_meta","payload":{"cwd":"/path/to/project",...}}
//	{"timestamp":"...","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"..."}]}}
func TestCodexParser_Parse_RolloutFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "rollout-test.jsonl")

	content := `{"timestamp":"2026-03-03T15:42:43.091Z","type":"session_meta","payload":{"id":"sess-1","cwd":"/Users/dev/myproject","originator":"codex_cli_rs"}}
{"timestamp":"2026-03-03T15:43:00.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"不要用 npm，用 pnpm"}]}}
{"timestamp":"2026-03-03T15:44:00.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"text","text":"Switching to pnpm now."}]}}
{"timestamp":"2026-03-03T15:45:00.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"记得以后都用 pnpm"}]}}
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	parser := NewCodexParser()
	signals, _, err := parser.Parse(logFile, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Expect 2 user messages (assistant message is skipped).
	if len(signals) != 2 {
		t.Fatalf("expected 2 user-message signals, got %d: %+v", len(signals), signals)
	}

	// Project path should be inherited from session_meta.
	if signals[0].ProjectPath != "/Users/dev/myproject" {
		t.Errorf("signal[0].ProjectPath = %q, want /Users/dev/myproject", signals[0].ProjectPath)
	}
	if signals[1].ProjectPath != "/Users/dev/myproject" {
		t.Errorf("signal[1].ProjectPath = %q, want /Users/dev/myproject", signals[1].ProjectPath)
	}

	// First user signal: Chinese negation.
	if signals[0].Type != "explicit_instruction" || signals[0].Strength != "strong" {
		t.Errorf("signal[0] = %+v, want explicit_instruction/strong", signals[0])
	}

	// Second user signal: "记得" → manual_mark.
	if signals[1].Type != "manual_mark" {
		t.Errorf("signal[1].Type = %q, want manual_mark", signals[1].Type)
	}

	// AgentName must be "codex" for both.
	for i, s := range signals {
		if s.AgentName != "codex" {
			t.Errorf("signal[%d].AgentName = %q, want codex", i, s.AgentName)
		}
	}
}

// TestCodexParser_Parse_IncrementalOffset verifies the parser can resume from a saved offset.
func TestCodexParser_Parse_IncrementalOffset(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "history.jsonl")

	part1 := `{"session_id":"a","ts":1,"text":"first message"}
`
	part2 := `{"session_id":"a","ts":2,"text":"second message"}
`
	if err := os.WriteFile(logFile, []byte(part1), 0644); err != nil {
		t.Fatal(err)
	}

	parser := NewCodexParser()
	signals, offset, err := parser.Parse(logFile, 0)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("first parse: expected 1 signal, got %d", len(signals))
	}

	// Append more content.
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(part2); err != nil {
		t.Fatal(err)
	}
	f.Close()

	signals2, _, err := parser.Parse(logFile, offset)
	if err != nil {
		t.Fatalf("incremental parse: %v", err)
	}
	if len(signals2) != 1 {
		t.Errorf("incremental parse: expected 1 new signal, got %d", len(signals2))
	}
	if signals2[0].Content != "second message" {
		t.Errorf("incremental signal content = %q, want %q", signals2[0].Content, "second message")
	}
}

// TestCodexParser_Parse_SkipsMalformedLines ensures bad JSONL lines don't break the parser.
func TestCodexParser_Parse_SkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "history.jsonl")

	content := `not json at all
{"session_id":"a","ts":1,"text":"valid first"}
{also broken
{"session_id":"a","ts":2,"text":"valid second"}
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	parser := NewCodexParser()
	signals, _, err := parser.Parse(logFile, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(signals) != 2 {
		t.Errorf("expected 2 valid signals (skipped 2 malformed), got %d", len(signals))
	}
}

// TestCodexParser_DiscoverLogPaths verifies both history.jsonl and rollout-*.jsonl are found.
func TestCodexParser_DiscoverLogPaths(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	historyFile := filepath.Join(codexDir, "history.jsonl")
	rolloutDir := filepath.Join(codexDir, "sessions", "2026", "03", "03")
	rolloutFile := filepath.Join(rolloutDir, "rollout-2026-03-03T12-00-00-abc.jsonl")

	if err := os.MkdirAll(rolloutDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(historyFile, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rolloutFile, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Noise file: should not be picked up.
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	parser := &CodexParser{homeDir: home}
	paths, err := parser.DiscoverLogPaths()
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths (history + rollout), got %d: %v", len(paths), paths)
	}

	foundHistory, foundRollout := false, false
	for _, p := range paths {
		if p == historyFile {
			foundHistory = true
		}
		if p == rolloutFile {
			foundRollout = true
		}
	}
	if !foundHistory {
		t.Error("history.jsonl not discovered")
	}
	if !foundRollout {
		t.Error("rollout file not discovered")
	}
}

// TestCodexParser_DiscoverLogPaths_NoCodex verifies graceful behavior when ~/.codex is absent.
func TestCodexParser_DiscoverLogPaths_NoCodex(t *testing.T) {
	parser := &CodexParser{homeDir: t.TempDir()}
	paths, err := parser.DiscoverLogPaths()
	if err != nil {
		t.Fatalf("discover should not error on missing dir: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths when ~/.codex absent, got %d: %v", len(paths), paths)
	}
}

// TestCodexParser_Parse_MissingFile verifies the parser errors cleanly on missing files.
func TestCodexParser_Parse_MissingFile(t *testing.T) {
	parser := NewCodexParser()
	_, _, err := parser.Parse("/nonexistent/path/history.jsonl", 0)
	if err == nil {
		t.Error("expected error on missing file")
	}
}
