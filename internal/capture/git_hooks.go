package capture

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitHooks manages Shadow's git hooks for capturing implicit correction signals.
//
// Hook scripts use #!/bin/sh and curl. On Windows, Git for Windows (which
// includes Git Bash) is required for hooks to execute. Without it hooks
// silently fail — capture simply misses those implicit signals.
type GitHooks struct {
	homeDir  string
	sockPath string
}

// NewGitHooks creates a new git hooks manager.
func NewGitHooks(homeDir, sockPath string) *GitHooks {
	return &GitHooks{homeDir: homeDir, sockPath: sockPath}
}

const shadowMarker = "# >>> shadow hook >>>"

// Install installs Shadow git hooks into a project's .git/hooks directory.
func (g *GitHooks) Install(projectPath string) error {
	hooksDir := filepath.Join(projectPath, ".git", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", projectPath)
	}

	hooks := map[string]string{
		"post-checkout": g.postCheckoutScript(),
		"post-rewrite":  g.postRewriteScript(),
	}

	for name, script := range hooks {
		hookPath := filepath.Join(hooksDir, name)
		if err := g.installHook(hookPath, script); err != nil {
			return fmt.Errorf("install %s hook: %w", name, err)
		}
	}

	return nil
}

// Uninstall removes Shadow hooks from a project.
func (g *GitHooks) Uninstall(projectPath string) error {
	hooksDir := filepath.Join(projectPath, ".git", "hooks")

	hooks := []string{"post-checkout", "post-rewrite"}
	for _, name := range hooks {
		hookPath := filepath.Join(hooksDir, name)
		g.removeShadowFromHook(hookPath)
	}

	return nil
}

func (g *GitHooks) installHook(hookPath, script string) error {
	// Check if hook already exists with Shadow marker.
	if data, err := os.ReadFile(hookPath); err == nil {
		if strings.Contains(string(data), shadowMarker) {
			return nil // Idempotent: already installed.
		}
		// Append Shadow section to existing hook.
		f, err := os.OpenFile(hookPath, os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString("\n" + script)
		return err
	}

	// Create new hook file.
	return os.WriteFile(hookPath, []byte("#!/bin/sh\n"+script), 0755)
}

func (g *GitHooks) removeShadowFromHook(hookPath string) {
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return
	}

	content := string(data)
	if !strings.Contains(content, shadowMarker) {
		return
	}

	// Remove Shadow block.
	lines := strings.Split(content, "\n")
	var cleaned []string
	inShadowBlock := false
	for _, line := range lines {
		if strings.Contains(line, shadowMarker) {
			inShadowBlock = true
			continue
		}
		if inShadowBlock && strings.Contains(line, "<<< shadow hook <<<") {
			inShadowBlock = false
			continue
		}
		if !inShadowBlock {
			cleaned = append(cleaned, line)
		}
	}

	result := strings.Join(cleaned, "\n")
	if strings.TrimSpace(result) == "" || result == "#!/bin/sh\n" {
		os.Remove(hookPath)
	} else {
		os.WriteFile(hookPath, []byte(result), 0755)
	}
}

func (g *GitHooks) postCheckoutScript() string {
	return fmt.Sprintf(`%s
# Shadow: detect git reset/revert
PREV_HEAD="$1"
NEW_HEAD="$2"
IS_BRANCH="$3"
if [ "$IS_BRANCH" = "0" ]; then
  curl -s -X POST http://localhost:7878/api/capture/git-signal \
    -H "Content-Type: application/json" \
    -d "{\"type\":\"git_checkout\",\"prev\":\"$PREV_HEAD\",\"new\":\"$NEW_HEAD\",\"pwd\":\"$(pwd)\"}" > /dev/null 2>&1 &
fi
# <<< shadow hook <<<`, shadowMarker)
}

func (g *GitHooks) postRewriteScript() string {
	return fmt.Sprintf(`%s
# Shadow: detect rebase/amend
curl -s -X POST http://localhost:7878/api/capture/git-signal \
  -H "Content-Type: application/json" \
  -d "{\"type\":\"git_rewrite\",\"op\":\"$1\",\"pwd\":\"$(pwd)\"}" > /dev/null 2>&1 &
# <<< shadow hook <<<`, shadowMarker)
}

// AnalyzeDiff computes the change ratio between two git refs.
// Returns the percentage of lines changed (0.0-1.0).
func AnalyzeDiff(projectPath, fromRef, toRef string) (float64, error) {
	cmd := exec.Command("git", "-C", projectPath, "diff", "--stat", fromRef, toRef)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git diff: %w", err)
	}

	// Parse diff stat to get change ratio.
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return 0, nil
	}

	// Last line is summary: "X files changed, Y insertions(+), Z deletions(-)"
	summary := lines[len(lines)-1]
	insertions := countChanges(summary, "insertion")
	deletions := countChanges(summary, "deletion")

	totalChanges := insertions + deletions
	if totalChanges == 0 {
		return 0, nil
	}

	// Get total lines in the repo for context.
	totalCmd := exec.Command("git", "-C", projectPath, "ls-files")
	totalOutput, _ := totalCmd.Output()
	fileCount := len(strings.Split(strings.TrimSpace(string(totalOutput)), "\n"))
	if fileCount == 0 {
		fileCount = 1
	}

	// Ratio: changes per file (rough heuristic).
	ratio := float64(totalChanges) / float64(fileCount)
	if ratio > 1.0 {
		ratio = 1.0
	}
	return ratio, nil
}

func countChanges(summary, keyword string) int {
	parts := strings.Split(summary, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.Contains(p, keyword) {
			var count int
			fmt.Sscanf(p, "%d", &count)
			return count
		}
	}
	return 0
}
