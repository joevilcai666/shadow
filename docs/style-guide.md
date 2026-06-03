# Shadow Visual Style Guide

> Single source of truth for Shadow's TUI look and feel.
> Anything that touches the terminal should reach for tokens here
> before inventing new ones. If you change a color or spacing
> constant, change it here **and** in the code, in the same commit.

---

## 1. Brand identity

Shadow is a personal memory layer for AI coding agents. The visual
language says two things at once:

- **Calm and trustworthy** — you correct once, the agent remembers.
  No drama, no alarm. Cool purples and blues, not fire-engine red.
- **Engineered, not toy** — the screen has structure: a banner that
  anchors the brand, a pipeline that shows progress, cards that
  group decisions. Every pixel earns its place.

Reference points: claude code TUI, hermes agent TUI, charm.sh
ecosystem defaults. Rounded borders, generous padding, no
ASCII-art-for-its-own-sake.

---

## 2. Color palette

All hex values are real, terminal-safe, and tuned for 256-color
displays. The single source of truth lives in
`internal/daemon/styles.go` (and the mirror in
`internal/daemon/components/styles.go` — see §10 about the split).

| Token            | Hex       | RGB              | Role                                                        |
|------------------|-----------|------------------|-------------------------------------------------------------|
| `ColorBrand`     | `#A855F7` | 168, 85, 247     | Primary purple. Titles, brand wordmark, current step dot.   |
| `ColorAccent`    | `#818CF8` | 129, 140, 248    | Secondary blue. Slogan, links, secondary highlights.         |
| `ColorSuccess`   | `#22C55E` | 34, 197, 94      | Checkmarks (✓), done-state indicators, positive confirmations. |
| `ColorWarning`   | `#EAB308` | 234, 179, 8      | Soft alerts, "caution" without panic.                       |
| `ColorError`     | `#EF4444` | 239, 68, 68      | Error marks (✗), fail-state.                                |
| `ColorDim`       | `#6B7280` | 107, 114, 128    | Non-interactive hints, secondary labels, pending steps.     |
| `ColorBorder`    | `#6D28D9` | 109, 40, 217     | Box borders on unfocused cards (deep purple).               |
| `ColorBg`        | `#1E1B4B` | 30, 27, 75       | Optional deep-purple background tint (rare; mostly unused). |

### 2.1 Gradient

The banner wordmark and the slogan use a **left-to-right gradient**
sweeping from `ColorBrand` (left) to `ColorAccent` (right), one
character at a time. The gradient is interpolated linearly in RGB
space.

```
#A855F7  #9A65F7  #8B76F8  #7D80F8  #6E8BF8  #5F95F8  #569FF8  #818CF8
   |--------|--------|--------|--------|--------|--------|
 start                                              end
```

Implementation: `components.gradientColor(t float64)` in
`internal/daemon/components/banner.go`.

### 2.2 Semantic color mapping

Use the **role**, not the hex, when styling:

| When you want to say...          | Use          |
|----------------------------------|--------------|
| "this is the brand" / "I'm here" | `Brand`      |
| "clickable / secondary action"   | `Accent`     |
| "good news / all clear"          | `Success`    |
| "be careful, not broken"         | `Warning`    |
| "broken, needs attention"        | `Error`      |
| "non-interactive helper text"    | `Dim`        |
| "unfocused card edge"            | `Border`     |

If a use case doesn't fit one of these rows, add a row before
adding a new color.

---

## 3. Typography & weight

The terminal renders one weight of text. We fake the rest with
lipgloss flags.

| Style name         | lipgloss flags                         | Used for                       |
|--------------------|----------------------------------------|--------------------------------|
| `Brand`            | `Bold(true).Foreground(ColorBrand)`    | Brand wordmark, titles          |
| `Accent`           | `Foreground(ColorAccent)`              | Slogan, links, secondary       |
| `Bold`             | `Bold(true)`                            | Emphasized text, no color      |
| `Dim`              | `Foreground(ColorDim)`                  | Helper text, hints, secondary  |
| `Success`          | `Foreground(ColorSuccess)`              | ✓ marks, positive state        |
| `Warning`          | `Foreground(ColorWarning)`              | Caution, soft alerts           |
| `Error`            | `Foreground(ColorError)`                | ✗ marks, fail state            |
| `Title` (in cards) | `Bold(true).Foreground(ColorBrand).MarginBottom(1)` | Card section title |
| `Subtitle`         | `Italic(true).Foreground(ColorDim)`     | Muted subtitle                 |

