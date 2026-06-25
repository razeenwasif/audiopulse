# ADR-0012: Library mutations (like / follow) and scope re-auth

- Status: Accepted
- Date: 2026-06-09

## Context

Users wanted to like/unlike tracks and unfollow podcasts from the UI. These are
**write** operations on the Spotify library, which raises two issues:

1. **A new OAuth scope.** Saving/removing tracks and following/unfollowing shows
   require `user-library-modify`, which earlier builds didn't request. Existing
   users already have a cached token granted with the *old* scope set, and the
   token file doesn't record which scopes were granted — so we can't tell from it
   whether the new permission is present.
2. **A missing client method.** `zmb3/spotify v2.4.3` exposes save-track,
   remove-track, contains-track, and save-show, but **no remove-show** (unfollow).

## Decision

**Scope re-auth via a fingerprint sidecar.** `config.Scopes` gains
`user-library-modify`. On a successful authorization we write a `scopes` file
(sorted scope list) next to the token. `auth.NeedsReauthForScopes()` compares it
to the current scopes; `main` calls this before building the client and runs the
PKCE flow once if they differ (or if the sidecar is absent, i.e. a token that
predates this mechanism). Result: each user re-authorizes exactly once after
updating, then never again until scopes change.

**Raw HTTP for the gap.** The Spotify wrapper now keeps the authorized
`*http.Client` it was built with, and `UnfollowShow` issues a direct
`DELETE /v1/me/shows?ids=…` through it (the token is injected by the oauth2
transport, and 429s are still retried at the transport layer). Like/unlike and
the saved-state check use the library's `AddTracksToLibrary` /
`RemoveTracksFromLibrary` / `UserHasTracks`.

**UI.** `L` toggles like for the selected music track (else the playing track),
optimistically updating a `liked` cache and showing a `♥` in now-playing; the
real result reconciles the cache (reverting on error). The Liked Songs list marks
all its rows liked, and the playing track's state is checked on each track change.
`F` unfollows the highlighted show and reloads the list. Following a *new* show
isn't reachable yet (the pane only lists already-followed shows) — that arrives
with show search.

## Update (2026-06-18): add to playlist

The same mechanism now backs **adding a track to a playlist** (`P`). `config.Scopes`
gains `playlist-modify-public` and `playlist-modify-private`, so existing users
get one more one-time re-auth (the fingerprint handles it). The wrapper's
`AddTrackToPlaylist` uses the library's typed `AddTracksToPlaylist` (no raw HTTP
needed this time). `Playlists()` now also returns each playlist's owner id and
collaborative flag; the UI marks a playlist **editable** when the user owns it or
it's collaborative (using `MeID`), and the `P` picker offers only those — so the
user never picks a playlist the API would 403 on. `P` targets the selected music
track (else the playing track), mirroring `L`.

## Consequences

**Positive**
- Real library mutations with correct, optimistic, self-reconciling UI state.
- The fingerprint makes future scope additions a clean one-time re-auth instead
  of silent 403s.
- Keeping the `*http.Client` unlocks any endpoint the library doesn't wrap
  (unfollow today; add-to-playlist, episode queueing later).

**Negative / trade-offs**
- One forced re-authorization for existing users on upgrade (clearly messaged).
- The `liked` cache isn't exhaustive — non-Liked-Songs lists only show `♥` for
  tracks seen via the playing track or a like action, not a per-row contains
  query (avoids a batch lookup per list load).
- Raw calls bypass the typed client; they're minimal and centralized in
  `deleteRaw`.

## Alternatives considered
- **Detect the missing scope from a 403 at runtime and re-auth mid-TUI** —
  re-auth needs a browser + localhost server, awkward inside the alt-screen; the
  pre-launch fingerprint check is simpler and deterministic.
- **Vendor/patch zmb3 for remove-show** — heavier than one `deleteRaw` call.
- **Per-row saved-state via `UserHasTracks` batches** — extra latency/traffic on
  every list load; deferred until it's shown to matter.
