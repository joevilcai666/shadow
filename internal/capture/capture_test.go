package capture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifySignal(t *testing.T) {
	tests := []struct {
		input           string
		wantType        string
		wantStrength    string
		minConfidence   float64
	}{
		{"不对，应该用 pnpm", "explicit_instruction", "strong", 0.8},
		{"Don't use npm, use pnpm instead", "explicit_instruction", "strong", 0.8},
		{"记住这条规则", "manual_mark", "strong", 0.9},
		{"Remember to always use tabs", "manual_mark", "strong", 0.9},
		{"请帮我写一个函数", "explicit_instruction", "medium", 0.5},
		{"This looks good", "explicit_instruction", "medium", 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			sigType, strength, confidence := ClassifySignal(tt.input)
			if sigType != tt.wantType {
				t.Errorf("type: got %q, want %q", sigType, tt.wantType)
			}
			if strength != tt.wantStrength {
				t.Errorf("strength: got %q, want %q", strength, tt.wantStrength)
			}
			if confidence < tt.minConfidence {
				t.Errorf("confidence: got %.2f, want >= %.2f", confidence, tt.minConfidence)
			}
		})
	}
}

func TestClaudeCodeParserParse(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "session.jsonl")

	// Write a test log with mixed content.
	content := `{"role":"assistant","content":"Here's the code using npm install"}
{"role":"human","content":"不对，应该用 pnpm，不要用 npm"}
{"role":"assistant","content":"Updated to use pnpm"}
{"role":"human","content":"请帮我写测试"}
{"role":"human","content":"记住这条：永远不要用 npm"}
`
	os.WriteFile(logFile, []byte(content), 0644)

	parser := NewClaudeCodeParser()
	signals, newOffset, err := parser.Parse(logFile, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(signals) < 2 {
		t.Fatalf("expected at least 2 signals, got %d", len(signals))
	}

	// First signal should be negation (contains "不对").
	found := false
	for _, s := range signals {
		if s.Type == "explicit_instruction" && s.AgentName == "claude-code" && s.Strength == "strong" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find strong negation signal")
	}

	// Should find "记住" mark signal.
	foundMark := false
	for _, s := range signals {
		if s.Type == "manual_mark" {
			foundMark = true
		}
	}
	if !foundMark {
		t.Error("expected to find manual_mark signal")
	}

	if newOffset == 0 {
		t.Error("offset should advance after parsing")
	}

	// Incremental parse: write more data and parse from offset.
	appendContent := `{"role":"human","content":"别这么写，用 Go 的惯用写法"}
`
	f, _ := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString(appendContent)
	f.Close()

	newSignals, _, err := parser.Parse(logFile, newOffset)
	if err != nil {
		t.Fatalf("incremental parse: %v", err)
	}
	if len(newSignals) < 1 {
		t.Error("expected at least 1 new signal from incremental parse")
	}
}

func TestClaudeCodeDiscoverLogPaths(t *testing.T) {
	parser := &ClaudeCodeParser{homeDir: t.TempDir()}

	// Create fake log structure.
	projectsDir := filepath.Join(parser.homeDir, ".claude", "projects", "-tmp-testproject")
	os.MkdirAll(projectsDir, 0755)
	os.WriteFile(filepath.Join(projectsDir, "session.jsonl"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(projectsDir, "other.txt"), []byte("not a log"), 0644)

	paths, err := parser.DiscoverLogPaths()
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(paths), paths)
	}
}

func TestExtractProjectPath(t *testing.T) {
	tests := []struct {
		logPath string
		want    string
	}{
		{"/Users/dev/.claude/projects/-Users-dev-myproject/session.jsonl", "Users/dev/myproject"},
		{"/Users/dev/.claude/projects/myproject/session.jsonl", "myproject"},
		{"/tmp/random/file.jsonl", ""},
	}

	for _, tt := range tests {
		got := extractProjectPath(tt.logPath)
		if got != tt.want {
			t.Errorf("extractProjectPath(%q) = %q, want %q", tt.logPath, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("should not truncate short strings")
	}
	long := "this is a very long string that should be truncated"
	truncated := truncate(long, 20)
	if len(truncated) > 23 { // 20 + "..."
		t.Errorf("truncated too long: %q", truncated)
	}
	if truncated[len(truncated)-3:] != "..." {
		t.Error("truncated should end with ...")
	}
}
