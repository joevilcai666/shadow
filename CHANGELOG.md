# Changelog

All notable changes to Shadow will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **TUI visual redesign.** New layered component architecture: ASCII
  art SHADOW banner (purple→blue gradient), horizontal step pipeline
  (●━━○━━○━━○), rounded-border cards, and a unified help footer.
  The new components live in `internal/daemon/components/` and are
  unit-tested in isolation: `banner_test.go`, `pipeline_test.go`,
  `card_test.go`.
- **2-question onboarding preferences.** A new `Questionnaire`
  component (`internal/daemon/questionnaire.go`) is shown after the
  initial scan. It asks two questions (code style + auto-apply low
  risk) with radio buttons, ↑↓ navigation, Enter to confirm, and `s`
  to skip. The answers drive apply-phase behavior (auto-apply only
  fires when the user opted in).
- **Real adapter writes during onboarding.** The onboarding flow now
  opens the database, loads active rules, and writes them to the
  adapters backing each selected agent (Claude Code, Cursor, Codex).
  Both global (`~/`) and project (`./`) scopes are written. Each
  managed file uses the `>>> shadow managed >>>` block markers so
  hand-written content is preserved and a future `uninstall
  --clean-blocks` can strip just the managed region.
- **Integration test** `internal/daemon/integration_test.go` drives
  the full onboarding state machine end-to-end (steps 1→2→3→4a→4b→
  4c→4d) and verifies that an on-disk `CLAUDE.md` is produced with
  the managed block containing the active rule.
- **Screenshot helper test** `internal/daemon/onboarding_screenshot_test.go`
  writes a plain-text "screenshot" of every TUI state to
  `$SHADOW_SCREENSHOT_DIR`, stripping ANSI codes so the layout is
  human-readable in any text editor.
- **E2E test** `internal/daemon/e2e_test.go` was extended to verify
  that hand-written content in an existing `CLAUDE.md` is preserved
  when the apply phase runs, and that the managed block contains the
  active rule.

### Changed

- **State machine fix.** The onboarding `daemonCheckMsg` handler no
  longer skips past step 2 (privacy) and step 3 (agent selection)
  when the daemon is already running. The user must still explicitly
  accept privacy defaults and select agents. (Earlier code skipped
  both, which violated the spec's "user must opt in" requirement.)
- **Sub-step rendering bug fix.** `OnboardingModel.View()` for the
  questionnaire sub-step was rendering `Questionnaire{}.View()` (a
  freshly-constructed zero-value struct) instead of
  `m.questionnaire.View()`. The user's selections were being
  ignored on screen. Now the model field is the single source of
  truth, and the View() matches what the Update() mutated.
- **Back-key (`b`) supported everywhere except loading.** Pressing
  `b` on any non-loading step goes back one step and clears any
  transient error. It does nothing during loading (the spinner
  owns the input pipeline).
- **`.env*` filter is now in the default protections list.** Step 2
  (Privacy & Scope) now explicitly lists "Block: API keys, tokens,
  .env files" and "Exclude: node_modules, .git, dist, .env*" so the
  user sees what is being filtered.
- **Error handling refined.** Step-level errors render in a red
  card with `r` to retry and `q` to quit. The legacy 1-line error
  banner was replaced by a Card so the layout stays consistent with
  the other steps.
- **`go.mod` tidied.** `bubbletea`, `lipgloss`, `fsnotify`,
  `gorilla/mux`, `gorilla/websocket`, `yaml.v3`, and `sqlite` are
  now declared as direct dependencies (they were indirect before,
  which caused confusion on first import).

### Notes for reviewers

- The `tui-visual` track lives in commits
  `4722353 feat(tui): redesign TUI visuals with banner, pipeline, cards`.
- The `onboarding-logic` track lives in commits
  `5b896c8 fix(onboarding): real adapter writes, state machine,
   questionnaire, env filter` and
  `cc5a7e2 fix(onboarding): add questionnaire tests + fix
   subStepQuestion rendering bug`.
- This Unreleased section will be tagged as the next minor version
  when we cut a release.

## [0.1.0-dev] — initial MVP

First end-to-end MVP: capture engine, rule distillation, SQLite
storage, three adapter implementations (Claude Code, Cursor,
Codex), HTTP API, web console, and launchd-based daemon lifecycle.
See git log for the full commit history.
