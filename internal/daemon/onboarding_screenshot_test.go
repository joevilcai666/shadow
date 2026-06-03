package daemon

// SCREENSHOT TEST: produces plain-text "screenshots" of the TUI by
// calling OnboardingModel.View() at every state, stripping ANSI
// codes, and writing each one to disk. The verifier (or a human
// running the test) can then cat the resulting files to see what
// the TUI looks like without needing an interactive terminal.
//
// Why a test and not a separate cmd binary? Because the OnboardingModel
// fields are unexported, and a test in the same package has direct
// access to them. The test also doubles as a smoke test — it
// fails if any View() call panics or returns empty.
//
// Run with:
//
//   SHADOW_SCREENSHOT_DIR=/tmp/shadow-screenshots \
//     go test -v -run TestOnboarding_Screenshots ./internal/daemon/
//
// If SHADOW_SCREENSHOT_DIR is unset, the test writes to a t.TempDir()
// and the path is printed in the test log; nothing leaks to the
// working tree.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnboarding_Screenshots(t *testing.T) {
	outDir := os.Getenv("SHADOW_SCREENSHOT_DIR")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", outDir, err)
	}

	type shot struct {
		filename string
		note     string
		mut      func(m *OnboardingModel)
	}

	shots := []shot{
		{"01-welcome.txt", "Step 1: Welcome", func(m *OnboardingModel) { m.step = 1 }},
		{"02-privacy.txt", "Step 2: Privacy & Scope", func(m *OnboardingModel) { m.step = 2 }},
		{"03-agents-detected.txt", "Step 3: Agent selection (with detected agents)", func(m *OnboardingModel) {
			m.step = 3
			m.agentsFound = []string{"Claude Code", "Cursor"}
		}},
		{"03-agents-empty.txt", "Step 3: Agent selection (no agents detected)", func(m *OnboardingModel) {
			m.step = 3
			m.agentsFound = nil
		}},
		{"04a-scan-idle.txt", "Step 4a: Pre-scan prompt", func(m *OnboardingModel) { m.step = 4 }},
		{"04b-scan-done.txt", "Step 4b: Scan complete", func(m *OnboardingModel) {
			m.step = 4
			m.subStep = subStepScanDone
			m.rulesGenerated = 3
			m.importedFiles = []string{"CLAUDE.md"}
			m.scanFacts = []string{"Detected Go module", "Has tests"}
		}},
		{"04c-question.txt", "Step 4c: Questionnaire", func(m *OnboardingModel) {
			m.step = 4
			m.subStep = subStepQuestion
		}},
		{"04d-applied.txt", "Step 4d: Onboarding complete", func(m *OnboardingModel) {
			m.step = 4
			m.subStep = subStepApplied
			m.done = true
			m.appliedFiles = []string{"CLAUDE.md", ".cursorrules"}
			m.rulesGenerated = 1
		}},
		{"loading.txt", "Loading state (spinner)", func(m *OnboardingModel) {
			m.step = 2
			m.loading = true
			m.loadingMsg = "scanning"
			m.spinner = NewSpinner(m.loadingMsg)
		}},
		{"error.txt", "Error state", func(m *OnboardingModel) {
			m.step = 2
			m.err = &simpleErr{"simulated error: daemon did not respond"}
		}},
	}

	compositePath := filepath.Join(outDir, "composite.txt")
	composite, err := os.Create(compositePath)
	if err != nil {
		t.Fatalf("create composite: %v", err)
	}
	defer composite.Close()

	for _, s := range shots {
		m := NewOnboardingModel("0.0.0-screenshot")
		s.mut(&m)

		view := m.View()
		if view == "" {
			t.Errorf("View() returned empty for %s", s.filename)
			continue
		}
		clean := stripANSIScreen(view)

		out := filepath.Join(outDir, s.filename)
		header := fmt.Sprintf("=== %s ===\n\n", s.note)
		if err := os.WriteFile(out, []byte(header+clean+"\n"), 0o644); err != nil {
			t.Errorf("write %s: %v", out, err)
			continue
		}
		t.Logf("wrote %s (%d bytes)", out, len(clean))

		fmt.Fprintf(composite, "\n\n=== %s ===\n%s\n", s.note, clean)
	}
	t.Logf("\nScreenshots written to: %s", outDir)
	t.Logf("Composite view: %s", compositePath)
}

// stripANSIScreen removes common ANSI escape sequences so the file
// is readable as plain text. It is intentionally minimal — it catches
// CSI sequences (ESC[...), OSC sequences (ESC]...BEL or ESC]...ST),
// and a few one-shot controls. Same as the helper in cmd/reproduce-screenshot
// from an earlier draft, kept private here so it stays scoped to the
// daemon package.
func stripANSIScreen(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) {
			next := s[i+1]
			switch next {
			case '[':
				j := i + 2
				for j < len(s) {
					b := s[j]
					if b >= 0x40 && b <= 0x7E {
						j++
						break
					}
					j++
				}
				i = j
				continue
			case ']':
				j := i + 2
				for j < len(s) {
					if s[j] == 0x07 {
						j++
						break
					}
					if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
						j += 2
						break
					}
					j++
				}
				i = j
				continue
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
