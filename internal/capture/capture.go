package capture

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/joevilcai666/shadow/internal/config"
	"github.com/joevilcai666/shadow/internal/storage"
)

// Engine captures correction signals from coding agent logs.
type Engine struct {
	mu        sync.Mutex
	config    *config.Manager
	sourceDB  *storage.SourceRepo
	ruleDB    *storage.RuleRepo
	watcher   *fsnotify.Watcher
	parsers   map[string]LogParser
	offsets   map[string]int64 // file -> last parsed offset
	offsetDir string

	ctx    context.Context
	cancel context.CancelFunc
}

// LogParser extracts signals from an agent's log format.
type LogParser interface {
	Name() string
	DiscoverLogPaths() ([]string, error)
	Parse(filePath string, offset int64) ([]Signal, int64, error)
}

// Signal represents a captured correction signal.
type Signal struct {
	Type            string  // "explicit_instruction", "manual_mark", "repetition", "manual_edit"
	Strength        string  // "strong", "medium", "weak"
	Content         string  // The correction text (sanitized)
	AgentName       string  // Which agent produced this
	ProjectPath     string  // Which project
	RawSnippet      string  // Sanitized original text
	Confidence      float64 // Estimated confidence contribution
	SourceFilePath  string  // Log file path
	SourceLineNum   int     // Line in the log file
}

// NewEngine creates a new capture engine.
func NewEngine(cfg *config.Manager, sourceDB *storage.SourceRepo, ruleDB *storage.RuleRepo, homeDir string) *Engine {
	return &Engine{
		config:    cfg,
		sourceDB:  sourceDB,
		ruleDB:    ruleDB,
		parsers:   make(map[string]LogParser),
		offsets:   make(map[string]int64),
		offsetDir: filepath.Join(homeDir, "offsets"),
	}
}

// RegisterParser adds a log parser for a specific agent.
func (e *Engine) RegisterParser(p LogParser) {
	e.parsers[p.Name()] = p
}

// Start begins watching for log file changes.
func (e *Engine) Start(ctx context.Context) error {
	e.ctx, e.cancel = context.WithCancel(ctx)

	if err := os.MkdirAll(e.offsetDir, 0755); err != nil {
		return fmt.Errorf("create offsets dir: %w", err)
	}

	// Load saved offsets.
	e.loadOffsets()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	e.watcher = watcher

	// Discover and watch log paths.
	for _, p := range e.parsers {
		paths, err := p.DiscoverLogPaths()
		if err != nil {
			slog.Warn("discover log paths", "parser", p.Name(), "error", err)
			continue
		}
		for _, path := range paths {
			// Watch the directory (more reliable than watching files directly).
			dir := filepath.Dir(path)
			if err := watcher.Add(dir); err != nil {
				slog.Warn("watch dir", "dir", dir, "error", err)
			}
		}
		slog.Info("registered parser", "parser", p.Name(), "paths", len(paths))
	}

	// Do initial scan of all discovered paths.
	e.initialScan()

	go e.watchLoop()
	return nil
}

// Stop stops the capture engine.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.watcher != nil {
		e.watcher.Close()
	}
	e.saveOffsets()
}

func (e *Engine) initialScan() {
	for _, p := range e.parsers {
		paths, _ := p.DiscoverLogPaths()
		for _, path := range paths {
			e.parseFile(p, path)
		}
	}
}

func (e *Engine) watchLoop() {
	for {
		select {
		case <-e.ctx.Done():
			return
		case event, ok := <-e.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				e.handleFileChange(event.Name)
			}
		case err, ok := <-e.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("watcher error", "error", err)
		}
	}
}

func (e *Engine) handleFileChange(filePath string) {
	for _, p := range e.parsers {
		e.parseFile(p, filePath)
	}
}

func (e *Engine) parseFile(p LogParser, filePath string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	offset := e.offsets[filePath]
	signals, newOffset, err := p.Parse(filePath, offset)
	if err != nil {
		slog.Debug("parse error", "parser", p.Name(), "file", filePath, "error", err)
		return
	}

	if len(signals) > 0 {
		e.processSignals(signals)
	}

	e.offsets[filePath] = newOffset
}

