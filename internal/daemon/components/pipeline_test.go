package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestPipeline_AllStates verifies the horizontal step pipeline renders
// the four canonical states (done, current, pending) and that the
// labels appear beneath their indicators.
//
// We assert on visible cell width and the presence of the expected
// icons (●, ✓, ○) and the connector (━━━━━). We do not assert on raw
// ANSI sequences — those are exercised by lipgloss itself.
func TestPipeline_AllStates(t *testing.T) {
	p := NewPipeline([]string{"Setup", "Privacy", "Agents", "Memory"}, 1)
	out := p.View()
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("pipeline should render at least 2 lines, got %d: %q", len(lines), out)
	}

	ind := lines[0]
	lbl := lines[1]

	// Step 0 is Done → ✓
	if !strings.Contains(ind, "✓") {
		t.Errorf("expected done indicator ✓ in: %q", ind)
	}
	// Step 1 is Current → ●
	if !strings.Contains(ind, "●") {
		t.Errorf("expected current indicator ● in: %q", ind)
	}
	// Steps 2 and 3 are Pending → ○ (lipgloss may pad to align cells)
	if !strings.Contains(ind, "○") {
		t.Errorf("expected pending indicator ○ in: %q", ind)
	}
	// Connector must appear at least once between steps.
	if !strings.Contains(ind, "━") {
		t.Errorf("expected connector ━ in: %q", ind)
	}

	// Labels must appear on the second line.
	for _, want := range []string{"Setup", "Privacy", "Agents", "Memory"} {
		if !strings.Contains(lbl, want) {
			t.Errorf("expected label %q in: %q", want, lbl)
		}
	}

	// Indicator and label lines must have the same visual width so
	// labels sit under their indicators.
	if w1, w2 := lipgloss.Width(ind), lipgloss.Width(lbl); w1 != w2 {
		t.Errorf("indicator and label lines should have same width; got %d vs %d\nind=%q\nlbl=%q", w1, w2, ind, lbl)
	}
}

// TestPipeline_CurrentAtBoundary exercises the current=0 case (no done
// steps, just one active and the rest pending) and current=last case
// (all done, none pending) — both are easy off-by-one traps.
func TestPipeline_CurrentAtBoundary(t *testing.T) {
	t.Run("first_step", func(t *testing.T) {
		p := NewPipeline([]string{"A", "B", "C"}, 0)
		out := p.View()
		ind := strings.Split(out, "\n")[0]
		if !strings.Contains(ind, "●") {
			t.Errorf("first step should be current (●), got: %q", ind)
		}
		if strings.Contains(ind, "✓") {
			t.Errorf("no done step should appear at current=0, got: %q", ind)
		}
	})

	t.Run("last_step", func(t *testing.T) {
		p := NewPipeline([]string{"A", "B", "C"}, 2)
		out := p.View()
		ind := strings.Split(out, "\n")[0]
		if !strings.Contains(ind, "✓") {
			t.Errorf("last step should be current via done chain, got: %q", ind)
		}
		if strings.Contains(ind, "○") {
			t.Errorf("no pending step should appear at current=last, got: %q", ind)
		}
	})
}

// TestPipeline_EmptyLabels — degenerate input should not crash.
func TestPipeline_EmptyLabels(t *testing.T) {
	p := NewPipeline(nil, 0)
	if out := p.View(); out != "" {
		t.Errorf("empty pipeline should return empty string, got %q", out)
	}
}