Italic in terminals is unreliable — the `Subtitle` style exists but
is currently unused; prefer `Dim` for subtle secondary text.

---

## 4. Spacing

| Where                              | Value                  | lipgloss API                |
|------------------------------------|------------------------|-----------------------------|
| Card internal padding              | 1 row × 2 cols         | `Padding(1, 2)`             |
| Vertical gap between banner and pipeline | one empty line   | `"\n"`                      |
| Vertical gap between pipeline indicators and labels | one newline | `"\n"` in `Pipeline.View` |
| Vertical gap between sections inside a card | one empty line  | `"\n"`                      |
| Horizontal margin from terminal edge | 0 — let the card breathe naturally | (no margin) |

**Rule of thumb:** the screen should feel like an 80-col document,
not a phone screen. If you're reaching for a margin or gutter, you
probably need a card instead.

---

## 5. Borders

We use **one** border style: `lipgloss.RoundedBorder()`. Square
borders feel too utilitarian; ASCII `+---+` borders feel too retro.

| Card state   | Border color   | Weight | When                          |
|--------------|----------------|--------|-------------------------------|
| Unfocused    | `ColorBorder`  | normal | Default, every passive card.  |
| Focused      | `ColorBrand`   | bold   | The current step's card, the active selection. |

The focused border is the **only** place we use bold + brand color
on a frame — it's the strongest "look at me" signal in the system,
and we use it sparingly.

---

## 6. Layout: the three pillars

Every Shadow TUI screen is composed of three regions, top to bottom.

### 6.1 Banner (top)

- ASCII art wordmark "SHADOW" (figlet "ANSI Shadow" style, 5 rows tall).
- Below it, the slogan "correct once, remember everywhere." in
  accent blue.
- Total visual weight: heavy. It's the only ornament in the system.
- Optional fade-in: 1 second, 20ms per frame. Suppressed in non-TTY
  contexts (CI, snapshots, pipes).

### 6.2 Pipeline (below banner)

- Horizontal step indicator, two lines:
  - Line 1: `●━━━━━●━━━━━○━━━━━○`  (indicators + connectors)
  - Line 2: `Daemon  Privacy  Agents  Memory`  (labels, centered under indicators)
- Cell width auto-computed from the longest label.
- Indicator states:
  - `●`  — current step, brand purple, bold label
  - `✓`  — done, success green
  - `○`  — pending, dim
- Connector: `━━━━━` in `ColorBorder` (deep purple).
- Width budget: 60 col minimum, 100 col target. Falls back to a
  compact `[N/M] …` form below 40 cols.

### 6.3 Card (below pipeline)

- One card per "thing the user is looking at right now" — usually
  the active step's content.
- Title (optional, brand purple, bold) + body.
- Rounded border, 1×2 padding.
- Focused when it's the active step; otherwise unfocused.

### 6.4 Help footer (bottom, optional)

- One line of `dim` text listing key bindings: `↑↓ move · Space toggle · Enter confirm · b back · q quit`.
- Always at the bottom, never above a card.

---

## 7. Animation

The TUI is mostly static — animation is reserved for moments that
benefit from motion.

| Animation       | Trigger                              | Spec                                           |
|-----------------|--------------------------------------|------------------------------------------------|
| Banner fade-in  | First time banner renders            | 1.0s total, 20ms per frame, t=0→1 progress      |
| Spinner         | Any loading state                    | Standard bubbletea spinner, ~80ms per frame     |
| Step transition | Enter advances a step                | Instant (no animation) — clarity over delight   |
| Error flash     | Validation failure                    | Brief red border accent (planned, not yet built) |

**Rule:** if an animation doesn't communicate state, cut it. We
don't animate things for their own sake.

### 7.1 TTY detection

The banner fade-in is suppressed in non-TTY contexts (CI, piped
output, snapshot tests) because ANSI cursor control becomes noise.
Detection helper: `components.IsTTY()` (stat-based, not lib use).

---

