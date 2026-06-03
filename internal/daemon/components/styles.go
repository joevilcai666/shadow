// Package components holds reusable bubbletea/lipgloss building blocks
// for the Shadow TUI. Each file is one self-contained visual component —
// it can be unit-tested, swapped, or re-used across screens.
//
// The components intentionally do NOT import the parent daemon package
// (to avoid circular imports when the daemon's onboarding screen
// composes them). Instead, this file holds the local style helpers
// used by the components. The color values mirror internal/daemon/styles.go;
// keep them in sync if either changes.
package components

import "github.com/charmbracelet/lipgloss"

// Color palette — mirror of internal/daemon/styles.go. Keep in sync.
const (
	ColorBrand   = "#A855F7" // primary purple
	ColorAccent  = "#818CF8" // secondary blue
	ColorSuccess = "#22C55E"
	ColorWarning = "#EAB308"
	ColorError   = "#EF4444"
	ColorDim     = "#6B7280"
	ColorBorder  = "#6D28D9" // deep purple for box borders
)

// C is the components-local style registry. It exposes the same kinds
// of styles as daemon.S() but as package-level vars (so we don't need
// a constructor call). This is fine because the styles are pure
// functions of the color constants — no mutable state.
var C = struct {
	Brand   lipgloss.Style
	Accent  lipgloss.Style
	Bold    lipgloss.Style
	Dim     lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Border  lipgloss.Style

	// Pipeline step styles
	StepDone    lipgloss.Style
	StepCurrent lipgloss.Style
	StepPending lipgloss.Style
	StepLine    lipgloss.Style

	// Box
	BoxPadding lipgloss.Style
	Title      lipgloss.Style
}{
	Brand:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorBrand)),
	Accent:  lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)),
	Bold:    lipgloss.NewStyle().Bold(true),
	Dim:     lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim)),
	Success: lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)),
	Warning: lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)),
	Error:   lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)),
	Border:  lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)),

	StepDone:    lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)),
	StepCurrent: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorBrand)),
	StepPending: lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim)),
	StepLine:    lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)),

	BoxPadding: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Padding(1, 2),
	Title: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorBrand)).
		MarginBottom(1),
}
