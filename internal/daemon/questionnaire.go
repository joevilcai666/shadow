package daemon

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Questionnaire is a small 2-question radio-button component. The onboarding
// wizard uses it after the initial scan completes to collect two quick
// preferences before applying rules.
//
// It is intentionally minimal and test-friendly: it does not own a spinner
// or async work. The TUI is responsible for wiring Enter / s to commit or
// skip.
type Questionnaire struct {
	idx     int    // current question index (0 or 1)
	done    bool   // true after Enter on the last question
	skipped bool   // true if user pressed s to skip
	answers [2]int // selected option index per question
}

// NewQuestionnaire returns a Questionnaire with sensible defaults that match
// the PRD 3.2 spec: "Use existing code style" and "Auto-apply low risk".
func NewQuestionnaire() Questionnaire {
	return Questionnaire{
		idx: 0,
		// 0 = top option (default) for both questions.
		answers: [2]int{0, 0},
	}
}

// Questions is the static list of questions + options displayed to the user.
// Indexing matches the [2]int answers array.
//
// Q1 — code style preference
//   0: Use existing code style
//   1: Set later in the dashboard
//
// Q2 — auto-apply low-risk rules
//   0: Yes (auto-apply)
//   1: Candidate only — I review first
var Questions = []Question{
	{
		Prompt: "Q1/2  代码风格偏好?  Code style preference?",
		Options: []QuestionOption{
			{Label: "与现有代码一致  Use existing code style", Description: "Match whatever's already in the repo."},
			{Label: "我稍后在面板设定  Set later in the dashboard", Description: "Apply defaults now; refine in web UI."},
		},
	},
	{
		Prompt: "Q2/2  是否自动生效低风险规则?  Auto-apply low-risk rules?",
		Options: []QuestionOption{
			{Label: "是  Yes", Description: "Auto-promote confidence >= 0.9 rules to active."},
			{Label: "仅候选待我审  Candidate only — I'll review", Description: "Keep everything as candidate; you approve in review."},
		},
	},
}

// Question is one prompt with its radio options.
type Question struct {
	Prompt  string
	Options []QuestionOption
}

// QuestionOption is a single radio choice.
type QuestionOption struct {
	Label       string
	Description string
}

// Index returns the current question index (0-based).
func (q Questionnaire) Index() int { return q.idx }

// IsDone returns true once the user has answered (or skipped) the last
// question. After this, the host TUI should call commit()/Apply().
func (q Questionnaire) IsDone() bool { return q.done || q.skipped }

// IsSkipped reports whether the user explicitly chose to skip.
func (q Questionnaire) IsSkipped() bool { return q.skipped }

// Answers returns a copy of the chosen option indexes. Index 0 = top option.
func (q Questionnaire) Answers() [2]int { return q.answers }

// Update handles a key event for the questionnaire component.
//
//   - up / k     : move selection up on the current question
//   - down / j   : move selection down
//   - enter      : commit the current answer; advance to next question,
//                  or mark done on the last one
//   - s          : skip the rest of the questionnaire
//
// The component is pure: it returns the next state and a tea.Cmd (always nil
// for now). It does not depend on a global event bus.
func (q Questionnaire) Update(msg tea.Msg) Questionnaire {
	if q.IsDone() {
		return q
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return q
	}
	switch km.String() {
	case "up", "k":
		if q.answers[q.idx] > 0 {
			q.answers[q.idx]--
		}
	case "down", "j":
		if q.answers[q.idx] < len(Questions[q.idx].Options)-1 {
			q.answers[q.idx]++
		}
	case "enter":
		if q.idx >= len(Questions)-1 {
			q.done = true
		} else {
			q.idx++
		}
	case "s":
		q.skipped = true
		q.done = true
	}
	return q
}

// View renders the questionnaire as a string ready to print.
func (q Questionnaire) View() string {
	s := S()
	if q.IsDone() {
		if q.skipped {
			return s.Dim.Render("  (questionnaire skipped)")
		}
		return s.Dim.Render(fmt.Sprintf("  (answers recorded: %v)", q.answers))
	}

	var b strings.Builder
	question := Questions[q.idx]
	b.WriteString(s.Bold.Render(question.Prompt))
	b.WriteString("\n\n")

	for i, opt := range question.Options {
		marker := "○"
		if i == q.answers[q.idx] {
			marker = s.Success.Render("●")
		}
		row := fmt.Sprintf("  %s  %s", marker, opt.Label)
		b.WriteString(row)
		if opt.Description != "" {
			b.WriteString("\n")
			b.WriteString("      " + s.Dim.Render(opt.Description))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(s.Dim.Render("  ↑↓ select  Enter confirm  s skip"))
	b.WriteString("\n")

	return b.String()
}

// AutoApplyLowRisk reports whether the user wants low-risk rules to be
// auto-applied. Returns true for answer 0 of Q2, false for answer 1.
func (q Questionnaire) AutoApplyLowRisk() bool {
	return q.answers[1] == 0
}

// UseExistingCodeStyle reports whether the user wants Shadow to match the
// existing code style. Returns true for answer 0 of Q1, false for answer 1.
func (q Questionnaire) UseExistingCodeStyle() bool {
	return q.answers[0] == 0
}

// Style helper to keep the questionnaire self-contained without forcing
// the host TUI to import lipgloss directly.
var _ = lipgloss.NewStyle()
