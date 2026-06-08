package capture

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// CursorParser reads Cursor AI chat logs.
//
// Cursor's chat history is stored in two ways depending on version and config:
//   - Plain JSONL files under ~/Library/Application Support/Cursor/ai/
//     or in workspaceStorage directories.
//   - A SQLite blob in state.vscdb (ItemTable, key=composer.composerData).
//
// This parser handles the plain-JSONL path. The SQLite state.vscdb path is
// NOT supported yet (would require either modernc.org/sqlite schema knowledge
// or shelling out to sqlite3). On machines where Cursor only writes the
// SQLite form, this parser will return zero signals — which is the correct
// behavior, not a bug.
//
// Format (JSONL, one entry per message):
//
//	{"type":"user","content":"...","timestamp":"..."}        newer
//	{"role":"user","text":"...","ts":<unix-seconds>}         older
type CursorParser struct {
	homeDir string
}

// NewCursorParser creates a parser for Cursor AI chat logs.
func NewCursorParser() *CursorParser {
	home, _ := os.UserHomeDir()
	return &CursorParser{homeDir: home}
}

// Name returns the parser identifier used in Source.AgentName.
func (p *CursorParser) Name() string { return "cursor" }

// cursorLogCandidates returns the directories where Cursor might keep
// plain-text chat logs. We probe rather than scan the SQLite state.vscdb
// (see file-level comment for why).
func (p *CursorParser) cursorLogCandidates() []string {
	return []string{
		// Newer: Cursor's dedicated AI logs directory.
		filepath.Join(p.homeDir, "Library", "Application Support", "Cursor", "ai"),
		// Per-workspace storage (some versions dump chat JSONL here).
		filepath.Join(p.homeDir, "Library", "Application Support", "Cursor", "User", "workspaceStorage"),
		// Cursor's app logs directory (rare, but cheap to probe).
		filepath.Join(p.homeDir, "Library", "Logs", "Cursor"),
	}
}

// DiscoverLogPaths finds Cursor AI chat JSONL files in known locations.
func (p *CursorParser) DiscoverLogPaths() ([]string, error) {
	var paths []string

	for _, dir := range p.cursorLogCandidates() {
		// Each candidate is a directory; walk it for .jsonl / .json files.
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip directories we can't read.
			}
			if !info.IsDir() && (strings.HasSuffix(path, ".jsonl") || strings.HasSuffix(path, ".json")) {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			// Directory may not exist; that's fine.
			continue
		}
	}

	return paths, nil
}

// Parse reads a Cursor log file from the given offset.
// Tries to handle both newer ({type,content,timestamp}) and older
// ({role,text,ts}) entry shapes.
func (p *CursorParser) Parse(filePath string, offset int64) ([]Signal, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()

	if _, err := f.Seek(offset, 0); err != nil {
		return nil, offset, err
	}

	var signals []Signal
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024) // 1MB max line.

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // Skip malformed lines.
		}

		if !p.isUserMessage(entry) {
			continue
		}

		content := extractUserText(entry)
		if content == "" {
			continue
		}

		sigType, strength, confidence := ClassifySignal(content)
		signals = append(signals, Signal{
			Type:           sigType,
			Strength:       strength,
			Content:        content,
			AgentName:      "cursor",
			ProjectPath:    extractCursorProjectPath(filePath),
			RawSnippet:     truncate(content, 500),
			Confidence:     confidence,
			SourceFilePath: filePath,
			SourceLineNum:  lineNum,
		})
	}

	newOffset, _ := f.Seek(0, 2)
	return signals, newOffset, nil
}

// isUserMessage returns true if the JSONL entry represents a user message.
// We accept both newer (type=user) and older (role=user) shapes; both with
// assistant/non-message entries explicitly filtered out.
func (p *CursorParser) isUserMessage(entry map[string]any) bool {
	// Newer: {"type":"user",...}
	if t, ok := entry["type"].(string); ok && t == "user" {
		return true
	}
	// Older: {"role":"user",...}
	if r, ok := entry["role"].(string); ok && r == "user" {
		return true
	}
	return false
}

// extractUserText pulls the user text from a Cursor entry, trying both
// the "content" field (newer) and the "text" field (older).
func extractUserText(entry map[string]any) string {
	if s, ok := entry["content"].(string); ok && s != "" {
		return s
	}
	if s, ok := entry["text"].(string); ok && s != "" {
		return s
	}
	return ""
}

// extractCursorProjectPath best-effort: derive the project path from
// the log file location. Cursor workspace storage uses opaque hash
// directories, so we can't recover the original project path from
// the log file path alone. This returns an empty string unless the
// log was placed under a known project directory.
func extractCursorProjectPath(_ string) string {
	// TODO: When Cursor's chat JSONL format includes a project/cwd field
	// (some workspaceStorage layouts do), parse it here. Until then we
	// leave ProjectPath empty for Cursor signals.
	return ""
}
