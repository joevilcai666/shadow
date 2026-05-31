package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbletea"
)

// OnboardingModel is the 4-step onboarding TUI.
type OnboardingModel struct {
	step      int
	version   string
	homeDir   string
	agents    CheckboxList
	progress  StepProgress
	spinner   SpinnerModel
	loading   bool
	loadingMsg string
	done      bool
	err       error

	// Results from each step.
	daemonRunning bool
	agentsFound   []string
	rulesGenerated int
}

// NewOnboardingModel creates the onboarding TUI model.
func NewOnboardingModel(version string) OnboardingModel {
	home, _ := os.UserHomeDir()
	return OnboardingModel{
		step:    1,
		version: version,
		homeDir: home,
		progress: StepProgress{
			Total:  4,
			Labels: []string{"Daemon Setup", "Privacy & Scope", "Agent Detection", "Initial Memory"},
		},
		agents: NewCheckboxList([]CheckboxItem{
			{Label: "Claude Code", Description: "writes to CLAUDE.md"},
			{Label: "Cursor", Description: "writes to .cursorrules"},
			{Label: "GitHub Copilot", Description: "writes to .github/copilot-instructions.md"},
		}),
	}
}

// Init initializes the onboarding.
func (m OnboardingModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m OnboardingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
		case "up", "k", "down", "j", " ":
			if m.step == 2 {
				m.agents = m.agents.Update(msg)
				return m, nil
			}
		}
	case daemonCheckMsg:
		m.daemonRunning = msg.running
		m.loading = false
		if msg.running {
			m.step = 4 // Skip to step 4 if already running.
		} else {
			m.step = 2
		}
		return m, nil
	case agentDetectMsg:
		m.agentsFound = msg.agents
		m.loading = false
		m.step = 4
		return m, nil
	case scanCompleteMsg:
		m.rulesGenerated = msg.count
		m.loading = false
		m.done = true
		return m, nil
	case errorMsg:
		m.err = msg.err
		m.loading = false
		return m, nil
	}

	if m.loading {
		model, cmd := m.spinner.Update(msg)
		if s, ok := model.(SpinnerModel); ok {
			m.spinner = s
		}
		return m, cmd
	}

	return m, nil
}

func (m OnboardingModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case 1:
		m.loading = true
		m.loadingMsg = "Checking daemon status..."
		m.spinner = NewSpinner(m.loadingMsg)
		return m, tea.Batch(m.spinner.Init(), checkDaemon())
	case 2:
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
		return m, tea.Batch(m.spinner.Init(), scanProject())
	case 4:
		if m.done {
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the onboarding TUI.
func (m OnboardingModel) View() string {
	var b strings.Builder

	b.WriteString(BrandHeader(m.version))
	b.WriteString("\n")

	m.progress.Current = m.step
	b.WriteString(m.progress.View())
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
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
		b.WriteString(dimStyle.Render("Press Enter to begin..."))

	case 2:
		b.WriteString(boldStyle.Render("Privacy & Scope"))
		b.WriteString("\n\n")
		b.WriteString("Shadow will:\n")
		b.WriteString("  ✓ Read project code & agent session logs\n")
		b.WriteString("  ✓ Write managed rules to agent context files\n")
		b.WriteString("  ✓ Never store keys, tokens, or credentials\n")
		b.WriteString("  ✓ Store only distilled rules, not raw conversations\n\n")
		b.WriteString(dimStyle.Render("Press Enter to accept (safe defaults)..."))

	case 3:
		b.WriteString(boldStyle.Render("Select Agents"))
		b.WriteString("\n\n")
		b.WriteString(m.agents.View())
		b.WriteString(dimStyle.Render("↑↓ move · Space toggle · Enter confirm"))

	case 4:
		if m.done {
			b.WriteString(successStyle.Render("✓ Shadow is ready!"))
			b.WriteString("\n\n")

			if len(m.agentsFound) > 0 {
				b.WriteString(fmt.Sprintf("  Agents connected: %s\n", strings.Join(m.agentsFound, ", ")))
			}
			if m.rulesGenerated > 0 {
				b.WriteString(fmt.Sprintf("  Initial memories: %d candidate rules generated\n", m.rulesGenerated))
			}

			b.WriteString("\n")
			b.WriteString(dimStyle.Render("Next steps:"))
			b.WriteString("\n")
			b.WriteString("  shadow status  — check everything is running\n")
			b.WriteString("  shadow open    — open web console at localhost:7878\n")
			b.WriteString("  shadow review  — review candidate rules\n\n")
			b.WriteString(dimStyle.Render("Press Enter to finish."))
		} else {
			b.WriteString(boldStyle.Render("Initial Memory Generation"))
			b.WriteString("\n\n")
			b.WriteString("Shadow will scan your project for:\n")
			b.WriteString("  • Package manager (lockfile detection)\n")
			b.WriteString("  • Test framework (config detection)\n")
			b.WriteString("  • Existing rules (CLAUDE.md, .cursorrules)\n")
			b.WriteString("  • Directory conventions\n\n")
			b.WriteString(dimStyle.Render("Press Enter to scan..."))
		}
	}

	return b.String()
}

// --- Tea commands ---

type daemonCheckMsg struct{ running bool }
type agentDetectMsg struct{ agents []string }
type scanCompleteMsg struct{ count int }
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
		}

		// Detect GitHub Copilot.
		if _, err := exec.LookPath("gh"); err == nil {
			agents = append(agents, "GitHub Copilot")
		}

		if len(agents) == 0 {
			agents = []string{"(none detected)"}
		}

		return agentDetectMsg{agents: agents}
	}
}

func scanProject() tea.Cmd {
	return func() tea.Msg {
		count := 0
		// Quick scan for lockfiles and config files.
		files := []string{
			"package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lockb",
			"go.mod", "Cargo.toml", "pyproject.toml",
			"CLAUDE.md", ".cursorrules", "AGENTS.md",
		}
		for _, f := range files {
			if _, err := os.Stat(f); err == nil {
				count++
			}
		}
		return scanCompleteMsg{count: count}
	}
}
