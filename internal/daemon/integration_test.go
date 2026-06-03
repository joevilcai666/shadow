package daemon

// INTEGRATION_TEST: end-to-end smoke test for the onboarding state machine
// combined with the real adapter-write pipeline. This is the bridge
// between the "tui-visual" track (banner / pipeline / card / View) and
// the "onboarding-logic" track (state machine, questionnaire, real
// adapter writes).
//
// What this test verifies:
//
//   1. The state machine advances through all 4 steps in order
//   2. Pressing 'b' on any step goes back one step
//   3. The questionnaire component integrates with the host TUI
//      (subStep == subStepQuestion → answer → startApply → subStepApplying)
//   4. ApplyAgentsToAdapters writes real files on disk and the
//      '>>> shadow managed >>>' block is present
//   5. The completion screen reports numbers that match the inputs
//      (rules generated = rules applied to at least one target)
//
// The test does NOT exercise the bubbletea Program loop. It drives
// OnboardingModel.Update / handleEnter directly, the same way the
// bubbletea runtime would. This keeps the test fast and deterministic —
// no terminal I/O, no timing.
//
// Run with:
//
//   go test -v -run TestIntegration ./internal/daemon/...

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joevilcai666/shadow/internal/storage"
)

// stepOnboarding is a tiny helper that drives the model one Update
// call and type-asserts the result back to OnboardingModel. It exists
// purely to keep the integration test body readable.
func stepOnboarding(m OnboardingModel, msg tea.Msg) (OnboardingModel, tea.Cmd) {
	res, cmd := m.Update(msg)
	return res.(OnboardingModel), cmd
}

