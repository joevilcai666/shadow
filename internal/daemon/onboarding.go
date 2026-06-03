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
	"github.com/charmbracelet/lipgloss"
	"github.com/joevilcai666/shadow/internal/capture"
	"github.com/joevilcai666/shadow/internal/daemon/components"
	"github.com/joevilcai666/shadow/internal/storage"
)

// OnboardingModel is the 4-step onboarding TUI.
type OnboardingModel struct {
	step      int
	version   string
	homeDir   string
	cwd       string
	agents    CheckboxList
	progress  StepProgress
	spinner   SpinnerModel
	loading   bool
	loadingMsg string
	done      bool
	err       error

	// Privacy step
	privacyAccepted bool

	// Results from each step.
	daemonRunning bool
	agentsFound   []string
	agentTargets  map[string]string // agent name -> write target
	rulesGenerated int
	importedFiles  []string
	scanFacts      []string

	// Database path for rule persistence.
	dbPath string

	// Step 4 sub-state. 4a = scan display, 4b = questionnaire + apply.
	subStep scanSubStep

	// 2-question onboarding preferences. questionnaireIdx tracks the current
	// question (0/1); questionnaireAnswers holds the chosen option index
	// for each question. The component-level state in `questionnaire` is the
	// authoritative source during 4b; these fields mirror it for tests.
	questionnaireIdx     int
	questionnaireAnswers [2]int
	questionnaireDone    bool
	questionnaire        Questionnaire

	// Apply phase results.
	appliedFiles []string
	applyErrors  map[string]error
}

// scanSubStep tracks where we are within step 4.
type scanSubStep int

const (
	subStepScanIdle    scanSubStep = 0 // before scan starts (initial view)
	subStepScanRunning scanSubStep = 1 // scan in flight
	subStepScanDone    scanSubStep = 2 // scan finished, results visible
	subStepQuestion    scanSubStep = 3 // questionnaire visible
	subStepApplying    scanSubStep = 4 // apply in flight
	subStepApplied     scanSubStep = 5 // everything done; show summary
)

// NewOnboardingModel creates the onboarding TUI model.
func NewOnboardingModel(version string) OnboardingModel {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	return OnboardingModel{
		step:    1,
		version: version,
		homeDir: home,
		cwd:     cwd,
		progress: StepProgress{
			Total:  4,
			Labels: []string{"Daemon Setup", "Privacy & Scope", "Agent Detection", "Initial Memory"},
		},
		agents: NewCheckboxList([]CheckboxItem{
			{Label: "Claude Code", Description: "→ writes to CLAUDE.md"},
			{Label: "Cursor", Description: "→ writes to .cursorrules"},
			{Label: "GitHub Copilot", Description: "→ writes to .github/copilot-instructions.md"},
			{Label: "Codex", Description: "→ writes to AGENTS.md"},
		}),
		agentTargets: map[string]string{
			"Claude Code":      "CLAUDE.md (project) + ~/.claude/CLAUDE.md (global)",
			"Cursor":           ".cursorrules (project) + ~/.cursorrules (global)",
			"GitHub Copilot":   ".github/copilot-instructions.md (project)",
			"Codex":            "AGENTS.md (project) + ~/AGENTS.md (global)",
		},
		privacyAccepted:    true,
		dbPath:             filepath.Join(home, ".shadow", "shadow.db"),
		subStep:            subStepScanIdle,
		questionnaire:      NewQuestionnaire(),
		questionnaireIdx:   0,
		questionnaireAnswers: [2]int{0, 0},
		applyErrors:        make(map[string]error),
	}
}

// Init initializes the onboarding. It kicks off the daemon health check so
// that by the time the user presses Enter on step 1, we already know whether
// to install/load the daemon or skip straight to step 2 (privacy).
func (m OnboardingModel) Init() tea.Cmd {
	// Run the daemon check in the background; the result is delivered as a
	// daemonCheckMsg and Update() will route the user to the right step.
	return checkDaemon()
}

