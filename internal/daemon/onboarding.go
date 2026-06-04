package daemon

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/joevilcai666/shadow/internal/capture"
	"github.com/joevilcai666/shadow/internal/daemon/components"
	"github.com/joevilcai666/shadow/internal/storage"
)

// OnboardingModel is the 4-step onboarding TUI.
type OnboardingModel struct {
	step       int
	version    string
	homeDir    string
	cwd        string
	agents     CheckboxList
	banner     components.Banner
	spinner    SpinnerModel
	loading    bool
	loadingMsg string
	done       bool
	err        error

	// Privacy step
	privacyAccepted bool

	// Results from each step.
	daemonRunning  bool
	agentsFound    []string
	agentTargets   map[string]string // agent name -> write target
	rulesGenerated int
	importedFiles  []string
	scanFacts      []string

	// Database path for rule persistence.
	dbPath string
}

// NewOnboardingModel creates the onboarding TUI model.
func NewOnboardingModel(version string) OnboardingModel {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	return OnboardingModel{
		step:    1,
		version: version,
		homeDir: home,
		cwd:     cwd,
		banner:  components.NewBanner(),
		agents: NewCheckboxList([]CheckboxItem{
			{Label: "Claude Code", Description: "→ writes to CLAUDE.md"},
			{Label: "Cursor", Description: "→ writes to .cursorrules"},
			{Label: "GitHub Copilot", Description: "→ writes to .github/copilot-instructions.md"},
			{Label: "Codex", Description: "→ writes to AGENTS.md"},
		}),
		agentTargets: map[string]string{
			"Claude Code":    "CLAUDE.md (project) + ~/.claude/CLAUDE.md (global)",
			"Cursor":         ".cursorrules (project) + ~/.cursorrules (global)",
			"GitHub Copilot": ".github/copilot-instructions.md (project)",
			"Codex":          "AGENTS.md (project) + ~/AGENTS.md (global)",
		},
		privacyAccepted: true,
		dbPath:          filepath.Join(home, ".shadow", "shadow.db"),
	}
}

// Init initializes the onboarding.
func (m OnboardingModel) Init() tea.Cmd {
	return m.banner.Init()
}

// Update handles messages.
func (m OnboardingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Drive banner animation regardless of step.
	var bannerCmd tea.Cmd
	m.banner, bannerCmd = m.banner.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
		case "up", "k", "down", "j", " ":
			if m.step == 3 && !m.loading {
				m.agents = m.agents.Update(msg)
				return m, bannerCmd
			}
		case "a":
			if m.step == 2 && !m.loading {
				// Advanced config placeholder
			}
		case "s":
			if m.step == 4 && !m.loading && !m.done {
				return m, tea.Quit
			}
		}
	case daemonCheckMsg:
		m.daemonRunning = msg.running
		m.loading = false
		if msg.running {
			m.step = 4
		} else {
			m.step = 2
		}
		return m, bannerCmd
	case agentDetectMsg:
		m.agentsFound = msg.agents
		m.loading = false
		if len(msg.agents) > 0 {
			for i, item := range m.agents.Items {
				found := false
				for _, a := range msg.agents {
					if a == item.Label {
						found = true
						break
					}
				}
				if !found {
					m.agents.Selected[i] = false
				}
			}
		}
		m.step = 4
		return m, bannerCmd
	case scanCompleteMsg:
		m.rulesGenerated = msg.count
		m.importedFiles = msg.importedFiles
		m.scanFacts = msg.facts
		m.loading = false
		m.done = true
		return m, bannerCmd
	case errorMsg:
		m.err = msg.err
		m.loading = false
		return m, bannerCmd
	}

	if m.loading {
		model, spinnerCmd := m.spinner.Update(msg)
		if s, ok := model.(SpinnerModel); ok {
			m.spinner = s
		}
		return m, tea.Batch(bannerCmd, spinnerCmd)
	}

	return m, bannerCmd
}

func (m OnboardingModel) handleEnter() (tea.Model, tea.Cmd) {
	// If error, retry current step
	if m.err != nil {
		m.err = nil
		return m.handleEnter()
	}

	switch m.step {
	case 1:
		m.loading = true
		m.loadingMsg = "Registering daemon..."
		m.spinner = NewSpinner(m.loadingMsg)
		return m, tea.Batch(m.spinner.Init(), checkDaemon())
	case 2:
		m.privacyAccepted = true
		m.step = 3
		m.loading = true
		m.loadingMsg = "Detecting installed agents..."
		m.spinner = NewSpinner(m.loadingMsg)
		return m, tea.Batch(m.spinner.Init(), detectAgents())
	case 3:
		m.step = 4
		m.loading = true
		m.loadingMsg = "Scanning for initial memories..."
		m.spinner = NewSpinner(m.loadingMsg)
		return m, tea.Batch(m.spinner.Init(), scanProject(m.cwd, m.dbPath, m.agents.SelectedItems(), m.agentsFound))
	case 4:
		if m.done {
			_ = exec.Command("open", "http://localhost:7878").Start()
			return m, tea.Quit
		}
		// Start scan when not done yet.
		m.loading = true
		m.loadingMsg = "Scanning for initial memories..."
		m.spinner = NewSpinner(m.loadingMsg)
		return m, tea.Batch(m.spinner.Init(), scanProject(m.cwd, m.dbPath, m.agents.SelectedItems(), m.agentsFound))
	}
	return m, nil
}

