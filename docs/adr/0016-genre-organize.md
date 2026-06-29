# ADR-0016: Organize Liked Songs into genre playlists

- Status: Accepted
- Date: 2026-06-29

## Context

The user wanted to ask the assistant to *"group every song in my Liked Songs into
respective playlists matching by genre"* — for a ~500-track library. The existing
`create_playlist` action ([ADR-0012](0012-library-mutations.md) scopes) curates a
*new* themed list from an LLM; it does not read or reorganize the existing
library, so it can't do this.

Two facts shaped the design:

1. **Spotify attaches genres to *artists*, not tracks.** A `track` object carries
   no genre; the `artist` object does (`GET /v1/artists?ids=…`, 50/call). So a
   track's genre is derived from its **primary artist**, which means the flow is
   API-driven and **deterministic — no LLM classification** (and no hallucinated
   genres).
2. **Artist genres are hyper-granular** ("australian indie rock", "indietronica",
   "melodic phonk"). Hundreds of distinct tags would produce a useless pile of
   playlists, so they must collapse into a handful of coarse buckets.

## Decision

**A new `organize_library` action** (NL/voice only; no keybinding) routed by the
assistant, distinct from `create_playlist`. The work is pure Spotify API + a small
pure-Go classifier — Ollama is used only to *route* the request, never to classify.

**Coarse genre bucketing (`internal/library/genres.go`).** `GenreBucket(genres)`
scans an **ordered** keyword→bucket rule list and returns the first match (so
specific beats generic: "k-pop" before "pop", "indie" before "rock"), else
`"Other"`. `BuildGenreGroups(tracks, genres, minBucket)` buckets every track by
its primary artist's genre, **merges buckets smaller than `minBucket` (4) into
"Other"** so there's no one-song-playlist spam, and sorts largest-first with
"Other" pinned last. Pure and unit-tested.

**Two-phase UX: preview then confirm.** Creating ~10–15 playlists on someone's
account is a big, outward-facing, hard-to-undo action, so it is never done
silently:

- *Plan* (`planOrganizeCmd`, behind the "working" spinner): page all Liked Songs,
  collect unique primary-artist ids, `ArtistGenres` (batched 50), build the
  groups.
- *Preview* (`organizeState == "preview"`): a modal lists each bucket and its size
  ("Liked: Rock 30, Liked: Hip-Hop 20, …, Liked: Other 5"); `↵` confirms, `esc`
  cancels — nothing is created until `↵`.
- *Run* (`organizeState == "running"`, channel-streamed like the index build):
  one playlist per bucket named `"Liked: <Genre>"`, tracks added chunked (100/req),
  with a live progress bar. On completion the sidebar reloads to show them.

**Scopes.** Reuses `playlist-modify-public` / `playlist-modify-private` from
ADR-0012 — no new permission.

## Consequences

**Positive**
- Accurate, deterministic genre grouping with no LLM guesswork; works for the
  full library in one pass.
- The preview/confirm step prevents surprise playlist spam and gives the user the
  bucket breakdown before committing.
- Reuses existing patterns (channel-streamed progress, `playlist-modify` scopes,
  `CreatePlaylist`/`AddTracksToPlaylist`).

**Negative / trade-offs**
- Bucketing is heuristic: an artist tagged only "pop rap" lands in one bucket;
  artists with no Spotify genres fall to "Other". The ordered rule list is a
  pragmatic curation, not a taxonomy.
- Genre is the **primary artist's**, so a genre-bending track follows its lead
  artist.
- Re-running creates a fresh set of "Liked: …" playlists (no dedupe/update of a
  previous run) — documented.

## Alternatives considered
- **LLM classifies each track's genre** — slow (500 calls), non-deterministic, and
  hallucination-prone when the model doesn't know a track. Artist genres are
  authoritative and free.
- **Audio-features (danceability/energy) clustering** — `/v1/audio-features` is
  403 for new apps ([ADR-0015](0015-library-rag.md)).
- **A keybinding instead of NL** — the user asked for it conversationally; a bulk,
  destructive-ish action is better gated behind an explicit phrase + confirm than
  a stray keypress.
