package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Step state controls how a single step node in the pipeline is rendered.
type StepState int

const (
	StepPending StepState = iota // not yet reached — gray ○
	StepCurrent                  // active step — purple ● + bold label
	StepDone                     // finished — green ✓
)

// Step is one node in a horizontal step pipeline. The label is shown
// beneath the indicator in the rendered output.
type Step struct {
	Label string
	State StepState
}

// Pipeline is a horizontal step indicator. Visually:
//
//	●━━━━━●━━━━━○━━━━━○
//	Setup  Auth  Sync  Done
//
// All steps are rendered on a single line (the indicators + connector
// lines) and the labels are on a second line beneath. Width is
// computed from the labels and connector length to look balanced
// in both 80-column and 120-column terminals.
type Pipeline struct {
	Steps    []Step
	Width    int // total width budget; 0 = auto-size to terminal width (up to 100)
	MinWidth int // minimum width budget; default 60 — degrades gracefully
}

// NewPipeline creates a pipeline where step 0..current-1 are Done,
// step `current` is Current, and the rest are Pending.
func NewPipeline(labels []string, current int) Pipeline {
	if current < 0 {
		current = 0
	}
	steps := make([]Step, len(labels))
	for i, label := range labels {
		var state StepState
		switch {
		case i < current:
			state = StepDone
		case i == current:
			state = StepCurrent
		default:
			state = StepPending
		}
		steps[i] = Step{Label: label, State: state}
	}
	return Pipeline{Steps: steps, Width: 0, MinWidth: 60}
}

// indicator returns the single-character icon for a step in its given state.
//
// We use ● for the active step (matches the brand dot used in the
// checkbox list) and ✓ for done (matches the successStyle in the
// existing codebase). ○ for pending keeps the visual progression
// ● → ✓ and ○ for "not yet".
func indicator(state StepState) string {
	switch state {
	case StepDone:
		return C.StepDone.Render("✓")
	case StepCurrent:
		return C.StepCurrent.Render("●")
	default:
		return C.StepPending.Render("○")
	}
}

// labelStyle returns the lipgloss style for a label based on its state.
func labelStyle(state StepState) lipgloss.Style {
	switch state {
	case StepDone:
		return C.StepDone
	case StepCurrent:
		return C.StepCurrent
	default:
		return C.StepPending
	}
}

// View renders the pipeline as two lines: indicators (with connectors)
// and labels aligned beneath each indicator.
func (p Pipeline) View() string {
	if len(p.Steps) == 0 {
		return ""
	}

	// Strategy: each step gets a "cell" wide enough to hold the
	// longer of (its label + padding) and (its indicator). The
	// indicator is centered in the cell; the label is centered in
	// the cell. Connectors fill the gap between adjacent cells.
	//
	// We give labels a +2 padding so short labels (e.g. "Go") still
	// have visible breathing room, and so the label-center coincides
	// visually with the indicator-center (which also gets +2 padding
	// symmetrically).
	const (
		connector     = "━━━━━"
		minCellWidth  = 8
		labelPadBonus = 2
	)

	cellWidths := make([]int, len(p.Steps))
	for i, step := range p.Steps {
		labelW := lipgloss.Width(step.Label) + labelPadBonus
		if labelW < minCellWidth {
			labelW = minCellWidth
		}
		cellWidths[i] = labelW
	}

	// Build the indicator line + label line in lockstep. The label
	// line gets the SAME connector width as padding (just spaces,
	// no color) so labels align beneath their indicators.
	connectorWidth := lipgloss.Width(connector)
	var ind, lbl strings.Builder
	for i, step := range p.Steps {
		cell := cellWidths[i]
		indCell := padCenter(indicator(step.State), cell)
		ind.WriteString(indCell)
		lblCell := padCenter(labelStyle(step.State).Render(step.Label), cell)
		lbl.WriteString(lblCell)
		if i < len(p.Steps)-1 {
			ind.WriteString(C.StepLine.Render(connector))
			// Match the indicator-line connector width on the label
			// line so the two lines have the same total width and
			// each label sits under its indicator.
			lbl.WriteString(strings.Repeat(" ", connectorWidth))
		}
	}

	out := ind.String() + "\n" + lbl.String()

	// Width adaptation. If the rendered pipeline exceeds the budget
	// by a wide margin, fall back to the compact form. Otherwise
	// we just return as-is — the caller (onboarding View) wraps
	// this in lipgloss.Place for centering.
	budget := p.Width
	if budget <= 0 {
		budget = p.MinWidth
	}
	if budget > 0 && lipgloss.Width(out) > budget+20 {
		return compactProgress(len(p.Steps), currentIndex(p.Steps))
	}
	return out
}

func currentIndex(steps []Step) int {
	for i, s := range steps {
		if s.State == StepCurrent {
			return i
		}
	}
	return 0
}

// padCenter pads s with spaces so its visible width is exactly w.
func padCenter(s string, w int) string {
	visible := lipgloss.Width(s)
	if visible >= w {
		return s
	}
	left := (w - visible) / 2
	right := w - visible - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// compactProgress is a last-resort fallback for very narrow terminals
// (e.g. < 40 cols). It shows the step counter and the current label
// only — readable, even if not pretty.
func compactProgress(total, current int) string {
	return fmt.Sprintf("%s %s",
		C.StepCurrent.Render(fmt.Sprintf("[%d/%d]", current+1, total)),
		"...")
}
