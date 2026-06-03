package daemon

import (
	"strings"
	"testing"
)

// TestOnboardingView_AllSteps exercises the new View() of OnboardingModel
// across all 4 steps + error + loading states. It is a smoke test —
// the goal is to catch panics and obvious layout breakage in the
// redesigned View(), not to assert on the exact text content (which
// is informational, not a contract).
//
// We do NOT touch Update / handleEnter / Init. This test only
// exercises the View() function — the "another track" code path.
func TestOnboardingView_AllSteps(t *testing.T) {
	base := NewOnboardingModel("0.0.0-test")

	cases := []struct {
		name string
		mut  func(m *OnboardingModel)
	}{
		{"step1_welcome", func(m *OnboardingModel) { m.step = 1 }},
		{"step2_privacy", func(m *OnboardingModel) { m.step = 2 }},
		{"step3_agents_detected", func(m *OnboardingModel) {
			m.step = 3
			m.agentsFound = []string{"Claude Code", "Cursor"}
		}},
		{"step3_agents_none", func(m *OnboardingModel) {
			m.step = 3
			m.agentsFound = nil
		}},
		{"step4_pending", func(m *OnboardingModel) { m.step = 4 }},
		{"step4_scan_done", func(m *OnboardingModel) {
			m.step = 4
			m.subStep = subStepScanDone
			m.rulesGenerated = 3
			m.importedFiles = []string{"CLAUDE.md"}
			m.scanFacts = []string{"Detected Go module"}
		}},
		{"step4_question", func(m *OnboardingModel) {
			m.step = 4
			m.subStep = subStepQuestion
		}},
		{"step4_applied", func(m *OnboardingModel) {
			m.step = 4
			m.subStep = subStepApplied
			m.done = true
			m.appliedFiles = []string{"CLAUDE.md", ".cursorrules"}
		}},
		{"step4_applying", func(m *OnboardingModel) {
			m.step = 4
			m.subStep = subStepApplying
			m.loading = true
			m.loadingMsg = "applying"
			m.spinner = NewSpinner(m.loadingMsg)
		}},
		{"step4_done", func(m *OnboardingModel) {
			m.step = 4
			m.done = true
			m.rulesGenerated = 5
			m.importedFiles = []string{"CLAUDE.md"}
			m.scanFacts = []string{"Detected Go module", "Has tests"}
		}},
		{"loading", func(m *OnboardingModel) {
			m.step = 2
			m.loading = true
			m.loadingMsg = "scanning"
			// In real flow, handleEnter() initializes the spinner
			// with NewSpinner. The test bypasses handleEnter, so we
			// must do the same initialization here to avoid an
			// empty-frames panic in SpinnerModel.View().
			m.spinner = NewSpinner(m.loadingMsg)
		}},
		{"error", func(m *OnboardingModel) {
			m.step = 2
			m.err = &simpleErr{"simulated error"}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			tc.mut(&m)
			out := m.View()
			if out == "" {
				t.Errorf("View() returned empty for case %q", tc.name)
			}
			// Sanity: the banner must always be present (it sits
			// at the top of every screen).
			if !strings.Contains(out, "correct once, remember everywhere") {
				t.Errorf("View() missing banner slogan for case %q; got:\n%s", tc.name, out)
			}
		})
	}
}

// simpleErr is a tiny error type used to drive the error-state test
// without importing the larger error chains from the codebase.
type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }

// TestOnboardingView_BannerAndPipelineAlwaysPresent is a quick
// regression test: every rendered screen must include the SHADOW
// wordmark and the step pipeline.
func TestOnboardingView_BannerAndPipelineAlwaysPresent(t *testing.T) {
	for step := 1; step <= 4; step++ {
		m := NewOnboardingModel("test")
		m.step = step
		out := m.View()
		if !strings.Contains(out, "correct once, remember everywhere") {
			t.Errorf("step %d: banner slogan missing", step)
		}
		// The pipeline is rendered by the new StepProgress.View() —
		// it must include all 4 step labels.
		for _, want := range []string{"Daemon", "Privacy", "Agent", "Memory"} {
			if !strings.Contains(out, want) {
				t.Errorf("step %d: pipeline label %q missing", step, want)
			}
		}
	}
}
