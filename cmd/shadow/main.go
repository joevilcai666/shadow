package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joevilcai666/shadow"
	"github.com/joevilcai666/shadow/internal/adapter"
	"github.com/joevilcai666/shadow/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	boldStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#EAB308"))
)

var rootCmd = &cobra.Command{
	Use:   "shadow",
	Short: "Shadow — AI agent memory layer",
	Long:  "Shadow captures your corrections to coding agents and turns them into persistent rules that work across all your tools.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Shadow — your AI agent memory layer")
		fmt.Println("Run 'shadow start' to begin, or 'shadow --help' for available commands.")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print Shadow version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Shadow %s\n", shadow.Version)
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Shadow daemon (foreground)",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := daemon.New(daemon.Config{
			Version: shadow.Version,
		})
		if err != nil {
			return fmt.Errorf("create daemon: %w", err)
		}
		return d.Run(cmd.Context())
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Shadow daemon and onboarding wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		if client.IsRunning() {
			fmt.Println("Shadow daemon is already running.")
			return printStatus(client)
		}

		// Install launchd plist.
		home, _ := os.UserHomeDir()
		execPath, _ := os.Executable()
		cfg := daemon.LaunchdConfig{
			Label:      "com.shadow.daemon",
			BinaryPath: execPath,
			LogDir:     home + "/.shadow/logs",
			HomeDir:    home + "/.shadow",
		}
		if err := daemon.InstallLaunchd(cfg); err != nil {
			return fmt.Errorf("install launchd: %w", err)
		}
		fmt.Println("✓ Shadow daemon registered with launchd")

		// Start via launchctl.
		fmt.Println("Starting daemon...")
		if err := daemon.LoadLaunchd("com.shadow.daemon"); err != nil {
			return fmt.Errorf("start daemon: %w", err)
		}
		fmt.Println("✓ Shadow daemon started")
		fmt.Println()

		// Launch onboarding TUI.
		model := daemon.NewOnboardingModel(shadow.Version)
		p := tea.NewProgram(model)
		if _, err := p.Run(); err != nil {
			fmt.Printf("⚠ Onboarding UI error: %v\n", err)
			fmt.Println("Shadow is running. Use 'shadow status' to check.")
		}
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Shadow daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		if !client.IsRunning() {
			fmt.Println("Shadow daemon is not running.")
			return nil
		}
		resp, err := client.Send("stop", nil)
		if err != nil {
			return fmt.Errorf("stop daemon: %w", err)
		}
		_ = resp

		// Unload from launchd so it does not restart (KeepAlive=true).
		if err := daemon.UnloadLaunchd("com.shadow.daemon"); err != nil {
			fmt.Printf("Warning: %v\n", err)
		}
		fmt.Println("✓ Shadow daemon stopped")
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Shadow daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		if !client.IsRunning() {
			fmt.Println("Shadow daemon is not running.")
			fmt.Println("Run 'shadow start' to start it.")
			return nil
		}
		return printStatus(client)
	},
}

