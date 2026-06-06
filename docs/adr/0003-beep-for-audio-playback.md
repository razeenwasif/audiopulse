# ADR-0003: `faiface/beep` for audio playback

- Status: Accepted
- Date: 2026-06-06

## Context

We need to play 30-second MP3 previews from within a Go program, with:

- Play / pause / resume / stop control.
- A queryable playback position (for the progress bar).
- Volume control.
- A completion signal (to drive autoplay).

Options ranged from pure-Go audio libraries to shelling out to an external
player.

## Decision

Use **`faiface/beep`** (over `hajimehoshi/oto` and `hajimehoshi/go-mp3`) as the
audio engine. beep provides exactly the streaming primitives we need:

- `mp3.Decode` → a `StreamSeekCloser` with `Position()`/`Len()` for progress.
- `beep.Ctrl` for pause/resume.
- `effects.Volume` for gain.
- `beep.Resample` to match the speaker's sample rate.
- `beep.Seq` + `beep.Callback` to fire a callback when a stream drains naturally.

Each preview is downloaded fully into an in-memory seekable buffer before
decoding, which makes decoding robust and gives an accurate track length.

## Consequences

**Positive**

- A clean, composable streaming graph maps directly onto our feature list.
- Accurate progress and a reliable completion callback enable autoplay.
- Staying in-process (vs. shelling out) avoids managing child processes and
  parsing another program's output.

**Negative / trade-offs**

- beep depends on `oto`, which uses **cgo** and links the system audio library
  (ALSA on Linux). This adds a build dependency (`libasound2-dev`) and
  complicates static builds. Mitigated by
  [ADR-0004](0004-build-tag-fallback-strategy.md).
- Introduces a documented **lock-ordering** requirement between `Player.mu` and
  beep's mixer lock (see
  [Architecture → Concurrency model](../architecture.md#concurrency-model)).

## Alternatives considered

- **Shell out to `mpv`/`ffplay`** — trivial to play audio and avoids cgo, but
  requires those binaries to be installed, makes position/volume/pause control
  and completion detection awkward (IPC or parsing), and weakens portability of
  the experience. (These tools remain handy for manual diagnostics.)
- **`oto` + `go-mp3` directly** — exactly what beep wraps; using them raw would
  mean re-implementing the control/resample/sequencing layer beep already
  provides.
- **`gopxl/beep`** (the maintained fork) — API-compatible and a reasonable future
  migration target; `faiface/beep` was chosen for familiarity and stability at
  the time of writing.