// Update handles messages.
func (m OnboardingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Ctrl+C and q always quit, even mid-flow.
		if k := msg.String(); k == "ctrl+c" || k == "q" {
			return m, tea.Quit
		}
		// 'b' goes back one step. We don't go below step 1 and we don't allow
		// back during loading.
		if msg.String() == "b" && !m.loading {
			if m.step > 1 {
				m.step--
				m.loading = false
				m.err = nil
				// Reset any per-step transient state.
				m.spinner = SpinnerModel{}
			}
			return m, nil
		}
		switch msg.String() {
		case "enter":
			return m.handleEnter()
		case "r":
			// Retry the current step. We only enable this when there
			// was an error; otherwise 'r' is a no-op so it doesn't
			// disrupt normal navigation.
			if m.err != nil {
				return m.handleEnter()
			}
		case "up", "k", "down", "j", " ":
			if m.step == 3 && !m.loading && m.subStep != subStepQuestion {
				m.agents = m.agents.Update(msg)
				return m, nil
			}
			// While the questionnaire is visible, hand the keys to it.
			if m.step == 4 && m.subStep == subStepQuestion && !m.loading {
				m.questionnaire = m.questionnaire.Update(msg)
				m.questionnaireIdx = m.questionnaire.Index()
				m.questionnaireAnswers = m.questionnaire.Answers()
				return m, nil
			}
		case "a":
			// Advanced config hint in step 2
			if m.step == 2 && !m.loading {
				// Could expand advanced config; for now just a no-op placeholder
			}
		case "s":
			// Skip shortcut — only meaningful on the questionnaire.
			if m.step == 4 && m.subStep == subStepQuestion && !m.loading {
				m.questionnaire = m.questionnaire.Update(msg)
				m.questionnaireIdx = m.questionnaire.Index()
				m.questionnaireAnswers = m.questionnaire.Answers()
				m.questionnaireDone = m.questionnaire.IsDone()
				return m, nil
			}
			// Legacy behavior: at the very end of step 4, "s" still quits.
			if m.step == 4 && m.subStep == subStepApplied && !m.loading {
				return m, tea.Quit
			}
		}
	case daemonCheckMsg:
		// State-machine fix: daemon is already running → don't skip past
		// privacy (step 2) or agent selection (step 3). Only step 1 (welcome)
		// is skippable when the daemon is up.
		m.daemonRunning = msg.running
		m.loading = false
		if msg.running {
			// Skip the welcome screen; user lands on privacy.
			m.step = 2
		} else {
			// Daemon not running yet — keep the user on the welcome screen
			// so they can press Enter to register + start it. The next
			// checkDaemon() will then advance to step 2.
			if m.step == 0 {
				m.step = 1
			}
		}
		return m, nil
	case agentDetectMsg:
		m.agentsFound = msg.agents
		m.loading = false
		// Auto-check detected agents
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
		m.subStep = subStepScanIdle
		return m, nil
	case scanCompleteMsg:
		m.rulesGenerated = msg.count
		m.importedFiles = msg.importedFiles
		m.scanFacts = msg.facts
		m.loading = false
		// We treat a "done" sub-step here as the scan-finished display
		// state, not the final done state. The final done state is
		// subStepApplied, reached after the user answers the
		// questionnaire and we run ApplyAgentsToAdapters.
		m.subStep = subStepScanDone
		m.done = false
		return m, nil
	case applyCompleteMsg:
		// Apply phase finished — show the final summary.
		m.appliedFiles = msg.appliedFiles
		m.applyErrors = msg.errs
		m.loading = false
		m.subStep = subStepApplied
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
	// If error, retry current step
	if m.err != nil {
		m.err = nil
		// Re-run the current step's action.
		return m.handleEnter()
	}

	switch m.step {
	case 1:
		// Welcome → kick off daemon check/install.
		m.loading = true
		m.loadingMsg = "Registering daemon..."
		m.spinner = NewSpinner(m.loadingMsg)
		return m, tea.Batch(m.spinner.Init(), checkDaemon())
	case 2:
		// Privacy accept → detect agents.
		m.privacyAccepted = true
		m.step = 3
		m.loading = true
		m.loadingMsg = "Detecting installed agents..."
		m.spinner = NewSpinner(m.loadingMsg)
		return m, tea.Batch(m.spinner.Init(), detectAgents())
	case 3:
		// Agent selection → start the scan.
		m.step = 4
		m.subStep = subStepScanRunning
		m.loading = true
		m.loadingMsg = "Scanning for initial memories..."
		m.spinner = NewSpinner(m.loadingMsg)
		return m, tea.Batch(m.spinner.Init(), scanProject(m.cwd, m.dbPath, m.agents.SelectedItems(), m.agentsFound))
	case 4:
		// Step 4 is a 3-phase mini-flow: scan → questionnaire → apply.
		switch m.subStep {
		case subStepScanIdle:
			// Begin the scan.
			m.subStep = subStepScanRunning
			m.loading = true
			m.loadingMsg = "Scanning for initial memories..."
			m.spinner = NewSpinner(m.loadingMsg)
			return m, tea.Batch(m.spinner.Init(), scanProject(m.cwd, m.dbPath, m.agents.SelectedItems(), m.agentsFound))
		case subStepScanDone:
			// Scan finished — advance to the questionnaire.
			m.subStep = subStepQuestion
			m.questionnaire = NewQuestionnaire()
			m.questionnaireIdx = m.questionnaire.Index()
			m.questionnaireAnswers = m.questionnaire.Answers()
			return m, nil
		case subStepQuestion:
			// Commit the current question's answer and advance.
			m.questionnaire = m.questionnaire.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m.questionnaireIdx = m.questionnaire.Index()
			m.questionnaireAnswers = m.questionnaire.Answers()
			m.questionnaireDone = m.questionnaire.IsDone()
			if m.questionnaire.IsDone() {
				// Move to the apply phase.
				return m.startApply()
			}
			return m, nil
		case subStepApplied:
			// Try to open browser and quit.
			return m, openBrowser()
		}
	}
	return m, nil
}