## 8. Component inventory

| Component     | File                                  | When to use                          |
|---------------|---------------------------------------|--------------------------------------|
| `Banner`      | `internal/daemon/components/banner.go` | Top of every onboarding / status screen. |
| `Pipeline`    | `internal/daemon/components/pipeline.go` | Below the banner, shows overall progress. |
| `Card`        | `internal/daemon/components/card.go`  | The workhorse. Wrap any block of content. |
| `Spinner`     | `internal/daemon/tui.go`              | Loading state, async work.            |
| `CheckboxList`| `internal/daemon/tui.go`              | Multi-select (agent selection, etc.). |

All components are package-level constructors, not styles. They
take data and return strings. They are independently testable.

---

## 9. Examples

### 9.1 A step screen

```go
banner := components.NewBanner().RenderStatic()
pipeline := components.NewPipeline(
    []string{"Daemon Setup", "Privacy & Scope", "Agent Detection", "Initial Memory"},
    1, // current step index
).View()
card := components.FocusedCard(
    "Privacy & Scope",
    "Shadow will:\n"+
        "  "+S().Success.Render("✓")+"  Read project code & agent session logs\n"+
        "  "+S().Success.Render("✓")+"  Write managed rules to agent context files\n"+
        "  "+S().Success.Render("✓")+"  Never store keys, tokens, or credentials\n",
).View()
footer := S().Dim.Render("Enter accept · a advanced · b back")
```

The three regions are vertically stacked with one empty line
between them. The card is the only focused element.

### 9.2 Adding a new color

Don't. First check §2.2 — there's a role for nearly every use case.
If you genuinely need a new color, do all of the following in the
same commit:

1. Add it to `internal/daemon/styles.go` **and**
   `internal/daemon/components/styles.go` (the mirror — see §10).
2. Add a row to the table in §2.
3. Add a role in §2.2.
4. Add a usage example in §9.

### 9.3 Adding a new component

1. Add it to `internal/daemon/components/<name>.go`.
2. Use only the colors from §2 and the styles from §3.
3. Write a unit test that renders at least one state and asserts
   the output contains the expected tokens (icon, label, etc.).
4. Update the inventory in §8.

---

## 10. Why are colors duplicated in two files?

The parent package `daemon` exposes the style registry
(`S().Brand`, `S().Accent`, …) as the canonical import for
onboarding code. The `components` subpackage needs the same colors
but **cannot import daemon** — that would be a circular import.

Solution: `components/styles.go` mirrors the color constants and
constructs a `C.*` registry. When you change a color:

- Update `internal/daemon/styles.go` (the source of truth).
- Update `internal/daemon/components/styles.go` (the mirror).
- The package-level comment in `components/styles.go` reminds
  you to keep them in sync.

A future refactor could generate one from the other (e.g. embed a
JSON file via `go:embed`). Until then: two files, one source of
truth, one comment, one developer discipline.

---

## 11. Future: web UI tokens

When the localhost web console grows TUI-aware views, the same
hex values should be served as CSS custom properties. The
canonical token file for that consumer is:

```
docs/style-tokens.json   (planned)
```

Until that file exists, copy from the table in §2 and treat it as
the contract. The web UI is a *renderer* of these tokens, not a
co-author.

---

## 12. Accessibility & non-TTY

- **Color is never the only signal.** Every ✓/✗ marker is the actual
  glyph, not just a color. Status messages include words
  ("success", "error").
- **Reduced motion.** The banner fade-in is the only animation on a
  critical path. We honor `NO_COLOR` (lipgloss) and detect TTY
  before applying any animation.
- **Screen readers.** The terminal doesn't have a screen reader
  story; that's by design — TUI is for sighted developers at the
  keyboard. The web console (PRD §3.6) is where accessibility
  becomes mandatory.
- **Width adaptation.** Tested at 80 col (default macOS Terminal)
  and 120 col (wide dev monitor). Below 40 col, the pipeline falls
  back to a `[N/M]` compact form.

---

## 13. Versioning

This guide is **versioned with the code**. When you change a token,
bump:

- `version` in any exported token file (planned `docs/style-tokens.json`)
- A line in `CHANGELOG.md` under the next release

The visual layer is the user-facing API. Don't drift it silently.
