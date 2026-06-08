# ADR-0008: Synthesized (non-FFT) audio visualizer

- Status: Accepted
- Date: 2026-06-08

## Context

A CAVA-style spectrum visualizer was wanted on the right side, below Now Playing.
CAVA is a real audio spectrum: it captures PCM, runs an FFT, and draws frequency
bars that react to the actual sound.

The blocker is the AudioPulse architecture (see ADR-0005). librespot is a
separate process that decodes and plays audio **straight to ALSA → PulseAudio**.
AudioPulse only *controls* librespot through the Web API and reads now-playing
metadata — it never receives the decoded PCM samples. There is no audio buffer in
the Go process to run an FFT over.

A true spectrum would therefore require separately capturing the audio, e.g.
reading the PulseAudio **monitor source** of the output sink (`parec` or the
PULSE API) on a background goroutine, windowing it, and running an FFT. In this
WSL2 → WSLg PulseAudio setup that path is fragile (monitor-source availability,
extra runtime dependency, sample-rate/format handling, an FFT dependency) and is
a sizeable feature in its own right.

## Decision

Ship a **synthesized** visualizer now, and leave the door open to real capture
later.

- The spectrum is generated procedurally in `vizLevels(n)`
  (`internal/ui/spotify_model.go`): a few layered sine waves advanced by a frame
  counter (`vizFrame`), under a bass-weighted envelope so it reads like a real
  spectrum. While paused it returns a flat low baseline.
- `renderBars` (`internal/ui/spotify_view.go`) maps the levels onto eighth-height
  Unicode block runes (`▁`–`█`) with a bottom-bright → top-pale green gradient.
- Animation is driven by a dedicated `vizTickMsg` at ~8 fps (`vizFrameRate`),
  scheduled **only** while `vizActive()` — a track is playing *and* the right
  column is visible (`width ≥ 112`). A `vizTicking` guard prevents duplicate tick
  loops; the loop ends on pause/hide and is restarted from the 1 Hz player poll.
- The visualizer is its own panel stacked under Now Playing in `renderRight`,
  both drawn with a light-green border (`lightPanelBox`).

## Consequences

**Positive**
- Delivers the requested look (animated green spectrum) with no new dependencies
  and no fragile audio-capture path on WSL2.
- The cost is bounded: the fast tick runs only while actually playing and the
  panel is on screen; everything else stays on the existing 1 Hz cadence.
- `vizLevels` is the single seam to replace if real capture is added later — the
  rendering and animation plumbing stay as-is.

**Negative / trade-offs**
- The bars **do not react to the actual audio** — only to play/pause. This is
  documented in the user guide so it isn't mistaken for a real analyzer.
- An ~8 fps full-frame redraw while playing is more CPU than the idle UI; it is
  gated to the visible-and-playing case and noted in the performance backlog.

## Alternatives considered
- **Real FFT via the PulseAudio monitor source** — the genuine CAVA approach.
  Deferred: fragile under WSLg, needs a capture goroutine + FFT dependency +
  format handling. Can be slotted in behind `vizLevels` later.
- **Shelling out to `cava` itself** and parsing its raw output — adds an external
  runtime binary and a second audio-capture configuration to get working; heavier
  than the in-process synthesized bars for the same on-screen result today.
- **No visualizer / a static meter** — rejected as not what was asked for.
