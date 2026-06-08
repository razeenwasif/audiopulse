# Performance Backlog

Profiling-driven optimization ideas for AudioPulse. **None of these are
implemented yet** — this is a prioritized backlog. They target the steady-state
cost of a TUI that re-renders on a timer and polls a remote API.

Each item lists where it lives, the suggested approach, and rough impact/effort.
Validate with a profile before and after (`go test -bench`, `pprof`, or a manual
CPU sample while the app idles vs. plays) — don't optimize blind.

## Contents

- [CPU](#cpu)
  - [1. Stop full-screen redraws when nothing changes](#1-stop-full-screen-redraws-when-nothing-changes)
  - [2. Reduce Spotify polling](#2-reduce-spotify-polling)
  - [3. Cache rendered list rows](#3-cache-rendered-list-rows)
  - [4. Avoid repeated rune truncation](#4-avoid-repeated-rune-truncation)
  - [5. Preallocate string builders](#5-preallocate-string-builders)
- [Memory](#memory)
  - [6. Album-art cache (LRU)](#6-album-art-cache-lru)
  - [7. Choose a smaller cover image](#7-choose-a-smaller-cover-image)
  - [8. Bound explicit URI slices](#8-bound-explicit-uri-slices)
  - [9. Stream Deezer previews instead of buffering](#9-stream-deezer-previews-instead-of-buffering)

---

## CPU

### 1. Stop full-screen redraws when nothing changes

**Problem.** The UI ticks on a fixed timer and rebuilds the entire view string
(with Lip Gloss styling, `fillBG`, and — in Spotify mode — the album-art ANSI)
on every tick, even when idle or paused.

- Deezer mode ticks every **500 ms** — `tickCmd` in `internal/ui/model.go`.
- Spotify mode ticks every **1 s** — `tickCmd` in `internal/ui/spotify_model.go`.

**Approach.** Make the tick cadence state-dependent:
- Tick at the normal rate only while a track is **playing**.
- Slow the tick (e.g. 2–5 s) or stop it entirely when **idle/paused** — there's
  no progress bar to advance.
- Resume the fast cadence on play/seek/track-change.

Bubble Tea re-renders on every message, so fewer ticks ⇒ fewer full renders.

**Impact:** High (this is the dominant idle cost). **Effort:** Low–Medium.
**Risk:** Make sure a resumed/seeked track restarts the fast tick promptly.

> Note: the visualizer adds a separate ~8 fps tick (`vizTickCmd`), but it already
> follows this pattern — it runs *only* while playing and the right column is
> visible (`vizActive()`), and ends on pause/hide. The redraw cost is the
> incentive to land this item: at 8 fps the per-frame `View()` cost matters more.

### 2. Reduce Spotify polling

**Problem.** `pollCmd` in `internal/ui/spotify_model.go` calls **both** `State()`
and `Queue()` every second. `Queue()` (`GET /me/player/queue`) is the heavier
call and rarely changes between ticks.

**Approach.** Split the cadences:
- `State()` — every **1 s while playing**, **3–5 s while paused**.
- `Queue()` — only after a **track change, play action, or next/prev**, plus a
  slow keep-alive every **15–30 s**. Not at 1 Hz.

**Impact:** High (halves the steady-state request rate, removes the heavy call
from the hot path; also reduces rate-limit pressure). **Effort:** Medium (two
independent timers / a "queue dirty" flag). **Risk:** the up-next panel may lag a
few seconds behind external changes — acceptable.

### 3. Cache rendered list rows

**Problem.** `renderTrackList` and `renderLibrary` (`internal/ui/spotify_view.go`)
and the Deezer `renderList` (`internal/ui/view.go`) rebuild every styled row
string on every render — `fmt.Sprintf`, `truncate`, `padRight`,
`lipgloss.Render`, rune conversions — even when nothing about a row changed.

**Approach.** Memoize rendered rows, invalidated by the inputs that actually
affect them:
- terminal **width**,
- the **focused/selected** row,
- the **playing** row,
- a **list version** counter bumped when the underlying slice changes.

Render only the rows whose key changed; reuse cached strings otherwise.

**Impact:** Medium–High on large lists / fast ticks. **Effort:** Medium.
**Risk:** Cache-invalidation bugs (stale rows). Key on *everything* that affects
a row, and keep the cache per-panel.

### 4. Avoid repeated rune truncation

**Problem.** `truncate()` in `internal/ui/model.go` allocates a `[]rune(s)` on
every call, and the views call it constantly (every row, every render).

**Approach.**
- Cache pre-truncated display strings per width (folds into item 3).
- Use width-aware helpers (`github.com/charmbracelet/x/ansi` `ansi.Truncate`, or
  `runewidth.Truncate`) where display width — not rune count — is what matters.
- Skip truncation entirely for strings already known to be short
  (`len(s) <= width` fast path before the `[]rune` conversion).

**Impact:** Medium (lots of small allocations → GC pressure). **Effort:** Low.
**Risk:** `truncate` currently counts runes, not display cells — switching to a
width-aware helper is more correct but changes behavior for wide glyphs; verify
alignment.

### 5. Preallocate string builders

**Problem.** `renderTrackList`, `renderLibrary`, and especially `halfBlocks`
(`internal/ui/albumart.go`) grow a `strings.Builder` incrementally. Album-art
ANSI is very allocation-heavy (two 24-bit SGR sequences per cell).

**Approach.** Call `b.Grow(estimatedSize)` up front. For `halfBlocks` the size is
predictable: `cellW * cellH * (bytesPerCell)` (each cell ≈ a `▀` plus two
`\x1b[38;2;r;g;bm`/`\x1b[48;2;r;g;bm` codes, ~40 bytes) — pre-grow to avoid
repeated reslicing/copying.

**Impact:** Low–Medium (fewer reallocations on the art hot path). **Effort:**
Low. **Risk:** None; just an allocation hint.

---

## Memory

### 6. Album-art cache (LRU)

**Problem.** The Spotify model keeps only the **current** rendered art
(`m.art`/`m.artURL` in `internal/ui/spotify_model.go`). Moving between recently
played tracks re-downloads and re-decodes the same covers.

**Approach.** A small **LRU cache** keyed by image URL (16–32 covers) of the
rendered half-block string (and/or the decoded image). Avoids repeat
network + decode + render when revisiting tracks.

**Impact:** Medium (fewer downloads/decodes on navigation). **Effort:** Medium.
**Risk:** Bound the size; covers are a few KB rendered, so 32 entries is small.
Re-render on width/aspect change (key includes those).

### 7. Choose a smaller cover image

**Problem.** `toTrack()` in `internal/spotify/client.go` uses `Album.Images[0]`,
which is usually the **largest** (640×640). The terminal art is only ~22 cells
wide.

**Approach.** Pick the **smallest acceptable** image from `Album.Images` (Spotify
returns multiple sizes, typically 640/300/64). For ~22-cell art the 300px or even
64px source is plenty. Reduces network bytes, decode memory, and downscale CPU.

**Impact:** Medium (smaller downloads + decodes). **Effort:** Low. **Risk:**
Too-small a source (64px) may look soft at larger cell sizes; pick by the target
pixel size (`artW`×`artH`×2).

### 8. Bound explicit URI slices — DONE

**Problem.** `playSelectedCmd()` in `internal/ui/spotify_model.go` built a full
`[]spotify.URI` for non-context sources (Liked/Recent/Search). This became a real
issue once library fetches were paginated (Liked Songs can now be thousands of
tracks): Spotify's play endpoint rejects very large URI arrays.

**Done.** `playSelectedCmd` now sends a bounded window of `maxPlayURIs` (500)
tracks starting at the selection, with the selected track at offset 0. Playlists
still prefer the context URI (`PlayContext`) and are unaffected.

**Impact:** Correctness fix (large Liked Songs would otherwise fail to play) plus
a smaller request. **Effort:** Low. **Risk:** Playback runs out after the window;
acceptable for a 500-track lookahead.

### 9. Stream Deezer previews instead of buffering

**Problem.** `internal/player/player_beep.go` downloads each preview fully into
memory with `io.ReadAll` before decoding.

**Approach.** Stream from the HTTP body into the decoder rather than buffering the
whole file, lowering peak memory.

**Impact:** Low (30-second MP3s are a few hundred KB). **Effort:** Medium.
**Risk:** **Real trade-off** — a seekable in-memory buffer makes MP3 decoding and
accurate length/progress simple. Streaming complicates `Len()`/seek. Probably
**not worth it** unless memory is shown to matter; documented for completeness.

---

## Suggested order

Biggest win per unit effort first:

1. **#1 idle ticks** + **#2 polling split** — the steady-state cost; mostly timer
   plumbing.
2. **#7 smaller cover** + **#5 builder preallocation** — quick, low-risk.
3. **#6 art LRU** + **#3 row cache** (with **#4** folded in) — more code, real
   gains on navigation and large lists.
4. **#8 / #9** — defer until profiling or feature growth justifies them.
