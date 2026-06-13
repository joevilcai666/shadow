# Shadow PRD Evaluation And Performance Pass

Date: 2026-06-14

## Product Read

Shadow is a local-first memory layer for coding agents. Its MVP promise is:
correct an agent once, convert the correction into a reusable rule, and make
that rule available to other local coding agents through their native context
files.

The PRD north-star metric is reducing repeated-error recurrence. In the current
implementation this is approximated by rule-hit tracking, adapter sync events,
and the dashboard hit-rate summary.

## PRD Coverage

| PRD Area | Current Status | Evidence |
| --- | --- | --- |
| Local daemon + API | Implemented | `shadow serve`, daemon IPC, localhost-only HTTP API, WebSocket hub |
| Onboarding | Partially implemented | `shadow start` launches the Bubble Tea onboarding model after service start |
| Capture | Implemented for core MVP paths | Claude Code, Codex, Cursor parsers, git hook signal ingestion, privacy filtering |
| Distill / crystallization | Implemented and wired | daemon starts a periodic distill loop, explicit signals become candidate rules |
| Rule storage | Implemented | SQLite rules, sources, versions, events, projects, user memories |
| Cross-agent output | Implemented | Claude Code, Cursor, Codex, OpenClaw, and Copilot adapters with managed-block writes for rules and user memories |
| Safe sync preview | Implemented | `shadow sync --dry-run` reports adapter targets and pending managed-block changes without writing files |
| Review flow | Implemented | `shadow review`, `/api/rules?status=candidate`, batch approve/reject |
| Rule management UI/API | Implemented | CRUD, timeline, events, versions, rollback, config, adapters, conflicts |
| Effectiveness dashboard | Implemented as proxy | hit-rate endpoint, repeated-hit recurrence proxy, rule-hit events, adapter sync status, memory map |
| Privacy boundary | Implemented | deny-pattern checks before rule/memory/source persistence |
| Local export | Implemented | `GET /api/export` returns a local JSON package with rules and user memories |
| Uninstall rollback | Mostly implemented | `shadow uninstall --clean-blocks` removes managed blocks for primary adapters |

## Remaining Product Gaps

- The PRD's exact user-perceived repeated-error recurrence metric is not
  directly measurable yet. Current hit-rate and repeated-hit rule rate are
  useful local proxies, not a replacement for explicit user feedback.
- Web onboarding and Aha demo exist as UI surfaces, but should be tested with
  real seeded candidate rules in the next product QA pass.
- Capture is broad enough for MVP validation, but more implicit signals
  (accept/reject feedback, repeated prompt detection, CI-failure corrections)
  remain experimental.
- Memory map graph view is PRD-adjacent Pro/visualization work; it should stay
  fast, but it should not displace the rules/review workflow.

## PRD Alignment Changes In Follow-up Iteration

- Added the OpenClaw adapter required by the Shadow_qt MVP adapter contract:
  project rules write to `OPENCLAW.md`, global rules write to `~/OPENCLAW.md`.
- Added OpenClaw to default config, daemon sync, dashboard adapter status,
  adapter toggling, onboarding target display, health sync status, task
  injection, and uninstall managed-block cleanup.
- Added focused regression coverage for OpenClaw adapter writes/removal, default
  config enablement, API listing/toggle persistence, task injection,
  onboarding display, and health status.
- Added a local export package endpoint so users can export their rules and
  user memories without cloud sync, matching the PRD's "visible, deletable,
  exportable" data boundary and MVP export requirement.
- Added explicit `shadow sync --dry-run` support so users can validate which
  agent context files would change before performing a local sync.
- Added rule metadata to managed agent context blocks: scope, tags, trigger
  context, confidence, version, and rule id are now visible alongside the rule
  text, matching the Shadow_qt real-agent interop evidence format more closely.
- Synced user memories into agent context files as always-active local context,
  so `shadow store-memory` now matches its "shared across all agents" promise
  instead of only storing/exporting the memory.
- Added regression coverage for user-memory privacy rejection, verifying that
  sensitive memory content is rejected before persistence.
- User-memory create/delete now triggers adapter sync so always-active personal
  context is written to native agent files immediately after changes.
- `shadow store-memory` now rejects invalid `--scope` values locally, preventing
  accidental global memories when a user mistypes the project/global switch.
- Added a web Memories view that lists user memories, supports local search,
  deletes memories, and downloads the local export package from the console.
- Added a repeated-hit recurrence proxy to `/api/stats/hit-rate` and the
  Memory Map HUD: active rules hit more than once in the last 7 days divided by
  active rules.

## Performance Changes In This Pass

- `/api/dashboard/map` now reads source snippets and evidence agents in batch
  instead of querying sources/events once per rule.
- Map edge generation reuses precomputed tag sets during pairwise scans,
  reducing allocations in the hottest loop.
- The map API keeps its rule limit at 300 and caps structure/whisper edges per
  node so a shared tag cannot produce an explosive complete graph.
- Adapter sync now loads active project rules once and groups them by project
  path, avoiding a repeated project-rule query for every adapter/project pair.
- Task context extraction now uses stable O(n log n) ranking instead of a
  hand-written O(n^2) sort while preserving equal-score rule order.
- The React memory map computes layout in a Web Worker and defaults to a reduced
  edge density so initial render prioritizes signal and structure edges.
- Embedded static assets were rebuilt from a clean `web/dist`, removing stale
  hashed files from previous builds.

## Verification Baseline

Run after this pass:

- `go test ./internal/storage -run "Test(SourceRepoEvidenceByRuleIDs|EventRepoAgentsByRuleIDs)"`
- `go test ./internal/server -run "TestDashboardMap|TestConflictPairsEndpoint"`
- `npm run build` from `web`
- `go test ./internal/adapter ./internal/config ./internal/daemon ./internal/server`

Full verification should include `go test ./...`, `go vet ./...`, frontend lint,
and a browser smoke test against the rebuilt local console.