func (e *Engine) processSignals(signals []Signal) {
	for _, sig := range signals {
		// Sanitize sensitive data.
		if found, _ := e.config.ContainsSensitiveData(sig.Content); found {
			slog.Warn("dropping signal with sensitive data", "type", sig.Type)
			continue
		}

		slog.Info("captured signal",
			"type", sig.Type,
			"strength", sig.Strength,
			"agent", sig.AgentName,
			"content", truncate(sig.Content, 80),
		)

		// Store as a source entry (linked to a rule later by the distill engine).
		source := &storage.Source{
			ID:                    storage.NewID(),
			SignalType:            sig.Type,
			SignalStrength:        sig.Strength,
			AgentName:             sig.AgentName,
			ProjectPath:           sig.ProjectPath,
			RawSnippet:            truncate(sig.Content, 500),
			Timestamp:             storage.Now(),
			ConfidenceContribution: sig.Confidence,
		}
		if err := e.sourceDB.Create(source); err != nil {
			slog.Error("save signal", "error", err)
		}
	}
}

// --- Offset persistence ---

func (e *Engine) loadOffsets() {
	data, err := os.ReadFile(filepath.Join(e.offsetDir, "capture_offsets.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &e.offsets)
}

func (e *Engine) saveOffsets() {
	data, _ := json.Marshal(e.offsets)
	os.WriteFile(filepath.Join(e.offsetDir, "capture_offsets.json"), data, 0644)
}

// --- Pattern matching helpers ---

var (
	// Negation patterns (Chinese + English).
	negationPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(不对|别这么写|不要用|不应该|错了|别用|不要|不行|不是这样|no\s|don't|wrong|incorrect|shouldn't|not like this|stop using)`),
	}
	// Explicit mark patterns.
	markPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(记住|记住这条|以后都要|以后都这样|remember|note this|always do|from now on|make sure)`),
	}
)

// ClassifySignal determines signal type and strength from user input.
func ClassifySignal(text string) (signalType, strength string, confidence float64) {
	// Check for explicit marks first.
	for _, p := range markPatterns {
		if p.MatchString(text) {
			return "manual_mark", "strong", 0.95
		}
	}

	// Check for negation patterns.
	for _, p := range negationPatterns {
		if p.MatchString(text) {
			return "explicit_instruction", "strong", 0.85
		}
	}

	// Default: treat as a general instruction.
	return "explicit_instruction", "medium", 0.6
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// --- Claude Code Parser ---

// ClaudeCodeParser reads Claude Code session logs.
type ClaudeCodeParser struct {
	homeDir string
}

// NewClaudeCodeParser creates a parser for Claude Code logs.
func NewClaudeCodeParser() *ClaudeCodeParser {
	home, _ := os.UserHomeDir()
	return &ClaudeCodeParser{homeDir: home}
}

// Name returns the parser name.
func (p *ClaudeCodeParser) Name() string { return "claude_code" }

// DiscoverLogPaths finds all Claude Code session log files.
func (p *ClaudeCodeParser) DiscoverLogPaths() ([]string, error) {
	projectsDir := filepath.Join(p.homeDir, ".claude", "projects")
	var paths []string

	err := filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Claude Code stores sessions as JSONL files.
		if !info.IsDir() && (strings.HasSuffix(path, ".jsonl") || strings.HasSuffix(path, ".json")) {
			paths = append(paths, path)
		}
		return nil
	})

	return paths, err
}

// Parse reads a Claude Code log file from the given offset.
func (p *ClaudeCodeParser) Parse(filePath string, offset int64) ([]Signal, int64, error) {
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
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	// Extract project path from file path.
	projectPath := extractProjectPath(filePath)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // Skip malformed lines.
		}

		// Look for user messages (role: "human" or "user").
		role, _ := entry["role"].(string)
		if role != "human" && role != "user" {
			continue
		}

		content, _ := entry["content"].(string)
		if content == "" {
			continue
		}

		sigType, strength, confidence := ClassifySignal(content)
		if sigType != "" {
			signals = append(signals, Signal{
				Type:           sigType,
				Strength:       strength,
				Content:        content,
				AgentName:      "claude-code",
				ProjectPath:    projectPath,
				RawSnippet:     truncate(content, 500),
				Confidence:     confidence,
				SourceFilePath: filePath,
				SourceLineNum:  lineNum,
			})
		}
	}

	newOffset, _ := f.Seek(0, 2) // Current position = end of what we read.
	return signals, newOffset, nil
}

// extractProjectPath derives the project path from a Claude Code log file path.
func extractProjectPath(logPath string) string {
	// Claude Code stores logs under ~/.claude/projects/<encoded-project-path>/
	// The path is encoded (e.g., "-Users-dev-myproject").
	parts := strings.Split(logPath, string(filepath.Separator))
	for i, p := range parts {
		if p == "projects" && i+1 < len(parts) {
			encoded := parts[i+1]
			// Decode: replace "-" with "/".
			if strings.HasPrefix(encoded, "-") {
				return strings.ReplaceAll(encoded[1:], "-", "/")
			}
			return encoded
		}
	}
	return ""
}