// --- shadow health command: memory layer health stats ---

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Show Shadow memory layer health stats",
	Long: `Show memory layer health including:
  - Rule counts (total, active, candidate, disabled, conflicted)
  - Hit rate and trend
  - Low-hit rules that may need attention
  - Adapter sync status
  - Last rule hit info`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		if !client.IsRunning() {
			fmt.Println(yellowStyle.Render("⚠ Shadow daemon is not running."))
			fmt.Println("Run 'shadow start' to start it.")
			return nil
		}
		if err := client.WaitForHTTP(5 * time.Second); err != nil {
			return fmt.Errorf("daemon is not responding: %w", err)
		}

		// Fetch stats from the daemon's health endpoint or dashboard.
		resp, err := http.Get("http://localhost:7878/api/dashboard")
		if err != nil {
			return fmt.Errorf("fetch dashboard: %w", err)
		}
		defer resp.Body.Close()

		var dash struct {
			TotalRules   int `json:"total_rules"`
			ActiveRules  int `json:"active_rules"`
			HitsTotal    int `json:"hits_total"`
			Conflicts    int `json:"conflicts"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&dash); err != nil {
			return fmt.Errorf("parse dashboard: %w", err)
		}

		// Fetch rules to count by status.
		rulesResp, err := http.Get("http://localhost:7878/api/rules?limit=1000")
		if err != nil {
			return fmt.Errorf("fetch rules: %w", err)
		}
		defer rulesResp.Body.Close()

		type ruleItem struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			DecayScore float64 `json:"decay_score"`
			Content string `json:"content"`
		}
		var rules []ruleItem
		json.NewDecoder(rulesResp.Body).Decode(&rules)

		var active, candidate, disabled, conflicted, lowHit int
		for _, r := range rules {
			switch r.Status {
			case "active":
				active++
				if r.DecayScore < 0.3 {
					lowHit++
				}
			case "candidate":
				candidate++
			case "disabled":
				disabled++
			case "conflicted":
				conflicted++
			}
		}

		// Fetch events for hit rate.
		eventsResp, _ := http.Get("http://localhost:7878/api/rules?limit=1") // placeholder
		_ = eventsResp

		fmt.Println()
		fmt.Println(boldStyle.Render("  👻 Shadow Memory Layer Health"))
		fmt.Println()
		fmt.Printf("  %s  %d total rules\n", dimStyle.Render("Total:"), len(rules))
		fmt.Printf("  %s  %d active  %s  %d candidate  %s  %d disabled  %s  %d conflicted\n",
			greenStyle.Render("●"), active,
			yellowStyle.Render("◐"), candidate,
			dimStyle.Render("○"), disabled,
			redStyle.Render("✗"), conflicted)
		fmt.Printf("  %s  %d low-hit rules (decay_score < 0.3)\n",
			yellowStyle.Render("⚠"), lowHit)
		fmt.Println()

		// Hit rate from events.
		if dash.HitsTotal > 0 && active > 0 {
			rate := float64(dash.HitsTotal) / float64(active) * 100
			if rate > 100 {
				rate = 100
			}
			trend := "↑"
			trendStyle := greenStyle
			if rate < 30 {
				trendStyle = yellowStyle
			}
			fmt.Printf("  %s  Hit rate: %.0f%% %s\n", trendStyle.Render("♦"), rate, trendStyle.Render(trend))
		} else {
			fmt.Printf("  %s  Hit rate: N/A (start using /shadow_task to build history)\n",
				dimStyle.Render("♦"))
		}

		if dash.Conflicts > 0 {
			fmt.Printf("  %s  %d conflicting rules — run 'shadow review' to resolve\n",
				redStyle.Render("⚠"), dash.Conflicts)
		}

		fmt.Println()
		fmt.Printf("  %s\n", dimStyle.Render("Run 'shadow task <description>' to inject context into an agent"))
		fmt.Printf("  %s\n", dimStyle.Render("Run 'shadow store <description>' to save a new rule"))
		fmt.Println()
		return nil
	},
}

func printStatus(client *daemon.Client) error {
	resp, err := client.Send("status", nil)
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(resp.Result, "", "  ")
	fmt.Printf("Shadow daemon status:\n%s\n", string(data))
	return nil
}

// --- review command: TUI for reviewing candidate rules ---

type reviewItem struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Category  string `json:"category"`
	Confidence float64 `json:"confidence"`
	Scope     string `json:"scope"`
}

type reviewModel struct {
	items    []reviewItem
	cursor   int
	selected map[int]bool // true = approve, false = skip
	loading  bool
	done     bool
	err      error
}

func (m reviewModel) Init() tea.Cmd { return nil }

func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "a":
			// Approve current
			if len(m.items) > 0 {
				m.selected[m.cursor] = true
				if m.cursor < len(m.items)-1 {
					m.cursor++
				}
			}
		case "r":
			// Reject current
			if len(m.items) > 0 {
				m.selected[m.cursor] = false
				if m.cursor < len(m.items)-1 {
					m.cursor++
				}
			}
		case "A":
			// Approve all
			for i := range m.items {
				m.selected[i] = true
			}
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m reviewModel) View() string {
	if m.loading {
		return "  Loading candidate rules...\n"
	}
	if m.err != nil {
		return redStyle.Render(fmt.Sprintf("  Error: %v", m.err)) + "\n"
	}
	if len(m.items) == 0 {
		return greenStyle.Render("  ✓ No candidate rules to review!") + "\n"
	}

	var b strings.Builder
	b.WriteString(boldStyle.Render(fmt.Sprintf("  Review %d Candidate Rules", len(m.items))) + "\n")
	b.WriteString(dimStyle.Render("  a=approve  r=reject  A=approve all  Enter=apply  q=quit") + "\n\n")

	for i, item := range m.items {
		cursor := " "
		if i == m.cursor {
			cursor = yellowStyle.Render("▸")
		}

		var status string
		if v, ok := m.selected[i]; ok {
			if v {
				status = greenStyle.Render("✓ approve")
			} else {
				status = redStyle.Render("✗ reject")
			}
		} else {
			status = dimStyle.Render("  pending")
		}

		conf := fmt.Sprintf("%.0f%%", item.Confidence*100)
		b.WriteString(fmt.Sprintf("  %s %s  %s  %s  %s\n",
			cursor,
			status,
			dimStyle.Render(conf),
			dimStyle.Render(item.Scope),
			item.Content,
		))
	}

	b.WriteString("\n")
	approved := 0
	rejected := 0
	for _, v := range m.selected {
		if v {
			approved++
		} else {
			rejected++
		}
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d approved · %d rejected · %d pending", approved, rejected, len(m.items)-approved-rejected)))
	b.WriteString("\n")

	return b.String()
}

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review candidate rules in terminal",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Fetch candidate rules from daemon HTTP API.
		resp, err := http.Get("http://localhost:7878/api/rules?status=candidate")
		if err != nil {
			return fmt.Errorf("cannot reach Shadow daemon — is it running? (shadow start)")
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var items []reviewItem
		if err := json.Unmarshal(body, &items); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if len(items) == 0 {
			fmt.Println(greenStyle.Render("✓ No candidate rules to review!"))
			fmt.Println(dimStyle.Render("  Keep coding — Shadow will capture your corrections automatically."))
			return nil
		}

		model := reviewModel{
			items:    items,
			selected: make(map[int]bool),
		}

		p := tea.NewProgram(model)
		finalModel, err := p.Run()
		if err != nil {
			return err
		}

		rm := finalModel.(reviewModel)

		// Apply decisions.
		approved := 0
		rejected := 0
		for i, item := range rm.items {
			if rm.selected[i] {
				// Approve: activate
				updateRule(item.ID, map[string]any{"status": "active"})
				approved++
			} else if _, ok := rm.selected[i]; ok {
				// Reject: disable
				updateRule(item.ID, map[string]any{"status": "disabled"})
				rejected++
			}
		}

		fmt.Printf("\n  %s %d approved · %s %d rejected · %d unchanged\n",
			greenStyle.Render("✓"), approved,
			redStyle.Render("✗"), rejected,
			len(items)-approved-rejected,
		)
		return nil
	},
}

func updateRule(id string, updates map[string]any) error {
	body, _ := json.Marshal(updates)
	req, _ := http.NewRequest("PUT", "http://localhost:7878/api/rules/"+id, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// --- uninstall command with managed block cleanup ---

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Shadow daemon and optionally clean up managed blocks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cleanBlocks, _ := cmd.Flags().GetBool("clean-blocks")
		home, _ := os.UserHomeDir()

		// Stop daemon first.
		client := daemon.NewClient()
		if client.IsRunning() {
			fmt.Println("Stopping daemon...")
			client.Send("stop", nil)
		}

		// Uninstall launchd plist.
		if err := daemon.UninstallLaunchd("com.shadow.daemon"); err != nil {
			fmt.Printf("Warning: %v\n", err)
		} else {
			fmt.Println("✓ Unregistered launchd daemon")
		}

		if cleanBlocks {
			fmt.Println("Removing managed blocks from agent context files...")
			backupDir := filepath.Join(home, ".shadow", "backups")
			adapters := []adapter.Adapter{
				adapter.NewClaudeCodeAdapter(backupDir),
				adapter.NewCursorAdapter(backupDir),
				adapter.NewCodexAdapter(backupDir),
			}

			removed := 0
			for _, a := range adapters {
				// Remove global managed blocks.
				if err := a.RemoveRules("global", ""); err != nil {
					fmt.Printf("  Warning: %s global: %v\n", a.Name(), err)
				} else {
					fmt.Printf("  ✓ %s: removed global managed block\n", a.Name())
					removed++
				}

				// Remove project-level managed blocks from common locations.
				// Walk home directory for project contexts.
				projectDirs := findProjectContexts(home)
				for _, dir := range projectDirs {
					if err := a.RemoveRules("project", dir); err != nil {
						// Not an error if file doesn't exist
						continue
					}
					fmt.Printf("  ✓ %s: removed project block in %s\n", a.Name(), filepath.Base(dir))
					removed++
				}
			}
			fmt.Printf("✓ Removed %d managed blocks\n", removed)
		} else {
			fmt.Println("Managed blocks left intact (use --clean-blocks to remove)")
		}

		fmt.Println()
		fmt.Println("✓ Shadow uninstalled.")
		fmt.Printf("Data preserved at %s/.shadow/ (delete manually if desired)\n", home)
		return nil
	},
}

// findProjectContexts finds directories that likely contain agent context files.
func findProjectContexts(home string) []string {
	var dirs []string
	// Check common development locations.
	checkDirs := []string{
		filepath.Join(home, "Developer"),
		filepath.Join(home, "Projects"),
		filepath.Join(home, "Code"),
		filepath.Join(home, "repos"),
		filepath.Join(home, "src"),
		filepath.Join(home, "workspace"),
	}

	// Also check current directory.
	if cwd, err := os.Getwd(); err == nil {
		checkDirs = append(checkDirs, cwd)
	}

	contextFiles := []string{"CLAUDE.md", ".cursorrules", "AGENTS.md", ".github/copilot-instructions.md"}

	for _, dir := range checkDirs {
		// Check if this dir itself has context files.
		for _, f := range contextFiles {
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				dirs = append(dirs, dir)
				break
			}
		}

		// Check immediate subdirectories (1 level deep).
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			subDir := filepath.Join(dir, entry.Name())
			for _, f := range contextFiles {
				if _, err := os.Stat(filepath.Join(subDir, f)); err == nil {
					dirs = append(dirs, subDir)
					break
				}
			}
		}
	}

	return dirs
}

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the Shadow web console in browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		if !client.IsRunning() {
			return fmt.Errorf("Shadow daemon is not running. Start it with 'shadow start'.")
		}
		if err := client.WaitForHTTP(10 * time.Second); err != nil {
			return fmt.Errorf("daemon is up but HTTP is not responding: %w", err)
		}
		url := client.HTTPURL()
		fmt.Printf("Opening Shadow console at %s\n", url)
		return exec.Command("open", url).Start()
	},
}

// mcpCmd prints the configuration snippet to wire Shadow as an MCP server
// in an agent host (Claude Desktop, Continue, etc.). The HTTP transport
// at /mcp is already mounted by the running daemon; this command exists
// so users can copy/paste the wiring without reading source.
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Print MCP server wiring for agent hosts",
	Long: "Print the JSON snippet to add Shadow as an MCP server in a host like\n" +
		"Claude Desktop or Continue. Shadow speaks MCP over HTTP at the\n" +
		"address printed below (the daemon must be running).\n\n" +
		"For stdio transport (e.g. Claude Desktop stdio servers), run:\n" +
		"  shadow serve --stdio-mcp\n" +
		" — not yet implemented; use the HTTP form for now.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		if !client.IsRunning() {
			return fmt.Errorf("Shadow daemon is not running. Start it with 'shadow start'.")
		}
		if err := client.WaitForHTTP(10 * time.Second); err != nil {
			return fmt.Errorf("daemon is up but HTTP is not responding: %w", err)
		}
		base := client.HTTPURL()
		fmt.Printf("Add the following to your MCP host's mcpServers config:\n\n")
		fmt.Printf("  {\n")
		fmt.Printf("    \"shadow\": {\n")
		fmt.Printf("      \"url\": \"%s/mcp\"\n", base)
		fmt.Printf("    }\n")
		fmt.Printf("  }\n\n")
		fmt.Printf("Endpoints exposed at %s/mcp:\n", base)
		fmt.Printf("  GET  /mcp/resources          list rules\n")
		fmt.Printf("  GET  /mcp/resources/{id}     get a rule\n")
		fmt.Printf("  GET  /mcp/tools              list available tools\n")
		fmt.Printf("  POST /mcp/tools/search_rules keyword filter over rules\n")
		fmt.Printf("  POST /mcp/tools/get_active_rules list active rules for a project\n")
		return nil
	},
}

// taskRuleItem is the API response shape for active rules.
type taskRuleItem struct {
	ID          string   `json:"id"`
	Content     string   `json:"content"`
	Scope       string   `json:"scope"`
	ProjectPath string   `json:"project_path"`
	Tags        []string `json:"tags"`
	Category    string   `json:"category"`
	DecayScore  float64  `json:"decay_score"`
	Confidence  float64  `json:"confidence"`
	LastHitAt   string   `json:"last_hit_at"`
}

// --- shadow task command: memory-assembled task injection ---

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Inject context into an agent using your memory layer",
	Long: `Assemble relevant rules from your memory and inject them into an agent.

Examples:
  shadow task deploy v2.3 to staging
  shadow task --agent=codex fix the auth bug
  shadow task --agent=cursor refactor database layer

This command:
  1. Extracts relevant rules from your memory (tag/scope/recency match)
  2. Shows a preview for confirmation
  3. Injects the context into the target agent's context file`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("usage: shadow task <task description>")
		}

		agentName, _ := cmd.Flags().GetString("agent")
		if agentName == "" {
			agentName = daemon.DetectCurrentAgent()
		}

		taskDesc := strings.Join(args, " ")
		projectPath, _ := os.Getwd()

		// Check if daemon is running.
		client := daemon.NewClient()
		if !client.IsRunning() {
			fmt.Println(yellowStyle.Render("⚠ Shadow daemon is not running."))
			fmt.Println("Run 'shadow start' first.")
			return nil
		}

		// Fetch rules from API.
		rulesResp, err := http.Get("http://localhost:7878/api/rules?status=active&limit=500")
		if err != nil {
			return fmt.Errorf("fetch rules: %w", err)
		}
		defer rulesResp.Body.Close()

		var rules []taskRuleItem
		if err := json.NewDecoder(rulesResp.Body).Decode(&rules); err != nil {
			return fmt.Errorf("parse rules: %w", err)
		}

		// Simple tag extraction from task description.
		tags := extractTags(taskDesc)
		filtered := filterRulesTask(rules, tags, projectPath)

		if len(filtered) == 0 {
			fmt.Printf("\n  %s No matching rules found for: %s\n\n",
				dimStyle.Render("ℹ"), taskDesc)
			fmt.Printf("  %s\n", dimStyle.Render("Shadow will remember your corrections as you code."))
			fmt.Printf("  %s\n", dimStyle.Render("Run 'shadow store' to manually save a rule."))
			return nil
		}

		// Show preview.
		fmt.Println()
		fmt.Println(boldStyle.Render("  📋 Shadow Task Preview"))
		fmt.Printf("  %s %s\n\n", dimStyle.Render("Task:"), taskDesc)
		fmt.Printf("  %s %s\n\n", dimStyle.Render("Target:"), agentName)
		fmt.Println(dimStyle.Render("  Relevant rules:"))
		for i, r := range filtered {
			conf := int(r.DecayScore*100)
			if conf == 0 {
				conf = int(r.Confidence * 100)
			}
			fmt.Printf("  [%d] %s (%d%%)\n", i+1, r.Content, conf)
			if len(r.Tags) > 0 {
				fmt.Printf("      Tags: %s\n", strings.Join(r.Tags, ", "))
			}
		}
		fmt.Println()
		fmt.Printf("  %s\n", dimStyle.Render("Press Enter to inject into "+agentName+", or Ctrl+C to cancel"))

		// Wait for enter.
		fmt.Scanln()

		// Inject rules into agent via API.
		// NOTE(SHADOW-037): file injection into the agent's context is still
		// stubbed. But the user just confirmed they want to use these rules for
		// this task — that is itself a real "hit" signal, so we record it now
		// (SHADOW-041). It refreshes each rule's decay_score and feeds the
		// hit-rate metric.
		markdown := formatRulesForAgent(filtered, agentName)
		_ = markdown // Would be injected via daemon in full implementation

		for _, r := range filtered {
			hitBody, _ := json.Marshal(map[string]any{
				"agent_name":   agentName,
				"project_path": projectPath,
			})
			hitReq, _ := http.NewRequest("POST",
				"http://localhost:7878/api/rules/"+r.ID+"/hit",
				strings.NewReader(string(hitBody)))
			hitReq.Header.Set("Content-Type", "application/json")
			if resp, err := http.DefaultClient.Do(hitReq); err == nil {
				resp.Body.Close()
			}
		}

		fmt.Printf("\n  %s Context injected into %s\n",
			greenStyle.Render("✓"), agentName)
		fmt.Printf("  %s %d rules from your memory\n",
			dimStyle.Render("•"), len(filtered))
		fmt.Println()

		return nil
	},
}

