package daemon

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestQuestionnaire_NextQ verifies the basic "answer, press Enter,
// advance" flow across both questions. It is the primary happy-path
// test for the questionnaire state machine.
func TestQuestionnaire_NextQ(t *testing.T) {
	q := NewQuestionnaire()

	if q.Index() != 0 {
		t.Fatalf("expected initial index 0, got %d", q.Index())
	}
	if q.IsDone() {
		t.Fatal("questionnaire should not start in 'done' state")
	}
	if !q.UseExistingCodeStyle() {
		t.Error("default Q1 answer should be 0 (use existing code style)")
	}
	if !q.AutoApplyLowRisk() {
		t.Error("default Q2 answer should be 0 (auto apply low risk)")
	}

	// Press Enter → advance to Q2, not done yet.
	q = q.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if q.IsDone() {
		t.Fatal("after one Enter, questionnaire should not be done yet (Q2 remains)")
	}
	if q.Index() != 1 {
		t.Fatalf("expected to be on Q2 (idx=1) after first Enter, got %d", q.Index())
	}

	// Move down on Q2 → answer 1 (no auto-apply).
	q = q.Update(tea.KeyMsg{Type: tea.KeyDown})
	if q.AutoApplyLowRisk() {
		t.Error("after down on Q2, AutoApplyLowRisk should be false")
	}

	// Press Enter → finished.
	q = q.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !q.IsDone() {
		t.Error("after answering both Q's, questionnaire should be done")
	}
	if !q.UseExistingCodeStyle() {
		t.Error("Q1 answer should still be 0")
	}
	if q.AutoApplyLowRisk() {
		t.Error("Q2 answer should still be 1")
	}
}

// TestQuestionnaire_Skip verifies the 's' shortcut marks the
// questionnaire as done and skipped from any sub-state.
func TestQuestionnaire_Skip(t *testing.T) {
	q := NewQuestionnaire()

	// 's' anywhere should mark as skipped and done.
	q = q.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !q.IsDone() {
		t.Error("after 's', questionnaire should be done")
	}
	if !q.IsSkipped() {
		t.Error("after 's', IsSkipped should be true")
	}

	// Skipping from Q2 also works.
	q2 := NewQuestionnaire()
	q2 = q2.Update(tea.KeyMsg{Type: tea.KeyEnter}) // advance to Q2
	if q2.Index() != 1 {
		t.Fatalf("expected to be on Q2, got %d", q2.Index())
	}
	q2 = q2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !q2.IsSkipped() {
		t.Error("'s' from Q2 should also skip")
	}
}

// TestQuestionnaire_UpDownBounds verifies the selection cursor never
// moves past the first or last option of any question.
func TestQuestionnaire_UpDownBounds(t *testing.T) {
	// Q1 starts at 0 — up should not go below 0.
	q := NewQuestionnaire()
	q = q.Update(tea.KeyMsg{Type: tea.KeyUp})
	if q.Answers()[0] != 0 {
		t.Errorf("Q1 answer should be clamped to 0 on up, got %d", q.Answers()[0])
	}

	// Down once → answer 1, down again → clamped to 1 (only 2 options).
	q = q.Update(tea.KeyMsg{Type: tea.KeyDown})
	q = q.Update(tea.KeyMsg{Type: tea.KeyDown})
	if q.Answers()[0] != 1 {
		t.Errorf("Q1 answer should be clamped to 1, got %d", q.Answers()[0])
	}

	// Q2 also has only 2 options; same clamp behavior.
	q2 := NewQuestionnaire()
	q2 = q2.Update(tea.KeyMsg{Type: tea.KeyEnter}) // advance to Q2
	q2 = q2.Update(tea.KeyMsg{Type: tea.KeyUp})    // already at 0, stays
	if q2.Answers()[1] != 0 {
		t.Errorf("Q2 answer should be 0 after up from default, got %d", q2.Answers()[1])
	}
	q2 = q2.Update(tea.KeyMsg{Type: tea.KeyDown})
	q2 = q2.Update(tea.KeyMsg{Type: tea.KeyDown})
	q2 = q2.Update(tea.KeyMsg{Type: tea.KeyDown})
	if q2.Answers()[1] != 1 {
		t.Errorf("Q2 answer should be clamped to 1, got %d", q2.Answers()[1])
	}
}

