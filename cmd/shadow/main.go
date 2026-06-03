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

		// Port conflict retry: probe 7878 (and a few neighbors) for a
		// free port. If the default is busy (e.g. another tool grabbed
		// it), walk forward up to 5 ports. The chosen port is then
		// passed to the daemon via the SHADOW_PORT env var so the
		// child process binds the same port we probed.
		port, err := daemon.TryPorts(7878, 5)
		if err != nil {
			return fmt.Errorf("port probe: %w", err)
		}
		if port != 7878 {
			fmt.Printf("⚠ Port 7878 is in use, falling back to %d\n", port)
			fmt.Println("  To free 7878, run: lsof -ti:7878 | xargs kill -9")
		}
		// Persist for the launchd plist (the daemon reads SHADOW_PORT
		// from its environment at startup).
		if err := os.Setenv("SHADOW_PORT", fmt.Sprintf("%d", port)); err != nil {
			return fmt.Errorf("set SHADOW_PORT: %w", err)
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
			// Permission issues (writing under ~/Library/LaunchAgents
			// shouldn't require sudo, but we surface a friendly hint
			// just in case the user is on a locked-down profile).
			if os.IsPermission(err) {
				fmt.Println("⚠ Permission denied writing the launchd plist.")
				fmt.Println("  Try: sudo launchctl load -w ~/Library/LaunchAgents/com.shadow.daemon.plist")
			}
			return fmt.Errorf("install launchd: %w", err)
		}
		fmt.Println("✓ Shadow daemon registered with launchd")

		// Start via launchctl.
		fmt.Println("Starting daemon...")
		if err := daemon.LoadLaunchd("com.shadow.daemon"); err != nil {
			// launchctl commonly returns a permission error if the
			// user session is missing a security context.
			if strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "not privileged") {
				fmt.Println("⚠ launchctl could not start the daemon.")
				fmt.Println("  Try: sudo launchctl load -w ~/Library/LaunchAgents/com.shadow.daemon.plist")
			}
			return fmt.Errorf("start daemon: %w", err)
		}
		fmt.Printf("✓ Shadow daemon started (http://localhost:%d)\n", port)
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

var (
	openURL          = "http://localhost:7878"
	openHTTPTimeout  = 5 * time.Second
	openPollInterval = 200 * time.Millisecond
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the Shadow web console in browser",
	Long: `Open the Shadow web console in your default browser.

If the daemon is not running, this command exits with an error
instructing you to run 'shadow start' first. If the daemon is
starting up (IPC socket up but HTTP server not yet bound), it polls
briefly before giving up.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		if !client.IsRunning() {
			return fmt.Errorf("Shadow daemon is not running.\n  Run 'shadow start' first, then 'shadow open'.")
		}

		if !waitForHTTP(openURL+"/api/dashboard", openHTTPTimeout) {
			return fmt.Errorf("daemon is running (IPC up) but HTTP server didn't become ready in %s.\n  Try 'shadow status' or 'shadow restart'.", openHTTPTimeout)
		}

		fmt.Printf("Opening Shadow console at %s\n", openURL)
		return exec.Command("open", openURL).Start()
	},
}

// waitForHTTP polls `url` until it returns HTTP 200, or `timeout`
// elapses. Returns true on 200, false on timeout. Used by `shadow
// open` to bridge the small window between IPC-socket-ready and
// HTTP-listener-ready during daemon startup.
func waitForHTTP(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(openPollInterval)
	}
	return false
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(openCmd)
	uninstallCmd.Flags().Bool("clean-blocks", false, "Remove managed blocks from agent context files")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