// View renders the onboarding TUI.
func (m OnboardingModel) View() string {
	var b strings.Builder

	b.WriteString(m.banner.BannerView())
	b.WriteString("\n")

	pipeline := components.NewPipeline(
		[]string{"Setup", "Privacy", "Agents", "Memory"},
		m.step-1,
	)
	b.WriteString(pipeline.View())
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("✗ Error: %v", m.err)))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("Press Enter to retry or q to quit."))
		return b.String()
	}

	if m.loading {
		b.WriteString(m.spinner.View())
		return b.String()
	}

	switch m.step {
	case 1:
		b.WriteString(boldStyle.Render("Welcome to Shadow!"))
		b.WriteString("\n\n")
		b.WriteString("This will take about 60 seconds. Everything stays local.\n")
		b.WriteString("You can skip, go back, or quit at any time.\n\n")
		b.WriteString("Your data:\n")
		b.WriteString("  ✓ Stored only on this machine\n")
		b.WriteString("  ✓ Never uploaded without your consent\n")
		b.WriteString("  ✓ Keys/tokens automatically blocked\n\n")
		b.WriteString(dimStyle.Render("Press Enter to begin..."))

	case 2:
		b.WriteString(boldStyle.Render("Privacy & Scope"))
		b.WriteString("\n\n")
		b.WriteString("Shadow will:\n")
		b.WriteString("  " + successStyle.Render("✓") + " Read project code & agent session logs\n")
		b.WriteString("  " + successStyle.Render("✓") + " Write managed rules to agent context files\n")
		b.WriteString("  " + successStyle.Render("✓") + " Never store keys, tokens, or credentials\n")
		b.WriteString("  " + successStyle.Render("✓") + " Store only distilled rules, not raw conversations\n\n")
		b.WriteString("🔒 Default protections (always on):\n")
		b.WriteString("  • Block: API keys, tokens, .env files\n")
		b.WriteString("  • Exclude: node_modules, .git, dist, .env*\n")
		b.WriteString("  • Only distilled rules stored, never raw code\n\n")
		b.WriteString(dimStyle.Render("Press Enter to accept safe defaults..."))

	case 3:
		b.WriteString(boldStyle.Render("Select Agents"))
		b.WriteString("\n\n")
		if len(m.agentsFound) > 0 {
			b.WriteString(dimStyle.Render(fmt.Sprintf("Detected %d agent(s) on your machine:\n\n", len(m.agentsFound))))
		} else {
			b.WriteString(dimStyle.Render("No agents auto-detected. Select manually:\n\n"))
		}
		b.WriteString(m.agents.View())
		b.WriteString(dimStyle.Render("↑↓ move · Space toggle · Enter confirm"))

	case 4:
		if m.done {
			b.WriteString(successStyle.Render("✓ Shadow is ready!"))
			b.WriteString("\n\n")

			if len(m.agentsFound) > 0 {
				b.WriteString(fmt.Sprintf("  Agents connected: %s\n", strings.Join(m.agentsFound, ", ")))
			}
			b.WriteString(fmt.Sprintf("  Initial memories: %d candidate rules generated\n", m.rulesGenerated))
			if len(m.importedFiles) > 0 {
				b.WriteString(fmt.Sprintf("  Imported from: %s\n", strings.Join(m.importedFiles, ", ")))
			}
			if len(m.scanFacts) > 0 {
				b.WriteString("\n  " + dimStyle.Render("Discovered:") + "\n")
				for _, fact := range m.scanFacts {
					if len(fact) > 60 {
						fact = fact[:60] + "..."
					}
					b.WriteString("    • " + dimStyle.Render(fact) + "\n")
				}
			}

			b.WriteString("\n")
			b.WriteString(dimStyle.Render("Next steps:"))
			b.WriteString("\n")
			b.WriteString("  shadow status  — check everything is running\n")
			b.WriteString("  shadow open    — open web console at localhost:7878\n")
			b.WriteString("  shadow review  — review candidate rules\n\n")
			b.WriteString(dimStyle.Render("Press Enter to open web console (or q to stay in terminal)..."))
		} else {
			b.WriteString(boldStyle.Render("Initial Memory Generation"))
			b.WriteString("\n\n")
			b.WriteString("Shadow will scan your project for:\n")
			b.WriteString("  • Package manager (lockfile detection)\n")
			b.WriteString("  • Test framework (config detection)\n")
			b.WriteString("  • Existing rules (CLAUDE.md, .cursorrules, AGENTS.md)\n")
			b.WriteString("  • Language and framework detection\n\n")
			b.WriteString(dimStyle.Render("Press Enter to scan...  (s to skip)"))
		}
	}

	return b.String()
}

