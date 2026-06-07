# ADR-0007: Spotlight-style search overlay

- Status: Accepted
- Date: 2026-06-07

## Context

Search initially lived inline in the top bar. The desired experience was a
**macOS-Spotlight-style** search: a floating box centered over the UI, with
live results as you type, keyboard-navigable, dismissible with Esc.

Two questions: how to *float* a box over the existing TUI (Bubble Tea/Lip Gloss
render a single string, with no built-in layer compositing), and how to search
*live* without hammering the Spotify API on every keystroke.

## Decision

**Compositing.** Render the normal frame, then splice the search box onto it at a
centered cell position using ANSI-aware string cutting from
`github.com/charmbracelet/x/ansi` (`ansi.Cut`, `ansi.StringWidth`). A small
`overlay(base, box, x, y)` helper replaces the box's columns on each affected
row, preserving the surrounding UI on the left/right. A `solidify` helper forces
an opaque background on the box (re-establishing it after every SGR reset) so it
isn't see-through where inner styles reset the background — the same technique as
`fillBG`.

**Live search with debounce.** Each keystroke increments a `searchGen` counter
and schedules a `tea.Tick` (~280 ms). Only when the tick's captured generation
still equals the current one (i.e. typing has paused) is a search issued.
Results carry their generation and are applied only if still current, so
in-flight responses for stale queries are dropped.

`/` opens the overlay; `↑`/`↓` move the selection; `enter` plays the selection
(and loads the results into the center panel); `esc` closes it.

## Consequences

**Positive**
- A genuine floating overlay over the live UI, matching the Spotlight feel.
- Debouncing keeps the API call rate sane while still feeling instant.
- The `overlay`/`solidify` helpers are reusable for future modals.

**Negative / trade-offs**
- Manual ANSI compositing is more fragile than a native layer system; it relies
  on `x/ansi` cutting styles correctly at the seams.
- Adds a direct dependency on `github.com/charmbracelet/x/ansi` (already present
  transitively via Bubble Tea).

## Alternatives considered
- **Full-screen modal** (dark background, centered box) — simpler, but loses the
  "floating over the UI" Spotlight feel.
- **A third-party overlay library** — avoided to keep the dependency surface
  small; the compositing needed here is a few lines.