// startApply fires the apply command after the questionnaire is complete.
// It loads active rules from the database and writes them to the adapters
// backing each selected agent.
func (m OnboardingModel) startApply() (tea.Model, tea.Cmd) {
	m.subStep = subStepApplying
	m.loading = true
	m.loadingMsg = "Applying rules to agent context files..."
	m.spinner = NewSpinner(m.loadingMsg)
	selected := m.agents.SelectedItems()
	selectedNames := make([]string, 0, len(selected))
	for _, it := range selected {
		selectedNames = append(selectedNames, it.Label)
	}
	// Only keep agents that have adapters wired up.
	selectedNames = filterSupportedAgents(selectedNames)
	// If the user has the "auto-apply low risk" preference, we promote
	// high-confidence candidates before applying.
	autoApply := m.questionnaire.AutoApplyLowRisk()
	return m, tea.Batch(
		m.spinner.Init(),
		applyRulesCmd(m.cwd, m.dbPath, selectedNames, autoApply),
	)
}

// openBrowser tries to open the web console in the user's browser, and
// always falls back to printing the URL so the user can open it manually.
func openBrowser() tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("open", "http://localhost:7878").Start()
		fmt.Println("If your browser did not open automatically, visit:")
		fmt.Println("  http://localhost:7878")
		return nil
	}
}

// filterSupportedAgents keeps only agents that have an adapter wired up in
// adapterForLabel. This avoids running into "unknown agent" errors when
// GitHub Copilot is selected but no adapter exists for it yet.
func filterSupportedAgents(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		switch n {
		case "Claude Code", "Cursor", "Codex":
			out = append(out, n)
		}
	}
	return out
}

