package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joevilcai666/shadow"
	"github.com/joevilcai666/shadow/internal/adapter"
	"github.com/joevilcai666/shadow/internal/config"
	"github.com/joevilcai666/shadow/internal/daemon"
	"github.com/joevilcai666/shadow/internal/storage"
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

		// On Windows, if running as a service, delegate to the SCM handler.
		if runtime.GOOS == "windows" && daemon.IsWindowsService() {
			return daemon.RunAsService(cmd.Context(), d)
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

		execPath, _ := os.Executable()

		if err := installDaemon(execPath); err != nil {
			return err
		}
		if err := startDaemonService(); err != nil {
			return err
		}
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

		stopDaemonService()
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
			TotalRules  int `json:"total_rules"`
			ActiveRules int `json:"active_rules"`
			HitsTotal   int `json:"hits_total"`
			Conflicts   int `json:"conflicts"`
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
			Status string `json:"status"`
		}
		var rules []ruleItem
		json.NewDecoder(rulesResp.Body).Decode(&rules)

		var active, candidate, disabled, conflicted int
		for _, r := range rules {
			switch r.Status {
			case "active":
				active++
			case "candidate":
				candidate++
			case "disabled":
				disabled++
			case "conflicted":
				conflicted++
			}
		}

		// Hit-rate summary from the dedicated stats endpoint (SHADOW-039).
		// Replaces the former placeholder hits_total/active estimate.
		hrResp, err := http.Get("http://localhost:7878/api/stats/hit-rate")
		if err != nil {
			return fmt.Errorf("fetch hit-rate: %w", err)
		}
		var hr struct {
			ActiveRules  int            `json:"active_rules"`
			HitRatePct   int            `json:"hit_rate_pct"`
			HitsThisWeek int            `json:"hits_this_week"`
			HitsLastWeek int            `json:"hits_last_week"`
			LowHitCount  int            `json:"low_hit_count"`
			Trend        string         `json:"trend"`
			LastHit      map[string]any `json:"last_hit"`
		}
		if err := json.NewDecoder(hrResp.Body).Decode(&hr); err != nil {
			return fmt.Errorf("parse hit-rate: %w", err)
		}
		hrResp.Body.Close()

		fmt.Println()
		fmt.Println(boldStyle.Render("  👻 Shadow Memory Layer Health"))
		fmt.Println()
		fmt.Printf("  %s  %d total rules\n", dimStyle.Render("Total:"), len(rules))
		fmt.Printf("  %s  %d active  %s  %d candidate  %s  %d disabled  %s  %d conflicted\n",
			greenStyle.Render("●"), active,
			yellowStyle.Render("◐"), candidate,
			dimStyle.Render("○"), disabled,
			redStyle.Render("✗"), conflicted)
		fmt.Println()

		// Hit rate + trend (Type-A proxy metric, SHADOW-041).
		trendGlyph := map[string]string{"up": "↑", "down": "↓", "equal": "="}
		glyph := trendGlyph[hr.Trend]
		if glyph == "" {
			glyph = "·"
		}
		rateStyle := greenStyle
		if hr.HitRatePct < 30 {
			rateStyle = yellowStyle
		}
		fmt.Printf("  %s  Hit rate: %d%% %s  (this week)\n",
			rateStyle.Render("♦"), hr.HitRatePct, rateStyle.Render(glyph))

		fmt.Printf("  %s  %d active · %d low-hit", dimStyle.Render("•"), hr.ActiveRules, hr.LowHitCount)
		if hr.LastHit != nil {
			agent, _ := hr.LastHit["agent_name"].(string)
			content, _ := hr.LastHit["content"].(string)
			if content == "" {
				content, _ = hr.LastHit["rule_id"].(string)
			}
			if len(content) > 48 {
				content = content[:47] + "…"
			}
			if content != "" {
				fmt.Printf(" · last hit: %s · %q", agent, content)
			}
		}
		fmt.Println()
		fmt.Printf("  %s  trend: %s · %d hits this week vs %d last week\n",
			dimStyle.Render("•"), hr.Trend, hr.HitsThisWeek, hr.HitsLastWeek)

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
	ID         string  `json:"id"`
	Content    string  `json:"content"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Scope      string  `json:"scope"`
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

// --- sync command: explicit adapter sync / dry-run preview ---

type syncOptions struct {
	homeDir string
	dbPath  string
	dryRun  bool
	out     io.Writer
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync active rules into agent context files",
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		home, _ := os.UserHomeDir()
		return runSync(syncOptions{
			homeDir: filepath.Join(home, ".shadow"),
			dbPath:  storage.DefaultDBPath(),
			dryRun:  dryRun,
			out:     cmd.OutOrStdout(),
		})
	},
}

func runSync(opts syncOptions) error {
	if opts.out == nil {
		opts.out = os.Stdout
	}
	if opts.dbPath == "" {
		opts.dbPath = storage.DefaultDBPath()
	}
	if opts.homeDir == "" {
		home, _ := os.UserHomeDir()
		opts.homeDir = filepath.Join(home, ".shadow")
	}

	db, err := storage.Open(opts.dbPath)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer db.Close()

	cfgMgr := config.NewManager(opts.homeDir)
	if err := cfgMgr.LoadGlobal(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg := cfgMgr.Get()

	ruleRepo := storage.NewRuleRepo(db)
	projectRepo := storage.NewProjectRepo(db)
	globalRules, err := ruleRepo.List(storage.RuleFilter{Status: "active", Scope: "global"})
	if err != nil {
		return fmt.Errorf("list global rules: %w", err)
	}
	projects, err := projectRepo.List()
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}
	projectRulesByPath, err := ruleRepo.ActiveProjectRulesByPath()
	if err != nil {
		return fmt.Errorf("list project rules: %w", err)
	}

	backupDir := filepath.Join(opts.homeDir, "backups")
	adapters := []adapter.Adapter{
		adapter.NewClaudeCodeAdapter(backupDir),
		adapter.NewCursorAdapter(backupDir),
		adapter.NewCodexAdapter(backupDir),
		adapter.NewOpenClawAdapter(backupDir),
		adapter.NewCopilotAdapter(backupDir),
	}

	prefix := "SYNC"
	if opts.dryRun {
		prefix = "DRY-RUN"
	}
	changes := 0

	for _, a := range adapters {
		if !config.AdapterEnabled(cfg, a.Name()) {
			continue
		}
		if len(globalRules) > 0 {
			changed, err := syncAdapterTarget(a, globalRules, "global", "", opts.dryRun)
			if err != nil {
				return err
			}
			if changed {
				changes++
			}
			fmt.Fprintf(opts.out, "%s %s global -> %s (%d rule(s))\n",
				prefix, a.Name(), a.TargetPath("global", ""), len(globalRules))
		}

		for _, p := range projects {
			if !projectIncludesAgent(p, a.Name()) {
				continue
			}
			projectRules := projectRulesByPath[p.Path]
			if len(projectRules) == 0 {
				continue
			}
			changed, err := syncAdapterTarget(a, projectRules, "project", p.Path, opts.dryRun)
			if err != nil {
				return err
			}
			if changed {
				changes++
			}
			fmt.Fprintf(opts.out, "%s %s project -> %s (%d rule(s))\n",
				prefix, a.Name(), a.TargetPath("project", p.Path), len(projectRules))
		}
	}

	if changes == 0 {
		fmt.Fprintf(opts.out, "%s no changes\n", prefix)
	}
	return nil
}

func syncAdapterTarget(a adapter.Adapter, rules []*storage.Rule, scope, projectPath string, dryRun bool) (bool, error) {
	if dryRun {
		result, err := a.PreviewRules(rules, scope, projectPath)
		if err != nil {
			return false, fmt.Errorf("preview %s %s: %w", a.Name(), scope, err)
		}
		return result.Changed, nil
	}
	if err := a.WriteRules(rules, scope, projectPath); err != nil {
		return false, fmt.Errorf("write %s %s: %w", a.Name(), scope, err)
	}
	return true, nil
}

func projectIncludesAgent(p *storage.Project, adapterName string) bool {
	if p == nil || len(p.Agents) == 0 {
		return true
	}
	for _, agentName := range p.Agents {
		if normalizeAgentName(agentName) == adapterName {
			return true
		}
	}
	return false
}

func normalizeAgentName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, "-", "_")
	n = strings.ReplaceAll(n, " ", "_")
	switch n {
	case "claude", "claude_code":
		return "claude_code"
	case "github_copilot":
		return "copilot"
	default:
		return n
	}
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

		uninstallDaemon()

		if cleanBlocks {
			fmt.Println("Removing managed blocks from agent context files...")
			backupDir := filepath.Join(home, ".shadow", "backups")
			adapters := []adapter.Adapter{
				adapter.NewClaudeCodeAdapter(backupDir),
				adapter.NewCursorAdapter(backupDir),
				adapter.NewCodexAdapter(backupDir),
				adapter.NewOpenClawAdapter(backupDir),
				adapter.NewCopilotAdapter(backupDir),
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

	contextFiles := []string{"CLAUDE.md", ".cursorrules", "AGENTS.md", "OPENCLAW.md", ".github/copilot-instructions.md"}

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
		if runtime.GOOS == "windows" {
			return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		}
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

// --- shadow task command: memory-assembled task injection ---
//
// Wired to the real M8 engine (internal/daemon.TaskCommand), which runs the
// ContextEngine (tag/scope/recency ranking) and writes the selected rules into
// the target agent's context file via the adapter layer — same in-process
// pattern uninstallCmd uses for adapter file operations. Reads the SQLite store
// directly; WAL (storage.Open) makes this safe alongside a running daemon.

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

		// Open the memory store in-process and run the real extract + inject.
		db, err := storage.Open(storage.DefaultDBPath())
		if err != nil {
			return fmt.Errorf("open memory store: %w (run 'shadow start' first)", err)
		}
		defer db.Close()

		tc := daemon.NewTaskCommand(db, storage.NewRuleRepo(db))

		result, err := tc.RunTask(taskDesc, agentName, projectPath)
		if err != nil {
			return fmt.Errorf("extract context: %w", err)
		}
		rules := result.ExtractedContext.Rules

		if len(rules) == 0 {
			fmt.Printf("\n  %s No matching rules found for: %s\n\n",
				dimStyle.Render("ℹ"), taskDesc)
			fmt.Printf("  %s\n", dimStyle.Render("Shadow will remember your corrections as you code."))
			fmt.Printf("  %s\n", dimStyle.Render("Run 'shadow store' to manually save a rule."))
			return nil
		}

		// Preview & confirm via multi-select TUI.
		selected := runTaskPreview(rules, taskDesc, agentName)
		if len(selected) == 0 {
			fmt.Printf("\n  %s Injection cancelled — no rules selected.\n\n",
				dimStyle.Render("ℹ"))
			return nil
		}

		// Inject the selected rules into the target agent's context file.
		if err := tc.InjectIntoAgent(agentName, selected, projectPath); err != nil {
			return fmt.Errorf("inject into %s: %w", agentName, err)
		}

		// Record a "hit" per injected rule (SHADOW-041): confirming injection is
		// itself a usage signal — it refreshes decay_score and feeds the hit-rate
		// metric. Best-effort via the daemon HTTP API; non-fatal if daemon is down.
		for _, r := range selected {
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
		fmt.Printf("  %s %d rule(s) from your memory", dimStyle.Render("•"), len(selected))
		if result.ExtractedContext.TotalFound > len(rules) {
			fmt.Printf(" · %d more matched", result.ExtractedContext.TotalFound-len(rules))
		}
		fmt.Println()
		return nil
	},
}

// runTaskPreview runs the multi-select TUI over the candidate rules and returns
// the rules the user confirmed for injection. Returns nil if cancelled.
func runTaskPreview(rules []*storage.Rule, taskDesc, agentName string) []*storage.Rule {
	model := taskModel{
		items:    rules,
		selected: make(map[int]bool),
		task:     taskDesc,
		agent:    agentName,
	}
	// Default-select all candidates — the engine already curated the top
	// MaxRules, so the common path is "confirm to inject all".
	for i := range rules {
		model.selected[i] = true
	}

	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return nil
	}

	tm, ok := finalModel.(taskModel)
	if !ok || tm.cancelled {
		return nil
	}

	var picked []*storage.Rule
	for i, r := range tm.items {
		if tm.selected[i] {
			picked = append(picked, r)
		}
	}
	return picked
}

// --- task preview TUI: multi-select + confirm inject (SHADOW-036) ---

type taskModel struct {
	items     []*storage.Rule
	cursor    int
	selected  map[int]bool
	task      string
	agent     string
	cancelled bool
}

func (m taskModel) Init() tea.Cmd { return nil }

func (m taskModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case " ":
			// Toggle current selection.
			if len(m.items) > 0 {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		case "a":
			for i := range m.items {
				m.selected[i] = true
			}
		case "n":
			for i := range m.items {
				m.selected[i] = false
			}
		case "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m taskModel) View() string {
	var b strings.Builder
	b.WriteString(boldStyle.Render("  📋 Shadow Task Preview") + "\n")
	b.WriteString(fmt.Sprintf("  %s %s\n", dimStyle.Render("Task:"), truncateStr(m.task, 64)))
	b.WriteString(fmt.Sprintf("  %s %s\n\n", dimStyle.Render("Target:"), m.agent))
	b.WriteString(dimStyle.Render("  space=toggle  a=all  n=none  ↑↓=move  enter=inject  q=cancel") + "\n\n")

	for i, r := range m.items {
		cursor := " "
		if i == m.cursor {
			cursor = yellowStyle.Render("▸")
		}
		mark := dimStyle.Render("▫")
		if m.selected[i] {
			mark = greenStyle.Render("✓")
		}
		conf := r.DecayScore
		if conf == 0 {
			conf = r.Confidence
		}
		meta := dimStyle.Render(fmt.Sprintf("%3.0f%% · %s", conf*100, r.Scope))
		if len(r.Tags) > 0 {
			meta += dimStyle.Render(fmt.Sprintf(" · #%s", strings.Join(r.Tags, " #")))
		}
		b.WriteString(fmt.Sprintf("  %s [%s] %s  %s\n", cursor, mark, meta, r.Content))
	}

	selected := 0
	for _, v := range m.selected {
		if v {
			selected++
		}
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d of %d selected · enter to inject into %s",
		selected, len(m.items), m.agent)))
	b.WriteString("\n")
	return b.String()
}

// truncateStr clips s to n runes, appending an ellipsis if truncated.
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
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
			"content":    content,
			"scope":      scope,
			"status":     "candidate",
			"tags":       tags,
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
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(storeCmd)
	rootCmd.AddCommand(storeMemoryCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(mcpCmd)
	syncCmd.Flags().Bool("dry-run", false, "Preview adapter writes without changing files")
	uninstallCmd.Flags().Bool("clean-blocks", false, "Remove managed blocks from agent context files")
	taskCmd.Flags().String("agent", "", "Target agent (claude-code, cursor, codex, openclaw, copilot)")
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
