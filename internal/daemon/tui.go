package daemon

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StepProgress is kept as an exported symbol for backwards compatibility
// with the existing onboarding flow (OnboardingModel.progress is of
// this type). Internally it now delegates to the new components.Pipeline
// for the actual rendering — this file is the "thin compatibility
// shim" between the old type-based world and the new component world.
//
// The fields are still readable by external code that may reference
// them (e.g. tests), and the new View() returns the same kind of
// multi-line string it always did.
type StepProgress struct {
	Current int
	Total   int
	Labels  []string
}

// View renders the legacy step progress indicator.
//
// In the new design we render a HORIZONTAL pipeline (the spec asks
// for "●━━━━━●━━━━━○━━━━━○" on a single line). We use the same
// components.Pipeline that the new screens use, so a UI snapshot
// will look identical whether the old or new call site invokes it.
func (s StepProgress) View() string {
	if s.Total == 0 {
		// Defensive: an uninitialized StepProgress shouldn't crash
		// anyone. Return an empty string instead.
		return ""
	}
	// StepProgress.Current is 1-indexed (the old contract); the
	// new components.Pipeline is 0-indexed.
	idx := s.Current - 1
	if idx < 0 {
		idx = 0
	}
	labels := s.Labels
	if labels == nil {
		labels = make([]string, s.Total)
		for i := range labels {
			labels[i] = fmt.Sprintf("Step %d", i+1)
		}
	}
	// Avoid an import cycle: StepProgress lives in package daemon,
	// components also imports daemon, so we can't import components
	// from here. The pipeline rendering is duplicated below — see
	// buildPipelineView. We keep this duplicate small and focused
	// on the spec's specific output (a horizontal line + label line).
	return buildPipelineView(labels, idx)
}

// buildPipelineView is the actual horizontal step indicator renderer.
// It produces two lines:
//
//	●━━━━━●━━━━━○━━━━━○
//	Setup  Auth  Sync  Done
//
// Done steps show ✓, current step shows ● (in brand purple), and
// pending steps show ○ (in dim gray). Connectors are ━━━ in border
// purple. All rendering is in fixed-width cells so labels align
// under their indicators.
//
// Note: this implementation mirrors components.Pipeline in
// internal/daemon/components/pipeline.go. They must stay in sync
// because StepProgress (here) and Pipeline (components) are both
// used in different code paths. We can't import components from
// here (circular import), so the logic is duplicated.
func buildPipelineView(labels []string, currentIdx int) string {
	if len(labels) == 0 {
		return ""
	}
	if currentIdx < 0 {
		currentIdx = 0
	}
	if currentIdx >= len(labels) {
		currentIdx = len(labels) - 1
	}
	s := S()

	const (
		connector     = "━━━━━"
		minCellWidth  = 8
		labelPadBonus = 2
	)
	cellWidths := make([]int, len(labels))
	for i, label := range labels {
		w := lipgloss.Width(label) + labelPadBonus
		if w < minCellWidth {
			w = minCellWidth
		}
		cellWidths[i] = w
	}

	connectorWidth := lipgloss.Width(connector)
	var ind, lbl strings.Builder
	for i, label := range labels {
		cell := cellWidths[i]
		var icon string
		var labelStyle lipgloss.Style
		switch {
		case i < currentIdx:
			icon = s.StepDone.Render("✓")
			labelStyle = s.StepDone
		case i == currentIdx:
			icon = s.StepCurrent.Render("●")
			labelStyle = s.StepCurrent
		default:
			icon = s.StepPending.Render("○")
			labelStyle = s.StepPending
		}
		indCell := padToWidth(icon, cell)
		ind.WriteString(indCell)
		lblCell := padToWidth(labelStyle.Render(label), cell)
		lbl.WriteString(lblCell)
		if i < len(labels)-1 {
			ind.WriteString(s.StepLine.Render(connector))
			// Match the indicator-line connector width on the
			// label line so the two lines align visually.
			lbl.WriteString(strings.Repeat(" ", connectorWidth))
		}
	}

	return ind.String() + "\n" + lbl.String()
}

