# ADR-0001: Language and TUI framework

- Status: Accepted
- Date: 2026-06-06

## Context

AudioPulse is a terminal music player. We needed to choose an implementation
language and a TUI toolkit that together deliver:

- A polished, styled interface (panels, borders, colour theming).
- Responsive keyboard-driven interaction.
- A clean way to manage asynchronous work (network search, audio events) without
  blocking rendering.
- A single, easily distributed binary.

## Decision

Implement AudioPulse in **Go**, using **Bubble Tea** for the application runtime
and **Lip Gloss** for styling (both from Charm), plus the **Bubbles** widget set
for the text input.

Bubble Tea implements the Elm architecture (Model–Update–View), which gives us a
single source of truth, pure rendering, and a first-class mechanism (`tea.Cmd`)
for moving I/O off the render loop.

## Consequences

**Positive**

- Go produces a self-contained binary with straightforward cross-compilation.
- The Elm architecture makes state transitions explicit and the UI
  unit-testable by driving the model with messages — no terminal needed.
- Lip Gloss centralises theming, so a single palette drives the whole UI.
- Strong concurrency primitives suit the audio/UI bridge.

**Negative / trade-offs**

- The real audio backend requires cgo, which complicates fully static builds and
  adds a native dependency (addressed by [ADR-0004](0004-build-tag-fallback-strategy.md)).
- Lip Gloss `Width()/Height()` semantics (border outside, padding inside)
  required a dedicated sizing helper to get exact layouts.

## Alternatives considered

- **Rust + ratatui** — excellent performance and a single binary, but a slower
  iteration loop and more boilerplate for this size of app.
- **Go + tview/tcell** — capable, but a more imperative widget model; Bubble
  Tea's message-driven design fit the async requirements better and is more
  testable.
- **Python + Textual / Rich** — very fast to prototype and easy to theme, but
  distribution (interpreter + dependencies) is heavier than a Go binary.
