package capture

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// CodexParser reads Codex CLI logs. Codex stores user messages in two formats:
//
//  1. history.jsonl: a flat list across all sessions, one line per user message.
//     {"session_id":"<uuid>","ts":<unix-seconds>,"text":"<message>"}
//
//  2. rollout-*.jsonl: per-session detailed log, including a session_meta header
//     with the working directory and structured message events.
//     {"timestamp":"...","type":"session_meta","payload":{"cwd":"/path",...}}
//     {"timestamp":"...","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"..."}]}}
//
// Project context (cwd) is only available in rollout files via session_meta.
// history.jsonl entries intentionally leave ProjectPath empty.
type CodexParser struct {
	homeDir string
}

// NewCodexParser creates a parser for Codex CLI logs.
func NewCodexParser() *CodexParser {
	home, _ := os.UserHomeDir()
	return &CodexParser{homeDir: home}
}

// Name returns the parser identifier used in Source.AgentName.
func (p *CodexParser) Name() string { return "codex" }

// DiscoverLogPaths finds Codex history.jsonl and any rollout-*.jsonl files.
func (p *CodexParser) DiscoverLogPaths() ([]string, error) {
	var paths []string

	// Primary: history.jsonl (always present if Codex has been used).
	histPath := filepath.Join(p.homeDir, ".codex", "history.jsonl")
	if _, err := os.Stat(histPath); err == nil {
		paths = append(paths, histPath)
	}

	// Secondary: per-session rollouts under ~/.codex/sessions/.
	sessionsDir := filepath.Join(p.homeDir, ".codex", "sessions")
	_ = filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip directories we can't read.
		}
		if !info.IsDir() && strings.HasSuffix(path, ".jsonl") {
			paths = append(paths, path)
		}
		return nil
	})

	return paths, nil
}

// Parse reads a Codex log file (history.jsonl OR rollout-*.jsonl) from the given offset.
// Returns signals, the new offset (file end), and any error encountered.
func (p *CodexParser) Parse(filePath string, offset int64) ([]Signal, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()

	if _, err := f.Seek(offset, 0); err != nil {
		return nil, offset, err
	}

	var signals []Signal
	// cwd is captured from session_meta (rollout files only) and used to
	// attribute all subsequent signals in the same file/parse to that project.
	var cwd string

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024) // 1MB max line.

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // Skip malformed lines; don't abort the whole parse.
		}

		// Track cwd from session_meta (rollout files).
		if t, _ := entry["type"].(string); t == "session_meta" {
			if payload, ok := entry["payload"].(map[string]any); ok {
				if c, ok := payload["cwd"].(string); ok {
					cwd = c
				}
			}
			continue
		}

		sig := p.entryToSignal(entry, cwd, filePath, lineNum)
		if sig != nil {
			signals = append(signals, *sig)
		}
	}

	newOffset, _ := f.Seek(0, 2) // End of file.
	return signals, newOffset, nil
}

// entryToSignal converts a single JSONL entry to a Signal, or nil if the
// entry doesn't represent a user correction (skips assistant messages, etc.).
func (p *CodexParser) entryToSignal(entry map[string]any, cwd, filePath string, lineNum int) *Signal {
	// Format 1: history.jsonl — flat user message.
	// {"session_id":"...","ts":...,"text":"..."}
	if text, ok := entry["text"].(string); ok && text != "" {
		sigType, strength, confidence := ClassifySignal(text)
		return &Signal{
			Type:           sigType,
			Strength:       strength,
			Content:        text,
			AgentName:      "codex",
			ProjectPath:    cwd, // empty for history.jsonl (no session_meta)
			RawSnippet:     truncate(text, 500),
			Confidence:     confidence,
			SourceFilePath: filePath,
			SourceLineNum:  lineNum,
		}
	}

	// Format 2: rollout-*.jsonl — structured response_item.
	// {"type":"response_item","payload":{"type":"message","role":"user","content":[...]}}
	entryType, _ := entry["type"].(string)
	if entryType != "response_item" {
		return nil
	}
	payload, ok := entry["payload"].(map[string]any)
	if !ok {
		return nil
	}
	if msgType, _ := payload["type"].(string); msgType != "message" {
		return nil
	}
	role, _ := payload["role"].(string)
	if role != "user" {
		return nil
	}
	content := extractContentText(payload["content"])
	if content == "" {
		return nil
	}

	sigType, strength, confidence := ClassifySignal(content)
	return &Signal{
		Type:           sigType,
		Strength:       strength,
		Content:        content,
		AgentName:      "codex",
		ProjectPath:    cwd,
		RawSnippet:     truncate(content, 500),
		Confidence:     confidence,
		SourceFilePath: filePath,
		SourceLineNum:  lineNum,
	}
}

// extractContentText pulls the text payload from a Codex content array
// (which can be [{type:"input_text", text:"..."}, ...] or a plain string).
func extractContentText(content any) string {
	// Plain string.
	if s, ok := content.(string); ok {
		return s
	}
	// Array of typed content parts.
	items, ok := content.([]any)
	if !ok {
		return ""
	}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["type"].(string)
		if t == "input_text" || t == "text" || t == "output_text" {
			if text, ok := m["text"].(string); ok {
				return text
			}
		}
	}
	return ""
}