// padToWidth returns s padded with leading and trailing spaces so its
// visible width is exactly w. Used to align labels beneath their
// indicators in the horizontal pipeline.
func padToWidth(s string, w int) string {
	visible := lipgloss.Width(s)
	if visible >= w {
		return s
	}
	left := (w - visible) / 2
	right := w - visible - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// CheckboxList is a multi-select list component (kept exported for
// backwards compat with the OnboardingModel.agents field).
type CheckboxList struct {
	Items    []CheckboxItem
	Cursor   int
	Selected map[int]bool
}

// CheckboxItem is one entry in a CheckboxList.
type CheckboxItem struct {
	Label       string
	Description string
}

// NewCheckboxList creates a new checkbox list with all items
// pre-selected (the previous default).
func NewCheckboxList(items []CheckboxItem) CheckboxList {
	selected := make(map[int]bool, len(items))
	for i := range items {
		selected[i] = true
	}
	return CheckboxList{Items: items, Selected: selected}
}

// Update handles key events. Kept identical to the previous
// implementation — this is the "another track" code path.
func (c CheckboxList) Update(msg tea.Msg) CheckboxList {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if c.Cursor > 0 {
				c.Cursor--
			}
		case "down", "j":
			if c.Cursor < len(c.Items)-1 {
				c.Cursor++
			}
		case " ":
			c.Selected[c.Cursor] = !c.Selected[c.Cursor]
		}
	}
	return c
}

// View renders the checkbox list as a series of cards. Each item is
// rendered inside a small card-style box (border + padding), which
// is the new visual treatment — the spec asked for "agent 选择卡片化".
//
// We keep the output a single string (per the bubbletea View contract)
// but compose it from individual card renders. The result is a
// vertical stack of cards, with the focused card highlighted.
func (c CheckboxList) View() string {
	s := S()
	var b strings.Builder
	for i, item := range c.Items {
		checked := s.StepPending.Render("○")
		checkboxMark := " "
		if c.Selected[i] {
			checked = s.StepDone.Render("●")
		}
		if i == c.Cursor {
			checkboxMark = s.StepCurrent.Render(">")
		} else {
			checkboxMark = " "
		}

		// Build the content line for this card. We use the
		// description (if any) as muted secondary text.
		titleLine := fmt.Sprintf("%s %s  %s", checkboxMark, checked, s.Bold.Render(item.Label))
		var descLine string
		if item.Description != "" {
			descLine = s.Dim.Render("  → "+item.Description) + "\n"
		}

		// Card border. Focused cards get the brand purple border;
		// unfocused cards get the deep-purple border.
		card := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Width(58)
		if i == c.Cursor {
			card = card.
				BorderForeground(lipgloss.Color(ColorBrand)).
				Bold(true)
		} else {
			card = card.
				BorderForeground(lipgloss.Color(ColorBorder))
		}
		b.WriteString(card.Render(titleLine + "\n" + descLine))
		b.WriteString("\n")
	}
	return b.String()
}

// SelectedItems returns the list of selected items.
func (c CheckboxList) SelectedItems() []CheckboxItem {
	items := make([]CheckboxItem, 0, len(c.Selected))
	for i, selected := range c.Selected {
		if selected {
			items = append(items, c.Items[i])
		}
	}
	return items
}

// SpinnerModel is the loading spinner (kept exported for
// backwards compat with OnboardingModel.spinner).
type SpinnerModel struct {
	Message string
	frames  []string
	frame   int
}

// NewSpinner creates a new spinner with the default braille-dot
// animation. The message is shown next to the spinner.
func NewSpinner(message string) SpinnerModel {
	return SpinnerModel{
		Message: message,
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// Init starts the spinner tick.
func (s SpinnerModel) Init() tea.Cmd {
	return spinnerTick()
}

// Update handles spinner ticks.
func (s SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case spinnerTickMsg:
		s.frame = (s.frame + 1) % len(s.frames)
		return s, spinnerTick()
	}
	return s, nil
}

// View renders the spinner in a card — the loading state should
// look like a distinct "loading" panel, not a loose line of text.
func (s SpinnerModel) View() string {
	spinnerText := fmt.Sprintf("%s %s", S().Accent.Render(s.frames[s.frame]), s.Message)
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Padding(1, 2).
		Width(60)
	return card.Render(spinnerText)
}

type spinnerTickMsg struct{}

func spinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(_ time.Time) tea.Msg { return spinnerTickMsg{} })
}

// BrandHeader returns the Shadow brand header — the legacy hook
// used by older code paths. The new TUI uses components.Banner
// directly (with the gradient + animation), so this function is
// kept as a thin compatibility shim.
func BrandHeader(version string) string {
	return S().Brand.Render("👻 Shadow") + S().Dim.Render(fmt.Sprintf(" v%s", version)) + "\n" +
		S().Dim.Render("Your AI agent memory layer — correct once, remember everywhere.") + "\n"
}