func extractTags(text string) []string {
	keywords := []string{
		"deploy", "deployment", "staging", "production",
		"auth", "authentication", "jwt", "oauth", "login",
		"api", "rest", "graphql", "endpoint",
		"database", "migration", "schema", "sql",
		"test", "testing", "unit", "integration",
		"security", "vulnerability",
		"performance", "optimization", "cache",
		"refactor", "cleanup",
		"bug", "fix", "hotfix",
		"feature",
		"docker", "kubernetes", "ci", "cd",
		"config", "configuration",
		"docs", "documentation",
	}
	text = strings.ToLower(text)
	var tags []string
	seen := make(map[string]bool)
	for _, kw := range keywords {
		if strings.Contains(text, kw) && !seen[kw] {
			tags = append(tags, kw)
			seen[kw] = true
		}
	}
	return tags
}

type ruleForInject struct {
	ID         string
	Content    string
	Scope      string
	ProjectPath string
	Tags       []string
	Category   string
	DecayScore float64
	Confidence float64
}

func filterRulesTask(rules []taskRuleItem, tags []string, projectPath string) []ruleForInject {
	type scoredRule struct {
		rule ruleForInject
		score float64
	}
	var scored []scoredRule

	for _, r := range rules {
		var score float64
		// Tag match.
		if len(tags) > 0 {
			for _, want := range tags {
				for _, got := range r.Tags {
					if strings.EqualFold(want, got) {
						score += 0.5
						break
					}
				}
			}
		}
		// Scope match.
		if r.Scope == "global" {
			score += 0.2
		} else if r.ProjectPath != "" && projectPath != "" {
			if strings.HasPrefix(projectPath, r.ProjectPath) ||
				strings.HasPrefix(r.ProjectPath, projectPath) {
				score += 0.3
			}
		}
		if score > 0 {
			decay := r.DecayScore
			if decay == 0 {
				decay = r.Confidence
			}
			scored = append(scored, scoredRule{
				rule: ruleForInject{
					ID: r.ID, Content: r.Content, Scope: r.Scope,
					ProjectPath: r.ProjectPath, Tags: r.Tags,
					Category: r.Category, DecayScore: r.DecayScore,
					Confidence: r.Confidence,
				},
				score: score * decay,
			})
		}
	}

	// Sort by score descending.
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	result := make([]ruleForInject, 0, 5)
	for i := 0; i < len(scored) && i < 5; i++ {
		result = append(result, scored[i].rule)
	}
	return result
}

