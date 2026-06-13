package daemon

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joevilcai666/shadow/internal/adapter"
	"github.com/joevilcai666/shadow/internal/distill"
	"github.com/joevilcai666/shadow/internal/storage"
)

// TaskCommand handles the /shadow_task CLI command logic.
type TaskCommand struct {
	db       *sql.DB
	ruleRepo *storage.RuleRepo
}

// NewTaskCommand creates a TaskCommand.
func NewTaskCommand(db *sql.DB, ruleRepo *storage.RuleRepo) *TaskCommand {
	return &TaskCommand{db: db, ruleRepo: ruleRepo}
}

// TaskResult holds the extracted context for a task.
type TaskResult struct {
	ExtractedContext *distill.ExtractedContext `json:"extracted_context"`
	Markdown         string                    `json:"markdown"`
	ProjectPath      string                    `json:"project_path"`
	AgentName        string                    `json:"agent_name"`
}

// RunTask extracts context for a task description and returns a preview.
func (tc *TaskCommand) RunTask(taskDesc, agentName, projectPath string) (*TaskResult, error) {
	if projectPath == "" {
		projectPath, _ = os.Getwd()
	}

	// Extract tags from task description (simple keyword extraction).
	tags := distill.ExtractTagsFromText(taskDesc)

	engine := distill.NewContextEngine(tc.ruleRepo)
	req := distill.TaskContextRequest{
		TaskDescription: taskDesc,
		ProjectPath:     projectPath,
		AgentName:       agentName,
		Tags:            tags,
		MaxRules:        5,
	}

	ctx, err := engine.Extract(req)
	if err != nil {
		return nil, fmt.Errorf("extract context: %w", err)
	}

	markdown, err := engine.ExtractForAgent(req)
	if err != nil {
		return nil, fmt.Errorf("format context: %w", err)
	}

	return &TaskResult{
		ExtractedContext: ctx,
		Markdown:         markdown,
		ProjectPath:      projectPath,
		AgentName:        agentName,
	}, nil
}

// InjectIntoAgent writes the context to the target agent's context file.
func (tc *TaskCommand) InjectIntoAgent(agentName string, rules []*storage.Rule, projectPath string) error {
	backupDir := filepath.Join(os.Getenv("HOME"), ".shadow", "backups")

	var a adapter.Adapter
	switch agentName {
	case "claude-code":
		a = adapter.NewClaudeCodeAdapter(backupDir)
	case "cursor":
		a = adapter.NewCursorAdapter(backupDir)
	case "codex":
		a = adapter.NewCodexAdapter(backupDir)
	case "openclaw":
		a = adapter.NewOpenClawAdapter(backupDir)
	case "copilot":
		a = adapter.NewCopilotAdapter(backupDir)
	default:
		return fmt.Errorf("unknown agent: %s", agentName)
	}

	if len(rules) == 0 {
		return nil
	}

	if projectPath != "" {
		return a.WriteRules(rules, "project", projectPath)
	}
	return a.WriteRules(rules, "global", "")
}

// DetectCurrentAgent returns the agent name based on common heuristics.
func DetectCurrentAgent() string {
	// Check environment variables that each agent sets.
	if os.Getenv("CLAUDE_CODE_SESSION_ID") != "" {
		return "claude-code"
	}
	if os.Getenv("CURSOR_SESSION_ID") != "" {
		return "cursor"
	}
	if os.Getenv("OPENAI_API_TYPE") != "" || os.Getenv("CODEX_SESSION") != "" {
		return "codex"
	}

	// Check parent processes.
	if isRunningIn("claude") || isRunningIn("anthropic") {
		return "claude-code"
	}
	if isRunningIn("cursor") {
		return "cursor"
	}
	if isRunningIn("codex") {
		return "codex"
	}
	if isRunningIn("github-copilot") || isRunningIn("copilot") {
		return "copilot"
	}

	return "claude-code" // default
}

func isRunningIn(name string) bool {
	// Simple heuristic: check if parent process name contains name.
	data, _ := exec.Command("ps", "ax", "-o", "comm=").Output()
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), name) {
			return true
		}
	}
	return false
}

// GenerateSessionID creates a random session ID.
func GenerateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
