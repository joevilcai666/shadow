package daemon

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Brand styles.
var (
	brandStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A855F7"))
	accentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#818CF8"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	boldStyle    = lipgloss.NewStyle().Bold(true)
)

// StepProgress renders a step progress indicator.
type StepProgress struct {
	Current int
	Total   int
	Labels  []string
}

// View renders the step progress bar.
func (s StepProgress) View() string {
	var b strings.Builder
	b.WriteString(dimStyle.Render(fmt.Sprintf("Step %d/%d", s.Current, s.Total)))
	b.WriteString("\n")

	for i, label := range s.Labels {
		if i+1 == s.Current {
			b.WriteString(accentStyle.Render("● "))
			b.WriteString(boldStyle.Render(label))
		} else if i+1 < s.Current {
			b.WriteString(successStyle.Render("✓ "))
			b.WriteString(dimStyle.Render(label))
		} else {
			b.WriteString(dimStyle.Render("○ "))
			b.WriteString(dimStyle.Render(label))
		}
		if i < len(s.Labels)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// CheckboxList is a multi-select list component.
type CheckboxList struct {
	Items    []CheckboxItem
	Cursor   int
	Selected map[int]bool
}

// CheckboxItem represents an item in the checkbox list.
type CheckboxItem struct {
	Label       string
	Description string
}

// NewCheckboxList creates a new checkbox list.
func NewCheckboxList(items []CheckboxItem) CheckboxList {
	selected := make(map[int]bool)
	for i := range items {
		selected[i] = true
	}
	return CheckboxList{Items: items, Selected: selected}
}

// Update handles key events.
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

// View renders the checkbox list.
func (c CheckboxList) View() string {
	var b strings.Builder
	for i, item := range c.Items {
		cursor := " "
		if i == c.Cursor {
			cursor = accentStyle.Render(">")
		}
		checked := "○"
		if c.Selected[i] {
			checked = successStyle.Render("●")
		}
		b.WriteString(fmt.Sprintf("  %s %s %s", cursor, checked, item.Label))
		if item.Description != "" {
			b.WriteString(dimStyle.Render(" — " + item.Description))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// SelectedItems returns the list of selected items.
func (c CheckboxList) SelectedItems() []CheckboxItem {
	var items []CheckboxItem
	for i, selected := range c.Selected {
		if selected {
			items = append(items, c.Items[i])
		}
	}
	return items
}

// SpinnerModel shows a loading spinner with a message.
type SpinnerModel struct {
	Message string
	frames  []string
	frame   int
}

// NewSpinner creates a new spinner.
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

// View renders the spinner.
func (s SpinnerModel) View() string {
	return fmt.Sprintf("  %s %s", accentStyle.Render(s.frames[s.frame]), s.Message)
}

type spinnerTickMsg struct{}

func spinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(_ time.Time) tea.Msg { return spinnerTickMsg{} })
}

// BrandHeader returns the Shadow brand header.
func BrandHeader(version string) string {
	return brandStyle.Render("👻 Shadow") + dimStyle.Render(fmt.Sprintf(" v%s", version)) + "\n" +
		dimStyle.Render("Your AI agent memory layer — correct once, remember everywhere.") + "\n"
}
