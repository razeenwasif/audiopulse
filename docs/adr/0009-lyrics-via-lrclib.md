# ADR-0009: Lyrics via lrclib.net

- Status: Accepted
- Date: 2026-06-08

## Context

A lyrics panel was wanted under the library, ideally karaoke-style (the current
line highlighted and following playback).

The **Spotify Web API has no lyrics endpoint**. Spotify's in-app lyrics are
licensed from Musixmatch and served through a private, token-gated endpoint that
is not part of the public API and is against terms to scrape. So lyrics must come
from an independent source, matched to the now-playing track by metadata.

Requirements: free, no extra auth/keys (AudioPulse already asks the user for a
Spotify client ID and two OAuth logins — adding another credential is
undesirable), and ideally **time-synced** (LRC) so the current line can be
highlighted.

## Decision

Fetch lyrics from **lrclib.net** in a new `internal/lyrics` package.

- lrclib is a free, no-auth community lyrics database that returns both
  `plainLyrics` and `syncedLyrics` (LRC with `[mm:ss.xx]` timestamps).
- `Fetch` tries the exact `/api/get` (matches on track/artist/album/duration),
  then falls back to the fuzzy `/api/search`. It returns an empty (non-error)
  result when nothing matches, so "no lyrics" is a normal state, not an error.
- We send the **primary** artist only (`"A, B"` → `"A"`); lrclib's exact match
  dislikes joined multi-artist strings.
- `parseLRC` expands timestamps (a line may carry several) and sorts by time;
  `currentLyricLine` selects the latest line whose timestamp has passed, driven
  by the player's `Progress`. The synced current line renders green and the
  window scrolls to keep it centered.
- The lookup runs when the now-playing **track ID changes** (in the player-state
  handler), keyed by track ID so stale responses are ignored. No extra ticker is
  needed: the existing 1 Hz player poll (and the visualizer's faster tick while
  playing) already re-render often enough to advance the highlighted line.

## Consequences

**Positive**
- Real lyrics, frequently time-synced, with no new credentials and one small
  dependency-free package (stdlib `net/http` + `regexp`).
- Degrades cleanly: missing/instrumental/untimed lyrics each have a sensible
  panel state.
- `internal/lyrics.Fetch` is the single seam if another provider is added later.

**Negative / trade-offs**
- Coverage is community-driven: some tracks have no entry, and a few synced
  lyrics may drift slightly from librespot's playback clock.
- Matching is metadata-based, so an odd remaster/edit can occasionally fetch the
  wrong version. We prefer exact `/api/get` first to minimize this.
- One more outbound network dependency (lrclib uptime), isolated to the panel.

## Alternatives considered
- **Musixmatch (Spotify's source)** — private token, ToS issues, fragile.
- **Genius API** — requires an API key and returns HTML page lyrics (no sync),
  needing scraping; heavier and not time-synced.
- **No lyrics / static "unavailable"** — rejected; doesn't meet the request.