// View renders the onboarding TUI.
//
// This is the new visual: ASCII art SHADOW banner → horizontal step
// pipeline → focused Card containing the current step's body → help
// footer. Each visual layer is a separate component (banner.go,
// pipeline.go, card.go) so they can be unit-tested in isolation and
// re-used on other screens.
//
// Constraints honored from the spec:
//   - Banner has purple→blue gradient (left-to-right sweep).
//   - Pipeline is a single line: ●━━━━━●━━━━━○━━━━━○
//   - Active step's card uses brand purple + bold border.
//   - All content is wrapped in rounded-border cards.
//   - Output is centered with lipgloss.Place so 80-col and 120-col
//     terminals both look balanced.
func (m OnboardingModel) View() string {
	s := S()

	// --- Header: brand banner (gradient SHADOW wordmark + slogan) ---
	banner := components.NewBanner()
	header := banner.View(1.0)

	// --- Pipeline: horizontal step progress ---
	m.progress.Current = m.step
	pipeline := m.progress.View()

	// --- Body: error / loading / per-step content, each in a Card ---
	var bodyCard components.Card
	var help string

	if m.err != nil {
		// Error state: a red-tinted card with the error and a hint.
		bodyCard = components.Card{
			Title:   "Something went wrong",
			Content: s.Error.Render(fmt.Sprintf("✗ %v", m.err)) + "\n\n" +
				s.Dim.Render("Press Enter to retry, or q to quit."),
			Focused: true,
		}
		help = s.Dim.Render("Enter retry · q quit")
	} else if m.loading {
		// Loading state: spinner in a card. The spinner brings its
		// own card chrome — we just lift it to a similar width.
		loadingBody := m.spinner.View()
		bodyCard = components.Card{
			Title:   "Working",
			Content: loadingBody,
			Focused: true,
		}
		help = s.Dim.Render("Please wait...")
	} else {
		switch m.step {
		case 1:
			bodyCard = components.Card{
				Title: "Welcome to Shadow!",
				Content: strings.Join([]string{
					"This will take about 60 seconds. Everything stays local.",
					"You can skip, go back, or quit at any time.",
					"",
					s.Bold.Render("Your data:") + "",
					"  " + s.Success.Render("✓") + " Stored only on this machine",
					"  " + s.Success.Render("✓") + " Never uploaded without your consent",
					"  " + s.Success.Render("✓") + " Keys / tokens automatically blocked",
				}, "\n"),
				Focused: true,
			}
			help = s.Dim.Render("Press Enter to begin...")

		case 2:
			bodyCard = components.Card{
				Title: "Privacy & Scope",
				Content: strings.Join([]string{
					s.Bold.Render("Shadow will:"),
					"  " + s.Success.Render("✓") + " Read project code & agent session logs",
					"  " + s.Success.Render("✓") + " Write managed rules to agent context files",
					"  " + s.Success.Render("✓") + " Never store keys, tokens, or credentials",
					"  " + s.Success.Render("✓") + " Store only distilled rules, not raw conversations",
					"",
					s.Bold.Render("🔒 Default protections (always on):"),
					"  • Block: API keys, tokens, .env files",
					"  • Exclude: node_modules, .git, dist, .env*",
					"  • Only distilled rules stored, never raw code",
				}, "\n"),
				Focused: true,
			}
			help = s.Dim.Render("Press Enter to accept safe defaults...")

		case 3:
			detectMsg := "No agents auto-detected. Select manually:"
			if len(m.agentsFound) > 0 {
				detectMsg = fmt.Sprintf("Detected %d agent(s) on your machine:", len(m.agentsFound))
			}
			bodyCard = components.Card{
				Title: "Select Agents",
				Content: s.Dim.Render(detectMsg) + "\n\n" + m.agents.View(),
				Focused: true,
			}
			help = s.Dim.Render("↑↓ move · Space toggle · Enter confirm")

		case 4:
			// Step 4 is a multi-phase flow tracked by m.subStep:
			//   subStepScanIdle    — pre-scan: show the "what we'll scan" prompt.
			//   subStepScanDone    — scan finished: show the results card
			//                        and invite the user to continue.
			//   subStepQuestion    — questionnaire component is shown
			//                        (Questionnaire.View() handles its own UI).
			//   subStepApplied     — final summary ("Shadow is ready!").
			// The other substeps (subStepScanRunning, subStepApplying) flip
			// m.loading=true and are handled by the spinner branch above.
			switch m.subStep {
			case subStepScanDone:
				bodyCard = components.Card{
					Title: "Initial scan complete",
					Content: renderScanResults(s, m),
					Focused: true,
				}
				help = s.Dim.Render("Press Enter to continue, b to go back, s to skip…")
			case subStepQuestion:
				bodyCard = components.Card{
					Title:   "A couple of questions",
					Content: Questionnaire{}.View(),
					Focused: true,
				}
				help = s.Dim.Render("↑↓ select  Enter confirm  s skip")
			case subStepApplied:
				bodyCard = components.Card{
					Title:   "Onboarding complete",
					Content: renderAppliedSummary(s, m),
					Focused: true,
				}
				help = s.Dim.Render("Press Enter to open web console (or q to stay in terminal)...")
			default:
				// subStepScanIdle (the entry state) and any unknown value
				// show the pre-scan prompt. This is also the safe
				// fallback when m.done is true but subStep is wrong.
				bodyCard = components.Card{
					Title: "Initial Memory Generation",
					Content: strings.Join([]string{
						"Shadow will scan your project for:",
						"  • Package manager (lockfile detection)",
						"  • Test framework (config detection)",
						"  • Existing rules (CLAUDE.md, .cursorrules, AGENTS.md)",
						"  • Language and framework detection",
					}, "\n"),
					Focused: true,
				}
				help = s.Dim.Render("Press Enter to scan...  (s to skip)")
			}
		}
	}

	// --- Assemble: header / pipeline / card / help ---
	var screen strings.Builder
	screen.WriteString(header)
	screen.WriteString("\n")
	screen.WriteString(pipeline)
	screen.WriteString("\n\n")
	screen.WriteString(bodyCard.View())
	screen.WriteString("\n")
	screen.WriteString(help)

	// --- Center the whole screen at the terminal width ---
	// We choose 80 as the default content width (cards and pipeline
	// look balanced around that), and cap to whatever the caller
	// passes via the bubble tea window size (the bubbletea program
	// sets this automatically; we use a sensible fallback).
	// NOTE: We don't have access to the terminal size from inside
	// the View() function (the bubbletea pattern is to read it in
	// a WindowSizeMsg in Update). To keep the View() pure, we just
	// rely on lipgloss.Place with a default 100-col width. A future
	// track can plumb the window size into the model.
	return lipgloss.PlaceHorizontal(100, lipgloss.Center, screen.String())
}

