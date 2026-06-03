package daemon

import "github.com/charmbracelet/lipgloss"

// Color palette — single source of truth for all TUI styling.
//
// Brand colors are the public face of Shadow (purple → blue gradient).
// Status colors are universal (green=success, yellow=warning, red=error).
// Dim/border are chrome — used for non-interactive UI scaffolding.
//
// IMPORTANT: this file is the canonical token source for the daemon
// package. The human-readable contract is docs/style-guide.md and
// the machine-readable contract is docs/style-tokens.json. When you
// change a value here, update BOTH docs in the same commit.
//
// The colors are mirrored (intentionally) in
// internal/daemon/components/styles.go because the components
// subpackage cannot import this one (would be a circular import).
// Keep the two in sync.
const (
	ColorBrand   = "#A855F7" // primary purple
	ColorAccent  = "#818CF8" // secondary blue
	ColorSuccess = "#22C55E"
	ColorWarning = "#EAB308"
	ColorError   = "#EF4444"
	ColorDim     = "#6B7280"
	ColorBorder  = "#6D28D9" // deep purple for box borders
	ColorBg      = "#1E1B4B" // optional deep background tint
)

// Styles is the central style registry. All TUI components import this
// instead of declaring their own lipgloss.Style — that way a single
// tweak to ColorBrand propagates everywhere consistently.
type Styles struct {
	// Brand
	Brand   lipgloss.Style // bold purple — primary titles, brand wordmark
	Accent  lipgloss.Style // blue — secondary highlights, links
	Bold    lipgloss.Style // bold-only — for emphasized text without color
	Dim     lipgloss.Style // gray — non-interactive hints, secondary labels
	Success lipgloss.Style // green — ✓ marks, positive state
	Warning lipgloss.Style // yellow — caution, soft alerts
	Error   lipgloss.Style // red — ✗ marks, error state

	// Box / layout
	Border       lipgloss.Style // unfocused card border (deep purple)
	BorderActive lipgloss.Style // active/focused card border (brand purple, bold)
	BoxPadding   lipgloss.Style // base box (rounded border + padding)
	Title        lipgloss.Style // section title inside a card
	Subtitle     lipgloss.Style // small caps / muted subtitle

	// Pipeline steps
	StepDone    lipgloss.Style // green ● with label
	StepCurrent lipgloss.Style // brand ● with bold label
	StepPending lipgloss.Style // dim ○ with dim label
	StepLine    lipgloss.Style // the connecting ━━ between steps
}

// Default returns the production style set.
func Default() Styles {
	return Styles{
		// Brand
		Brand:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorBrand)),
		Accent:  lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)),
		Bold:    lipgloss.NewStyle().Bold(true),
		Dim:     lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim)),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)),
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)),
		Error:   lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)),

		// Box / layout
		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBorder)),
		BorderActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorBrand)),
		BoxPadding: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Padding(1, 2),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorBrand)).
			MarginBottom(1),
		Subtitle: lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color(ColorDim)),

		// Pipeline steps
		StepDone: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSuccess)),
		StepCurrent: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorBrand)),
		StepPending: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDim)),
		StepLine: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBorder)),
	}
}

// App-wide style singleton. Use S() everywhere instead of constructing styles inline.
var appStyles = Default()

// S returns the application style registry.
func S() Styles { return appStyles }