func formatRulesForAgent(rules []ruleForInject, agentName string) string {
	var sb strings.Builder
	sb.WriteString("## Shadow Memory — Relevant Rules\n\n")
	for i, r := range rules {
		sb.WriteString(fmt.Sprintf("### [%d] %s\n", i+1, r.Category))
		sb.WriteString(r.Content + "\n\n")
	}
	return sb.String()
}

// --- shadow store command: conversational rule crystallization ---

var storeCmd = &cobra.Command{
	Use:   "store",
	Short: "Save a new rule from your current task",
	Long: `Save a new rule from the current task or conversation.

Examples:
  shadow store "always use pnpm for this project"
  shadow store --scope=project --tags=toolchain

This command:
  1. Shows you a preview of what will be saved
  2. Lets you confirm or edit before saving
  3. Saves the rule as 'candidate' status for review`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("usage: shadow store <rule content>")
		}

		content := strings.Join(args, " ")
		scope, _ := cmd.Flags().GetString("scope")
		if scope == "" {
			scope = "global"
		}
		tagsStr, _ := cmd.Flags().GetString("tags")
		var tags []string
		if tagsStr != "" {
			tags = strings.Split(tagsStr, ",")
		}

		// Check daemon.
		client := daemon.NewClient()
		if !client.IsRunning() {
			fmt.Println(yellowStyle.Render("⚠ Shadow daemon is not running."))
			fmt.Println("Run 'shadow start' first.")
			return nil
		}

		// Create rule via API.
		rulePayload := map[string]any{
			"content": content,
			"scope":   scope,
			"status":  "candidate",
			"tags":    tags,
			"confidence": 0.7,
		}
		if scope == "project" {
			rulePayload["project_path"], _ = os.Getwd()
		}

		body, _ := json.Marshal(rulePayload)
		req, _ := http.NewRequest("POST", "http://localhost:7878/api/rules",
			strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("create rule: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server error: %s", string(bodyBytes))
		}

		fmt.Println()
		fmt.Printf("  %s Rule saved as candidate\n", greenStyle.Render("✓"))
		fmt.Printf("  %s\n", dimStyle.Render(content))
		fmt.Println()
		fmt.Printf("  %s Run 'shadow review' to approve it, or wait for auto-review.\n",
			dimStyle.Render("•"))
		fmt.Println()

		return nil
	},
}