// --- Tea commands ---

type daemonCheckMsg struct{ running bool }
type agentDetectMsg struct{ agents []string }
type scanCompleteMsg struct {
	count         int
	importedFiles []string
	facts         []string
}
type applyCompleteMsg struct {
	appliedFiles []string
	errs         map[string]error
}
type errorMsg struct{ err error }

// renderScanResults builds the body text for the "scan complete" card
// in step 4. It shows the discovered facts, the imported files, and
// the count of candidate rules. Used by the new View() sub-step
// rendering.
func renderScanResults(s Styles, m OnboardingModel) string {
	var b strings.Builder
	if len(m.scanFacts) > 0 {
		b.WriteString(s.Dim.Render("Discovered:") + "\n")
		for _, fact := range m.scanFacts {
			if len(fact) > 60 {
				fact = fact[:60] + "..."
			}
			b.WriteString("  • " + s.Dim.Render(fact) + "\n")
		}
		b.WriteString("\n")
	}
	if len(m.importedFiles) > 0 {
		b.WriteString(s.Dim.Render("Imported from: ") +
			strings.Join(m.importedFiles, ", ") + "\n\n")
	}
	b.WriteString(fmt.Sprintf("  %s %d candidate rules generated\n",
		s.Success.Render("✓"), m.rulesGenerated))
	return b.String()
}

