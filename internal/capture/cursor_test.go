package capture

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestCursorParser_Parse verifies Cursor AI chat log parsing.
// Cursor's plain-text AI logs (when present as JSONL) are typically one
// entry per user message in a format like:
//
//	{"type":"user","content":"...","timestamp":"..."}
//
// OR (older composerData exports):
//
//	{"role":"user","text":"...","ts":<unix>}
func TestCursorParser_Parse(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "chat.jsonl")

	content := `{"type":"user","content":"不要用 console.log，用 structured logger","timestamp":"2026-05-01T10:00:00Z"}
{"type":"assistant","content":"Switching to structured logger."}
{"type":"user","content":"记得以后都用 pnpm","timestamp":"2026-05-01T10:01:00Z"}
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	parser := &CursorParser{homeDir: t.TempDir()} // home irrelevant for Parse.
	signals, newOffset, err := parser.Parse(logFile, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(signals) != 2 {
		t.Fatalf("expected 2 user signals (assistant skipped), got %d: %+v", len(signals), signals)
	}

	// First: Chinese negation.
	if signals[0].Type != "explicit_instruction" {
		t.Errorf("signal[0].Type = %q, want explicit_instruction", signals[0].Type)
	}
	if signals[0].AgentName != "cursor" {
		t.Errorf("signal[0].AgentName = %q, want cursor", signals[0].AgentName)
	}
	if signals[0].SourceLineNum != 1 {
		t.Errorf("signal[0].SourceLineNum = %d, want 1", signals[0].SourceLineNum)
	}

	// Second: "记得" → manual_mark.
	if signals[1].Type != "manual_mark" {
		t.Errorf("signal[1].Type = %q, want manual_mark", signals[1].Type)
	}

	if newOffset == 0 {
		t.Error("newOffset should advance")
	}
}

// TestCursorParser_Parse_AlternateFormat verifies the parser also handles
// the older {"role":..., "text":..., "ts":...} variant.
func TestCursorParser_Parse_AlternateFormat(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "old.jsonl")

	content := `{"role":"user","text":"Stop using var, use const","ts":1772165893}
{"role":"assistant","text":"Switched to const."}
{"role":"user","text":"Use tabs not spaces","ts":1772165894}
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	parser := &CursorParser{homeDir: t.TempDir()}
	signals, _, err := parser.Parse(logFile, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(signals) != 2 {
		t.Fatalf("expected 2 user signals, got %d: %+v", len(signals), signals)
	}
	for i, s := range signals {
		if s.AgentName != "cursor" {
			t.Errorf("signal[%d].AgentName = %q, want cursor", i, s.AgentName)
		}
	}
}

// TestCursorParser_Parse_SkipsAssistantAndMalformed ensures robustness.
func TestCursorParser_Parse_SkipsAssistantAndMalformed(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "mixed.jsonl")

	content := `not json
{"type":"user","content":"valid 1"}
{"type":"assistant","content":"assistant output"}
{also broken
{"type":"user","content":"valid 2"}
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	parser := &CursorParser{homeDir: t.TempDir()}
	signals, _, err := parser.Parse(logFile, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(signals) != 2 {
		t.Errorf("expected 2 valid signals, got %d", len(signals))
	}
}

// TestCursorParser_DiscoverLogPaths verifies the parser probes known
// Cursor AI chat locations and returns anything found as JSONL candidates.
func TestCursorParser_DiscoverLogPaths(t *testing.T) {
	home := t.TempDir()

	// Create platform-appropriate mock Cursor AI log directory.
	var aiDir string
	if runtime.GOOS == "windows" {
		// On Windows, Cursor stores logs under %APPDATA%.
		appData := filepath.Join(home, "AppData", "Roaming")
		aiDir = filepath.Join(appData, "Cursor", "User", "workspaceStorage")
	} else {
		aiDir = filepath.Join(home, "Library", "Application Support", "Cursor", "ai")
	}
	if err := os.MkdirAll(aiDir, 0755); err != nil {
		t.Fatal(err)
	}
	aiLog := filepath.Join(aiDir, "session-2026-05-01.jsonl")
	if err := os.WriteFile(aiLog, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Add a noise file that should be ignored.
	if err := os.WriteFile(filepath.Join(aiDir, "README.md"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}

	parser := &CursorParser{homeDir: home}
	paths, err := parser.DiscoverLogPaths()
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if paths[0] != aiLog {
		t.Errorf("discovered path = %q, want %q", paths[0], aiLog)
	}
}

// TestCursorParser_DiscoverLogPaths_NoCursor verifies graceful behavior
// when Cursor is not installed (no ~/Library/Application Support/Cursor).
func TestCursorParser_DiscoverLogPaths_NoCursor(t *testing.T) {
	parser := &CursorParser{homeDir: t.TempDir()}
	paths, err := parser.DiscoverLogPaths()
	if err != nil {
		t.Fatalf("discover should not error on missing dir: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths when Cursor not installed, got %d: %v", len(paths), paths)
	}
}

// TestCursorParser_Parse_MissingFile verifies the parser errors cleanly.
func TestCursorParser_Parse_MissingFile(t *testing.T) {
	parser := &CursorParser{homeDir: t.TempDir()}
	_, _, err := parser.Parse("/nonexistent/cursor/chat.jsonl", 0)
	if err == nil {
		t.Error("expected error on missing file")
	}
}