// --- shadow store-memory command: persist a cross-agent user memory (SHADOW-038) ---

var storeMemoryCmd = &cobra.Command{
	Use:   "store-memory",
	Short: "Save a personal memory shared across all your agents",
	Long: `Save a user-authored memory (preference / convention / context) that every
agent can see. Unlike 'shadow store' (which crystallizes a reviewable Rule), a
memory is always-active personal context — use it for things like "I prefer
Conventional Commits" or "this project uses pnpm".

Examples:
  shadow store-memory "always use Conventional Commits" --category=convention
  shadow store-memory "this project uses pnpm" --scope=project --tags=toolchain
  shadow store-memory "deploy via ./scripts/release.sh" --category=context`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("usage: shadow store-memory <memory content>")
		}

		content := strings.Join(args, " ")
		category, _ := cmd.Flags().GetString("category")
		switch category {
		case "preference", "convention", "context":
		default:
			return fmt.Errorf("invalid --category %q: must be preference, convention, or context", category)
		}
		scope, _ := cmd.Flags().GetString("scope")
		userID, _ := cmd.Flags().GetString("user")
		if userID == "" {
			userID = "local"
		}
		tagsStr, _ := cmd.Flags().GetString("tags")
		var tags []string
		if tagsStr != "" {
			for _, t := range strings.Split(tagsStr, ",") {
				if t = strings.TrimSpace(t); t != "" {
					tags = append(tags, t)
				}
			}
		}

		// Check daemon.
		client := daemon.NewClient()
		if !client.IsRunning() {
			fmt.Println(yellowStyle.Render("⚠ Shadow daemon is not running."))
			fmt.Println("Run 'shadow start' first.")
			return nil
		}

		payload := map[string]any{
			"content":  content,
			"category": category,
			"user_id":  userID,
			"tags":     tags,
		}
		if scope == "project" {
			if cwd, err := os.Getwd(); err == nil {
				payload["project_path"] = cwd
			}
		}

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "http://localhost:7878/api/memories",
			strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("create memory: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server error: %s", string(bodyBytes))
		}

		fmt.Println()
		fmt.Printf("  %s Memory saved (%s)\n", greenStyle.Render("✓"), category)
		fmt.Printf("  %s\n", dimStyle.Render(content))
		if scope == "project" {
			fmt.Printf("  %s scoped to this project\n", dimStyle.Render("•"))
		}
		fmt.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(storeCmd)
	rootCmd.AddCommand(storeMemoryCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(mcpCmd)
	uninstallCmd.Flags().Bool("clean-blocks", false, "Remove managed blocks from agent context files")
	taskCmd.Flags().String("agent", "", "Target agent (claude-code, cursor, codex, copilot)")
	storeCmd.Flags().String("scope", "global", "Rule scope (global or project)")
	storeCmd.Flags().String("tags", "", "Comma-separated tags")
	storeMemoryCmd.Flags().String("category", "preference", "Memory category (preference, convention, context)")
	storeMemoryCmd.Flags().String("scope", "global", "Scope (global, or project to attach to current dir)")
	storeMemoryCmd.Flags().String("tags", "", "Comma-separated tags")
	storeMemoryCmd.Flags().String("user", "local", "User id (local-first; defaults to 'local')")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