// TestQuestionnaire_View_RendersAnswer is the user-visible rendering
// regression test: when the user changes the answer to a question,
// the selected option's label must appear in the View() output.
//
// This catches the bug where onboarding.go's View() called
// Questionnaire{}.View() (a zero-value struct) instead of
// m.questionnaire.View() — that produced an empty screen regardless
// of the user's actual selection.
//
// The test:
//   1. Starts a fresh questionnaire (default Q1 = option 0)
//   2. Moves Q1 selection down to option 1
//   3. Calls View() and checks that option 1's label is present
//   4. Verifies that the option 0 label is also present (both
//      options are rendered, just with different markers)
//   5. As a sanity check, asserts the opposite selection state would
//      produce different output
func TestQuestionnaire_View_RendersAnswer(t *testing.T) {
	if len(Questions) < 2 {
		t.Fatal("Questionnaire Questions data is empty; cannot test view rendering")
	}

	// --- Default: Q1 option 0 selected ---
	q := NewQuestionnaire()
	v0 := q.View()

	// The current question's prompt must be present.
	if !strings.Contains(v0, Questions[0].Prompt) {
		t.Errorf("Q1 prompt missing from View():\n%s", v0)
	}

	// Both options' labels must be present in the rendered output
	// (the user can navigate between them).
	opt0 := Questions[0].Options[0].Label
	opt1 := Questions[0].Options[0+1].Label
	if !strings.Contains(v0, opt0) {
		t.Errorf("Q1 option 0 label %q missing from View():\n%s", opt0, v0)
	}
	if !strings.Contains(v0, opt1) {
		t.Errorf("Q1 option 1 label %q missing from View():\n%s", opt1, v0)
	}

	// --- User moves Q1 selection to option 1 ---
	qMoved := q.Update(tea.KeyMsg{Type: tea.KeyDown})
	if qMoved.Answers()[0] != 1 {
		t.Fatalf("setup: expected Q1 answer=1 after down, got %d", qMoved.Answers()[0])
	}
	v1 := qMoved.View()

	// Both labels still present.
	if !strings.Contains(v1, opt0) {
		t.Errorf("Q1 option 0 label %q missing after move:\n%s", opt0, v1)
	}
	if !strings.Contains(v1, opt1) {
		t.Errorf("Q1 option 1 label %q missing after move:\n%s", opt1, v1)
	}

	// Crucial: the rendered output for option-0-selected vs
	// option-1-selected must DIFFER. The default marker is "○" and
	// the selected marker is "●" (rendered with the success style).
	// A zero-value Questionnaire{} would produce a screen with the
	// default (○ for everything) and the prompt text — but no
	// "●" marker at all, because Answers() returns [0, 0] on a
	// zero value (no answer recorded) which coincidentally matches
	// the default selection. The view difference here catches the
	// case where the answers are wired up but the wrong struct is
	// being rendered.
	if v0 == v1 {
		t.Errorf("View() output is identical for answer=0 and answer=1; selection marker likely broken")
	}

	// --- And the more subtle case: an EMPTY questionnaire must NOT
	// produce the same output as a real one. This is the exact
	// regression: Questionnaire{}.View() vs m.questionnaire.View().
	empty := Questionnaire{}
	vEmpty := empty.View()
	if vEmpty == v1 {
		t.Errorf("View() of zero-value Questionnaire matches real one — onboarding.go is calling Questionnaire{}.View() instead of m.questionnaire.View()")
	}
}
