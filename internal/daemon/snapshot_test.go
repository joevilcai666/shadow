package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/joevilcai666/shadow/internal/daemon/components"
)

// stripANSI removes ANSI escape sequences for clean terminal output
// in a non-TTY context. The snapshot is what the user sees in their
// terminal WITHOUT color (e.g. when piping to a file).
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case inEscape:
			if r == 'm' {
				inEscape = false
			}
		case r == 0x1b:
			inEscape = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// recenter re-anchors a multi-line string at the given width. We
// use this to simulate how the View() output would look in a
// terminal of arbitrary width (the production View() hardcodes
// PlaceHorizontal(100, Center, ...) for now).
func recenter(s string, width int) string {
	lines := strings.Split(s, "\n")
	var out strings.Builder
	for _, line := range lines {
		visible := lipgloss.Width(line)
		if visible >= width {
			out.WriteString(line)
		} else {
			left := (width - visible) / 2
			out.WriteString(strings.Repeat(" ", left))
			out.WriteString(line)
		}
		out.WriteString("\n")
	}
	return out.String()
}

// TestSnapshot writes ASCII renderings of the new TUI to
// /tmp/shadow-tui-snapshot.txt so they can be embedded in the
// deliverable.md. This test is not gated — it always runs.
//
// To inspect: `cat /tmp/shadow-tui-snapshot.txt`.
func TestSnapshot(t *testing.T) {
	var out strings.Builder

	w := func(format string, a ...interface{}) {
		out.WriteString(fmt.Sprintf(format, a...))
	}

	w("=== BANNER (static, full reveal, no color) ===\n")
	banner := components.NewBanner()
	w("%s\n", stripANSI(banner.RenderStatic()))

	w("\n=== PIPELINE — current=0 (first step) ===\n")
	p := components.NewPipeline([]string{"Daemon Setup", "Privacy & Scope", "Agent Detection", "Initial Memory"}, 0)
	w("%s\n", stripANSI(p.View()))

	w("\n=== PIPELINE — current=2 (third step) ===\n")
	p = components.NewPipeline([]string{"Daemon Setup", "Privacy & Scope", "Agent Detection", "Initial Memory"}, 2)
	w("%s\n", stripANSI(p.View()))

	w("\n=== PIPELINE — current=3 (last step, all done) ===\n")
	p = components.NewPipeline([]string{"Daemon Setup", "Privacy & Scope", "Agent Detection", "Initial Memory"}, 3)
	w("%s\n", stripANSI(p.View()))

	w("\n=== CARD — focused (active step) ===\n")
	c := components.FocusedCard("Welcome to Shadow!", "This is the body.\nSecond line of body.")
	w("%s\n", stripANSI(c.View()))

	w("\n=== CARD — unfocused (inactive context) ===\n")
	c = components.NewCard("Background info", "Quiet content the user can glance at.")
	w("%s\n", stripANSI(c.View()))

	w("\n=== ONBOARDING VIEW — step 1 (welcome) @ 80 cols ===\n")
	m := NewOnboardingModel("1.0.0")
	m.step = 1
	w("%s\n", stripANSI(recenter(m.View(), 80)))

	w("\n=== ONBOARDING VIEW — step 3 (agent selection, 2 detected) @ 80 cols ===\n")
	m = NewOnboardingModel("1.0.0")
	m.step = 3
	m.agentsFound = []string{"Claude Code", "Cursor"}
	w("%s\n", stripANSI(recenter(m.View(), 80)))

	w("\n=== ONBOARDING VIEW — step 3 @ 120 cols (wider) ===\n")
	m = NewOnboardingModel("1.0.0")
	m.step = 3
	m.agentsFound = []string{"Claude Code", "Cursor"}
	w("%s\n", stripANSI(recenter(m.View(), 120)))

	w("\n=== ONBOARDING VIEW — step 4 (done) @ 80 cols ===\n")
	m = NewOnboardingModel("1.0.0")
	m.step = 4
	m.done = true
	m.rulesGenerated = 7
	m.importedFiles = []string{"CLAUDE.md", ".cursorrules"}
	m.scanFacts = []string{"Detected Go module", "Has tests", "Uses lipgloss"}
	w("%s\n", stripANSI(recenter(m.View(), 80)))

	w("\n=== ONBOARDING VIEW — error state @ 80 cols ===\n")
	m = NewOnboardingModel("1.0.0")
	m.step = 2
	m.err = fmt.Errorf("database not writable")
	w("%s\n", stripANSI(recenter(m.View(), 80)))

	// Write to /tmp so it's inspectable but not committed.
	tmpFile := filepath.Join(os.TempDir(), "shadow-tui-snapshot.txt")
	if err := os.WriteFile(tmpFile, []byte(out.String()), 0o644); err != nil {
		t.Logf("snapshot write failed (non-fatal): %v", err)
	} else {
		t.Logf("snapshot written to %s (%d bytes)", tmpFile, out.Len())
	}
}
