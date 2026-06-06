# ADR-0002: Deezer as the catalogue data source

- Status: Accepted
- Date: 2026-06-06

## Context

AudioPulse needs a music catalogue to search and audio to play. We wanted a
"Spotify-like" experience — real metadata and real, audible playback — with the
lowest possible setup friction. Candidate sources differ sharply in
authentication burden, whether they expose playable audio, and licensing.

Key forces:

- **Zero/low setup** — ideally no API key or login, so the app runs immediately.
- **Playable audio** — metadata alone isn't enough; we want sound out of the box.
- **Stable, documented HTTP API**.

## Decision

Use the **public Deezer API** (`https://api.deezer.com`) as the catalogue and
audio source. Specifically, the `/search` endpoint, which:

- Requires **no API key and no OAuth**.
- Returns rich metadata (title, artist, album, cover art, duration).
- Includes a **30-second `preview` MP3 URL** per track that can be streamed and
  played directly.

The client filters results to tracks that have a non-empty `preview`, so the UI
never offers something it cannot play.

## Consequences

**Positive**

- First-run friction is zero: no accounts, keys, or secrets.
- Real metadata **and** real audio, satisfying the "Spotify-like" goal.
- The security surface is minimal — nothing to store or leak (see
  [SECURITY.md](../../SECURITY.md)).

**Negative / trade-offs**

- **Previews are capped at 30 seconds** by Deezer. This is surfaced honestly in
  the UI and docs; the progress bar reflects the true preview length.
- Dependence on an external public API: availability and terms are outside our
  control, and the unauthenticated endpoints are best-effort and rate-limited.

## Alternatives considered

- **Shazam (via RapidAPI)** — requires an API key and a RapidAPI account, has a
  rate-limited free tier, and is oriented around audio recognition/charts rather
  than catalogue browsing. Higher friction, weaker fit.
- **Spotify Web API** — the closest brand match, but requires OAuth and a Spotify
  Premium account, and full playback needs an active device via the Connect API.
  Far higher setup cost; rejected for the default experience.
- **iTunes Search API** — also key-free with 30-second previews; kept in reserve
  as a viable backup/secondary source.
- **Local music files** — real full-length playback, but not a catalogue and
  requires the user to supply media. A possible future mode, not the default.

## Future direction

The UI depends only on the returned `Track` shape, and the player takes a plain
preview URL, so a future `Catalogue` interface could add iTunes, local files, or
the Spotify Web API as alternative sources without changing the UI. See
[Architecture → Extension points](../architecture.md#extension-points).
