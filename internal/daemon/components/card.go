package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Card is a box-border + padding wrapper for any content. It's the
// visual unit used throughout the TUI to group related content (a
// step, a question, a result) and make the screen feel "card-based"
// rather than a wall of text.
//
// A Card has an optional title (rendered inside the top border) and
// content (the body). Width and Focused both affect the border style.
type Card struct {
	Title   string
	Content string
	Width   int  // 0 = auto-size to content; >0 = force this width
	Focused bool // when true, border is brand purple + bold
	Accent  bool // when true, render with brand-accent top border accent
}

// View renders the card. The result is a single multi-line string.
//
// Style choices:
//   - Rounded border (lipgloss.RoundedBorder) — softer than square,
//     which is what the spec asked for.
//   - 1 row / 2 cols padding inside the box — content gets breathing
//     room without eating the screen.
//   - Focused cards use the brand purple border (and bold) to make
//     them stand out — this is what we'll use for the "current step"
//     card on the onboarding screen.
func (c Card) View() string {
	style := C.BoxPadding
	if c.Focused {
		style = style.
			BorderForeground(lipgloss.Color(ColorBrand)).
			Bold(true)
	} else {
		style = style.BorderForeground(lipgloss.Color(ColorBorder))
	}

	// Compose the inner content. We place the title above the body
	// content with a margin (handled by lipgloss in Title style),
	// then put the whole thing inside the bordered box.
	var inner strings.Builder
	if c.Title != "" {
		inner.WriteString(C.Title.Render(c.Title))
		inner.WriteString("\n")
	}
	inner.WriteString(c.Content)

	rendered := style.Render(inner.String())

	// Apply width constraint if set. lipgloss handles the truncation
	// / padding gracefully — the content inside stays at its natural
	// width and the box grows to fit.
	if c.Width > 0 {
		return lipgloss.NewStyle().Width(c.Width).Render(rendered)
	}
	return rendered
}

// NewCard constructs a card with a title and content.
func NewCard(title, content string) Card {
	return Card{Title: title, Content: content}
}

// FocusedCard is shorthand for an actively-focused card.
func FocusedCard(title, content string) Card {
	c := NewCard(title, content)
	c.Focused = true
	return c
}

// CenterIn places the rendered card horizontally centered within the
// given terminal width. Useful for screens narrower than 80 cols
// where the card would otherwise feel crammed against the left edge.
func CenterIn(content string, width int) string {
	if width <= 0 {
		return content
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}
