# ADR-0010: Podcasts (saved shows + episodes)

- Status: Accepted
- Date: 2026-06-08

## Context

The center panel had decorative "Music / Podcasts" chips but no podcast data. The
goal was to make Podcasts real: browse saved shows, drill into episodes, and play
them, shown beside Music in the center.

Two constraints shaped the design:

1. **Center width.** When the right column is visible the center is only ~46–64
   columns. A permanent side-by-side split makes each pane ~23–32 columns, too
   cramped for episode titles on narrower terminals.
2. **Playback.** The Spotify Web API can *queue* an episode on a device, but the
   audio is decoded by **librespot**, whose podcast support is partial — some
   episodes are region-locked or hosted off Spotify's CDN and won't decode.

## Decision

**Data.** Add `SavedShows` and `ShowEpisodes` to the Spotify client
(`internal/spotify`), wrapping `CurrentUsersShows` / `GetShowEpisodes` with the
same pagination pattern as the music endpoints. Episodes are capped
(`maxShowEpisodes = 200`, most-recent first). No new OAuth scope is needed —
`user-library-read` already covers saved shows. New domain types `Show` and
`Episode` (the latter carries `Playable`, from the API's `is_playable`).

**Layout — hybrid.** `renderCenter` shows Music and Podcasts **side by side when
the center is ≥ `centerSplitMin` (64) columns**, and otherwise a **single pane
toggled by the Music/Podcasts chips** (`centerTab`). Both center sub-panes are
focusable (`panelTracks`, `panelPodcasts`) and join the Tab cycle; focusing a
sub-pane sets `centerTab`, so in single-column mode Tab/shift+tab also swaps the
visible pane. Saved shows load lazily the first time the podcast pane is focused.

**Navigation.** The podcast pane is a **master/detail split**: the saved-shows
list (top) and the opened show's episodes (bottom) are both visible. `podcastFocus`
("shows"/"episodes") tracks which sub-box the keyboard drives and which gets the
focus border. `enter` on a show loads its episodes and moves focus down; `enter` on
an episode plays it; `esc`/`backspace` moves focus back up to the shows list.
Episode playback sends a bounded window of episode URIs (reusing `maxPlayURIs`),
so it continues through the show like a context-less music source.

**Honesty about playback.** Unplayable episodes (`is_playable == false`) are marked
`⊘` and dimmed. Play failures surface as the normal "Playback action failed"
status rather than being hidden.

## Consequences

**Positive**
- Real podcast browsing and playback with no new credentials or scopes.
- The hybrid layout stays readable at any width and reuses the existing pane,
  cursor, windowing, and mouse-geometry helpers.
- `SavedShows`/`ShowEpisodes` are the single seam for future podcast features
  (search, resume points, downloads).

**Negative / trade-offs**
- librespot may not play every episode; this is a librespot limitation we surface
  rather than solve.
- Side-by-side panes are still tight on ~112–130-column terminals; titles
  truncate. The toggle fallback covers narrower sizes.
- Episode lists are capped at 200; very long back catalogs aren't fully browsable.
- Resume points / "mark as played" are not implemented (would need
  `user-read-playback-position`).

## Alternatives considered
- **Always side-by-side** — rejected: too cramped on common terminal widths.
- **Always a toggle (no split)** — rejected: the user wanted both visible when
  there's room; the hybrid gives that without breaking narrow layouts.
- **Podcasts in the left library list** instead of the center — rejected: mixes
  shows with playlists and doesn't match the requested music-vs-podcast split.