// --- Tea commands ---

type daemonCheckMsg struct{ running bool }
type agentDetectMsg struct{ agents []string }
type scanCompleteMsg struct {
	count         int
	importedFiles []string
	facts         []string
}
type errorMsg struct{ err error }

func checkDaemon() tea.Cmd {
	return func() tea.Msg {
		client := NewClient()
		return daemonCheckMsg{running: client.IsRunning()}
	}
}

func detectAgents() tea.Cmd {
	return func() tea.Msg {
		var agents []string

		// Detect Claude Code.
		home, _ := os.UserHomeDir()
		if _, err := os.Stat(filepath.Join(home, ".claude")); err == nil {
			agents = append(agents, "Claude Code")
		}

		// Detect Cursor.
		if runtime.GOOS == "darwin" {
			if _, err := os.Stat("/Applications/Cursor.app"); err == nil {
				agents = append(agents, "Cursor")
			}
		} else if runtime.GOOS == "linux" {
			if _, err := os.Stat(filepath.Join(home, ".cursor")); err == nil {
				agents = append(agents, "Cursor")
			}
		}

		// Detect GitHub Copilot.
		if _, err := exec.LookPath("gh"); err == nil {
			// Check for copilot extension
			if _, err := exec.LookPath("github-copilot-cli"); err == nil {
				agents = append(agents, "GitHub Copilot")
			} else if _, err := os.Stat(filepath.Join(home, ".config", "gh")); err == nil {
				agents = append(agents, "GitHub Copilot")
			}
		}

		// Detect Codex.
		if _, err := exec.LookPath("codex"); err == nil {
			agents = append(agents, "Codex")
		} else if _, err := os.Stat(filepath.Join(home, ".codex")); err == nil {
			agents = append(agents, "Codex")
		}

		return agentDetectMsg{agents: agents}
	}
}

func scanProject(cwd, dbPath string, selectedAgents []CheckboxItem, detectedAgents []string) tea.Cmd {
	return func() tea.Msg {
		var facts []string
		var importedFiles []string
		totalRules := 0

		// Open database for writing rules.
		db, err := storage.Open(dbPath)
		if err != nil {
			slog.Warn("onboarding: cannot open database", "error", err)
			return scanCompleteMsg{count: countScanFiles(cwd), facts: facts}
		}
		defer db.Close()

		ruleRepo := storage.NewRuleRepo(db)

		// Step 1: Scan project for conventions.
		scanner := capture.NewScanner(cwd)
		scanResult, err := scanner.Scan()
		if err != nil {
			slog.Warn("onboarding: scan failed", "error", err)
		} else {
			facts = append(facts, scanResult.Facts...)

			// Convert scan results to candidate rules.
			rules := scanResult.ToRules()
			for _, rule := range rules {
				if err := ruleRepo.Create(rule); err != nil {
					slog.Warn("onboarding: create rule from scan", "error", err)
				} else {
					totalRules++
				}
			}
		}

		// Step 2: Import existing rule files.
		importer := capture.NewImporter()
		ruleFiles := []string{
			"CLAUDE.md",
			".claude/CLAUDE.md",
			".cursorrules",
			".cursor/rules",
			"AGENTS.md",
			".github/copilot-instructions.md",
		}

		for _, f := range ruleFiles {
			path := filepath.Join(cwd, f)
			if _, err := os.Stat(path); err != nil {
				continue
			}
			rules, err := importer.ImportFile(path)
			if err != nil {
				slog.Warn("onboarding: import file", "file", path, "error", err)
				continue
			}
			for _, rule := range rules {
				// Tag imported rules with their source file
				rule.Tags = append(rule.Tags, "import:"+f)
				if err := ruleRepo.Create(rule); err != nil {
					slog.Warn("onboarding: create imported rule", "error", err)
				} else {
					totalRules++
				}
			}
			importedFiles = append(importedFiles, f)
			facts = append(facts, fmt.Sprintf("Imported rules from %s (%d rules)", f, len(rules)))
		}

		// Step 3: Register the project.
		projectRepo := storage.NewProjectRepo(db)
		projectName := filepath.Base(cwd)
		project := &storage.Project{
			ID:        storage.NewID(),
			Path:      cwd,
			Name:      projectName,
			Agents:    agentNames(detectedAgents),
			CreatedAt: storage.Now(),
		}
		_ = projectRepo.Create(project)

		return scanCompleteMsg{
			count:         totalRules,
			importedFiles: importedFiles,
			facts:         facts,
		}
	}
}

func countScanFiles(cwd string) int {
	count := 0
	files := []string{
		"package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lockb",
		"go.mod", "Cargo.toml", "pyproject.toml",
		"CLAUDE.md", ".cursorrules", "AGENTS.md",
	}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(cwd, f)); err == nil {
			count++
		}
	}
	return count
}

func agentNames(selected []string) []string {
	if selected == nil {
		return []string{}
	}
	return selected
}
