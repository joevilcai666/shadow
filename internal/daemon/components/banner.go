// Package components holds reusable bubbletea/lipgloss building blocks
// for the Shadow TUI. Each file is one self-contained visual component —
// it can be unit-tested, swapped, or re-used across screens.
package components

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// shadowASCII is the SHADOW wordmark rendered in figlet's "ANSI Shadow"
// style — hand-shaped for compactness and a strong vertical axis. Each
// line is one row of the glyph, padded to a uniform visual width.
var shadowASCII = []string{
	" ██████  ██   ██  █████   ██████  ██████   ██████  ██     ██ ",
	"██       ██   ██ ██   ██ ██    ██ ██   ██ ██    ██ ██     ██ ",
	"███████  ███████ ███████ ██    ██ ██   ██ ██    ██ ██  █  ██ ",
	"     ██ ██   ██ ██   ██ ██    ██ ██   ██ ██    ██ ██ ███ ██ ",
	" ██████ ██   ██ ██   ██  ██████  ██████   ██████   ███ ███  ",
}

// bannerTickMsg is the tea message that advances the fade-in animation
// by one frame. We do NOT use tea.Tick inline because tests need to
// drive frames deterministically — see Banner.Tick() in tests.
type bannerTickMsg struct{}

// Banner is the Shadow ASCII art title plus a tagline ("correct once,
// remember everywhere") rendered with a left-to-right purple→blue gradient.
//
// The banner can be displayed statically via View(progress float)
// where progress=1.0, or animated via the bubbletea Model interface
// (Init/Update/View) that advances progress 0.0→1.0 over fadeDuration.
type Banner struct {
	fadeDuration time.Duration
	frame        time.Duration // ms per frame; default 20ms per spec
	progress     float64       // 0.0..1.0 — visible character ratio
	done         bool
}

// NewBanner creates a banner with the spec's default timings:
// 20ms per frame, 1 second total fade-in.
func NewBanner() Banner {
	return Banner{
		fadeDuration: time.Second,
		frame:        20 * time.Millisecond,
	}
}

// IsTTY reports whether stdout is a terminal. The fade-in animation
// is suppressed in non-TTY contexts (CI, redirects, tests) because
// ANSI cursor control sequences become garbage in those environments.
func IsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// gradientColor blends between the brand purple and accent blue
// according to t (0..1). 0 = pure brand, 1 = pure accent.
//
// We use simple linear interpolation on the RGB channels — lipgloss
// accepts "#RRGGBB" hex strings so we build the result as a string.
// (We avoid importing go-colorful here to keep the dep surface small.)
func gradientColor(t float64) string {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	// brand  #A855F7 = (168, 85, 247)
	// accent #818CF8 = (129, 140, 248)
	br, bg, bb := 168, 85, 247
	ar, ag, ab := 129, 140, 248
	r := int(float64(br) + float64(ar-br)*t)
	g := int(float64(bg) + float64(ag-bg)*t)
	b := int(float64(bb) + float64(ab-bb)*t)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// renderLine styles a single banner line with the horizontal gradient.
// Each character gets a slightly advanced t-value, so the wordmark
// sweeps purple→blue left to right.
//
// In a non-TTY context (CI / piped output) we return the line plain
// — the animation is suppressed to avoid ANSI escape leakage.
func renderLine(line string, progress float64) string {
	if !IsTTY() {
		// In a non-TTY environment we still apply ONE final gradient
		// so screenshots / piped output looks branded. The spec says
		// the animation must not crash — it doesn't say "no color".
		progress = 1.0
	}

	runes := []rune(line)
	n := len(runes)
	if n == 0 {
		return ""
	}

	// We render the line character-by-character so each cell can have
	// its own foreground color. This produces the gradient sweep
	// effect in the final TUI. To make sure partially-revealed frames
	// (progress < 1.0) look coherent, we cap the visible columns.
	visible := int(float64(n) * progress)
	if visible < 0 {
		visible = 0
	}
	if visible > n {
		visible = n
	}

	var b strings.Builder
	for i, r := range runes {
		if i >= visible {
			// unrevealed portion: emit a space at the same width so
			// the box doesn't shrink during the animation. This keeps
			// the banner from "growing" as the reveal completes.
			b.WriteRune(' ')
			continue
		}
		// t for color: full gradient sweep regardless of progress —
		// the gradient is the "branded look", progress is the reveal.
		t := float64(i) / float64(n-1)
		if n == 1 {
			t = 0.5
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(gradientColor(t))).Bold(true)
		b.WriteString(style.Render(string(r)))
	}
	return b.String()
}

// View renders the banner at the given progress (0.0..1.0).
// progress=1.0 means fully revealed; 0.0 means nothing visible.
//
// When the banner is used in a non-animated context (e.g. snapshot
// tests, web preview), call View with progress=1.0 and you get the
// final gradient output.
func (b Banner) View(progress float64) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	var lines []string
	for _, line := range shadowASCII {
		lines = append(lines, renderLine(line, progress))
	}

	// Slogan — smaller, dim, in the accent blue. We use the same
	// renderLine approach for visual consistency (a little italic feel
	// without an italic font, by spacing the slogan out).
	slogan := "  correct once, remember everywhere.  "
	var sb strings.Builder
	for i, r := range slogan {
		t := float64(i) / float64(max(len(slogan)-1, 1))
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(gradientColor(t)))
		sb.WriteString(style.Render(string(r)))
	}
	subtitle := sb.String()

	rendered := strings.Join(lines, "\n")
	return rendered + "\n" + subtitle
}

// RenderStatic returns the fully-revealed banner. Convenience for
// non-animated contexts (tests, web capture, --no-anim flag).
func (b Banner) RenderStatic() string {
	return b.View(1.0)
}

// --- bubbletea Model interface (optional animated usage) ---

// Init starts the fade-in animation tick. Returns nil if not on a TTY
// — the animation is meaningless in non-terminal contexts and would
// emit noisy escape sequences.
func (b Banner) Init() tea.Cmd {
	if !IsTTY() {
		return nil
	}
	return bannerTick(b.frame)
}

func bannerTick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg { return bannerTickMsg{} })
}

// Update advances the animation by one frame.
func (b Banner) Update(msg tea.Msg) (Banner, tea.Cmd) {
	switch msg.(type) {
	case bannerTickMsg:
		if b.done {
			return b, nil
		}
		// Each frame advances progress by frame/fadeDuration. We
		// cap to 1.0 so the final tick clamps cleanly.
		b.progress += float64(b.frame) / float64(b.fadeDuration)
		if b.progress >= 1.0 {
			b.progress = 1.0
			b.done = true
			return b, nil
		}
		return b, bannerTick(b.frame)
	}
	return b, nil
}

// BannerView renders the banner using the internal progress state.
// Use this when driving the banner as a full bubbletea Model.
func (b Banner) BannerView() string {
	return b.View(b.progress)
}

// Tick is a deterministic test hook — it advances the animation by
// exactly one frame regardless of wall-clock time. Tests can call
// this in a loop to simulate the animation in a TTY-mock context.
func (b *Banner) Tick() {
	if b.done {
		return
	}
	b.progress += float64(b.frame) / float64(b.fadeDuration)
	if b.progress >= 1.0 {
		b.progress = 1.0
		b.done = true
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