// renderAppliedSummary builds the body text for the final "Shadow is
// ready!" card. It mirrors the legacy summary but also surfaces
// apply errors and the list of files actually written.
func renderAppliedSummary(s Styles, m OnboardingModel) string {
	var b strings.Builder
	b.WriteString(s.Success.Render("✓ Shadow is ready!") + "\n\n")
	if len(m.agentsFound) > 0 {
		b.WriteString(fmt.Sprintf("  Agents connected: %s\n",
			strings.Join(m.agentsFound, ", ")))
	}
	b.WriteString(fmt.Sprintf("  Initial memories: %d candidate rules\n", m.rulesGenerated))
	if len(m.appliedFiles) > 0 {
		b.WriteString(fmt.Sprintf("  Files written: %s\n",
			strings.Join(m.appliedFiles, ", ")))
	}
	if len(m.applyErrors) > 0 {
		b.WriteString("\n")
		b.WriteString(s.Warning.Render("  ⚠ Some files could not be written:") + "\n")
		for name, err := range m.applyErrors {
			b.WriteString(fmt.Sprintf("    • %s: %s\n", name, s.Dim.Render(err.Error())))
		}
	}
	b.WriteString("\n")
	b.WriteString(s.Dim.Render("Next steps:") + "\n")
	b.WriteString("  shadow status  — check everything is running\n")
	b.WriteString("  shadow open    — open web console at localhost:7878\n")
	b.WriteString("  shadow review  — review candidate rules\n")
	return b.String()
}

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
			// Fall back to just counting
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
		// Try to create; ignore error if already exists.
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

// applyRulesCmd runs the apply phase after the questionnaire completes.
// It opens the database, optionally promotes high-confidence candidates
// (when the user chose "auto-apply low risk"), then writes the resulting
// active rules to each selected agent's adapter for both global and
// project scopes.
//
// The returned tea.Msg is always applyCompleteMsg — even on per-adapter
// errors — so the TUI can render the per-agent result list.
func applyRulesCmd(cwd, dbPath string, selectedAgentNames []string, autoApplyLowRisk bool) tea.Cmd {
	return func() tea.Msg {
		// If the user opted in to "auto apply low risk", promote
		// high-confidence candidates to active before writing them out.
		// Threshold matches the OnboardingModel.copy that uses 0.9.
		const lowRiskThreshold = 0.9

		db, err := storage.Open(dbPath)
		if err != nil {
			slog.Warn("onboarding: apply — cannot open database", "error", err)
			return applyCompleteMsg{
				appliedFiles: nil,
				errs:         map[string]error{"db": err},
			}
		}
		defer db.Close()

		ruleRepo := storage.NewRuleRepo(db)

		if autoApplyLowRisk {
			candidates, err := ruleRepo.List(storage.RuleFilter{Status: "candidate"})
			if err == nil {
				for _, c := range candidates {
					if c.Confidence >= lowRiskThreshold {
						c.Status = "active"
						c.UpdatedAt = storage.Now()
						if err := ruleRepo.Update(c, "auto", "low-risk auto-apply"); err != nil {
							slog.Warn("onboarding: promote candidate", "id", c.ID, "error", err)
						}
					}
				}
			}
		}

		// Load active rules and hand them to the adapter layer.
		active, err := ruleRepo.List(storage.RuleFilter{Status: "active"})
		if err != nil {
			return applyCompleteMsg{
				appliedFiles: nil,
				errs:         map[string]error{"db": err},
			}
		}

		applied, errs := ApplyAgentsToAdapters(selectedAgentNames, cwd, active)
		return applyCompleteMsg{appliedFiles: applied, errs: errs}
	}
}

func agentNames(selected []string) []string {
	if selected == nil {
		return []string{}
	}
	return selected
}