// TestIntegration_Onboarding_FullFlow drives the OnboardingModel through
// the entire happy path (steps 1→2→3→4 with questionnaire + apply) and
// verifies that:
//   - each step transition is correct
//   - the state machine honors the 'b' back-key
//   - the final CLAUDE.md on disk contains the managed block with the
//     active rule that was created earlier
//   - the completion summary numbers match what was actually written
func TestIntegration_Onboarding_FullFlow(t *testing.T) {
	// --- Setup: tmp project, fake home, tmp db -----------------------
	projectDir := t.TempDir()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dbPath := filepath.Join(t.TempDir(), "shadow.db")

	// Build a model and set its cwd to our tmp project.
	m := NewOnboardingModel("0.0.0-integration")
	m.cwd = projectDir
	m.dbPath = dbPath

	// --- Step 1: welcome ---------------------------------------------
	// The model starts on step 1. Simulate the daemon check reply
	// that Init() would have scheduled.
	m, _ = stepOnboarding(m, daemonCheckMsg{running: false})
	if m.step != 1 {
		t.Fatalf("after daemonCheckMsg{running:false}, expected step 1, got %d", m.step)
	}

	// Press Enter to register the daemon.
	m, _ = stepOnboarding(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.loading {
		t.Fatal("after Enter on step 1, expected loading=true (registering daemon)")
	}
	// Simulate the daemon check completing — and report it as running now.
	m, _ = stepOnboarding(m, daemonCheckMsg{running: true})
	if m.step != 2 {
		t.Fatalf("after daemon registers, expected step 2 (privacy), got %d", m.step)
	}

	// --- 'b' back-key regression: from step 2 go back to step 1 -------
	m, _ = stepOnboarding(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if m.step != 1 {
		t.Fatalf("after 'b' from step 2, expected step 1, got %d", m.step)
	}
	// Re-advance: Enter → loading, then daemonCheckMsg → step 2.
	m, _ = stepOnboarding(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = stepOnboarding(m, daemonCheckMsg{running: true})
	if m.step != 2 {
		t.Fatalf("after re-entering, expected step 2, got %d", m.step)
	}

	// --- Step 2: privacy --------------------------------------------
	m, _ = stepOnboarding(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != 3 {
		t.Fatalf("after Enter on step 2, expected step 3 (agents), got %d", m.step)
	}
	if !m.loading {
		t.Fatal("after Enter on step 2, expected loading=true (detecting agents)")
	}

	// --- Step 3: agent selection ------------------------------------
	// Simulate detectAgents result — only Claude Code is installed in
	// the fake home, so this matches what the production detection
	// logic would return for an empty environment.
	m, _ = stepOnboarding(m, agentDetectMsg{agents: []string{"Claude Code"}})
	if m.step != 4 {
		t.Fatalf("after agent detection, expected step 4, got %d", m.step)
	}
	if m.subStep != subStepScanIdle {
		t.Fatalf("after agent detection, expected subStep=subStepScanIdle, got %d", m.subStep)
	}

	// --- Step 4a: scan idle → scan running --------------------------
	m, _ = stepOnboarding(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.subStep != subStepScanRunning {
		t.Fatalf("after Enter on subStepScanIdle, expected subStepScanRunning, got %d", m.subStep)
	}
	if !m.loading {
		t.Fatal("after Enter on subStepScanIdle, expected loading=true")
	}

	// --- Step 4b: scan done → questionnaire -------------------------
	// Pre-create an active rule so the apply phase has something to write.
	if err := func() error {
		db, err := storage.Open(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		repo := storage.NewRuleRepo(db)
		return repo.Create(&storage.Rule{
			ID:         storage.NewID(),
			Content:    "Integration: always run `go test ./...` before committing",
			Status:     "active",
			Confidence: 0.95,
			Scope:      "project",
			Version:    1,
			CreatedAt:  storage.Now(),
			UpdatedAt:  storage.Now(),
		})
	}(); err != nil {
		t.Fatalf("seed rule: %v", err)
	}

	m, _ = stepOnboarding(m, scanCompleteMsg{
		count:         1,
		importedFiles: []string{},
		facts:         []string{"Detected Go module", "Has tests"},
	})
	if m.subStep != subStepScanDone {
		t.Fatalf("after scan complete, expected subStepScanDone, got %d", m.subStep)
	}
	if m.rulesGenerated != 1 {
		t.Fatalf("expected rulesGenerated=1, got %d", m.rulesGenerated)
	}

	// Press Enter → advance to questionnaire.
	m, _ = stepOnboarding(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.subStep != subStepQuestion {
		t.Fatalf("after Enter on subStepScanDone, expected subStepQuestion, got %d", m.subStep)
	}

	// --- Step 4c: questionnaire → apply -----------------------------
	// Sanity check: View() should now contain the questionnaire UI.
	view := m.View()
	if !strings.Contains(view, "A couple of questions") {
		t.Errorf("questionnaire view missing 'A couple of questions' card title; got:\n%s", view)
	}
	if !strings.Contains(view, "Q1/2") {
		t.Errorf("questionnaire view missing 'Q1/2' prompt; got:\n%s", view)
	}

	// Answer Q1: press Enter to accept default.
	m, _ = stepOnboarding(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.questionnaireIdx != 1 {
		t.Fatalf("after answering Q1, expected questionnaireIdx=1, got %d", m.questionnaireIdx)
	}
	if m.questionnaire.IsDone() {
		t.Fatal("after answering Q1, questionnaire should not yet be done")
	}

	// Answer Q2: press Enter to accept default.
	m, _ = stepOnboarding(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.questionnaireDone {
		t.Fatal("after answering Q2, questionnaire should be done")
	}
	if !m.questionnaire.IsDone() {
		t.Fatal("questionnaire.IsDone() should be true after second Enter")
	}

	// After the second Enter on subStepQuestion, the model should
	// have started the apply phase: subStepApplying + loading.
	if m.subStep != subStepApplying {
		t.Fatalf("after answering Q2, expected subStepApplying, got %d", m.subStep)
	}
	if !m.loading {
		t.Fatal("after answering Q2, expected loading=true (applying rules)")
	}

	// --- Step 4d: apply done → summary ------------------------------
	// Drive ApplyAgentsToAdapters directly (this is what applyRulesCmd
	// would have done) and feed the result back as a tea.Msg.
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer db.Close()
	active, err := storage.NewRuleRepo(db).List(storage.RuleFilter{Status: "active"})
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	applied, applyErrs := ApplyAgentsToAdapters(
		[]string{"Claude Code"},
		projectDir,
		active,
	)
	if len(applyErrs) > 0 {
		t.Fatalf("ApplyAgentsToAdapters errors: %v", applyErrs)
	}
	if len(applied) < 2 {
		// Global + project = 2 paths for Claude Code
		t.Fatalf("expected at least 2 applied paths (global+project), got %d: %v", len(applied), applied)
	}

	m, _ = stepOnboarding(m, applyCompleteMsg{
		appliedFiles: applied,
		errs:         applyErrs,
	})
	if m.subStep != subStepApplied {
		t.Fatalf("after apply complete, expected subStepApplied, got %d", m.subStep)
	}
	if !m.done {
		t.Fatal("after apply complete, expected done=true")
	}

	// --- Verify the final summary view reports the correct counts ----
	view = m.View()
	if !strings.Contains(view, "Shadow is ready!") {
		t.Errorf("final summary missing 'Shadow is ready!'; got:\n%s", view)
	}
	if !strings.Contains(view, "Initial memories: 1 candidate rules") {
		t.Errorf("final summary missing correct rule count; got:\n%s", view)
	}
	if !strings.Contains(view, "Files written:") {
		t.Errorf("final summary missing 'Files written:'; got:\n%s", view)
	}

	// --- Verify the on-disk project CLAUDE.md actually got written ---
	claudePath := filepath.Join(projectDir, "CLAUDE.md")
	claudeBytes, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	claudeStr := string(claudeBytes)
	if !strings.Contains(claudeStr, "# >>> shadow managed >>>") {
		t.Error("project CLAUDE.md missing managed block begin marker")
	}
	if !strings.Contains(claudeStr, "# <<< shadow managed <<<") {
		t.Error("project CLAUDE.md missing managed block end marker")
	}
	if !strings.Contains(claudeStr, "Integration: always run `go test ./...` before committing") {
		t.Error("active rule did not make it into the project CLAUDE.md managed block")
	}

	// --- Verify the global CLAUDE.md was also written ----------------
	globalPath := filepath.Join(fakeHome, ".claude", "CLAUDE.md")
	if _, err := os.Stat(globalPath); err != nil {
		t.Errorf("global CLAUDE.md not written at %s: %v", globalPath, err)
	}
}
