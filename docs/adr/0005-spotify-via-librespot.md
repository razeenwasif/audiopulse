# ADR-0005: Spotify full-song playback via embedded librespot

- Status: Accepted
- Date: 2026-06-07
- Extends: [ADR-0002](0002-deezer-as-data-source.md), [ADR-0003](0003-beep-for-audio-playback.md)

## Context

The Deezer integration ([ADR-0002](0002-deezer-as-data-source.md)) only exposes
30-second previews. The user has a Spotify **Premium** account and wants
**full-song** playback plus a desktop-style layout.

The decisive constraint: the **Spotify Web API does not serve raw audio**. Full
songs can only be played through an authorized Spotify playback device. The
realistic options were:

1. **Remote-control** an existing Spotify app (phone/desktop) via Spotify
   Connect — simplest, but requires another Spotify client running.
2. **Embed librespot** — the open-source Spotify client that authenticates as a
   Connect device and streams/decodes audio itself.

## Decision

Run **librespot** as a supervised child process that registers as a Spotify
Connect device named **"AudioPulse"**, and **control it through the Spotify Web
API** (zmb3/spotify). librespot decodes audio and outputs via its **ALSA
backend**, which routes through the existing `~/.asoundrc` → PulseAudio path.

- **Auth:** Web API uses OAuth 2.0 **PKCE** (public Client ID, no secret); the
  token is cached at `~/.config/audiopulse/token.json` (`0600`). librespot
  performs its own one-time OAuth device login, cached under
  `~/.config/audiopulse/librespot/`.
- **Control/audio split:** the Web API issues play/pause/seek/next/volume/
  shuffle/repeat against the librespot device; now-playing state is polled.
- **Deezer retained** as a no-login "guest" fallback (preview playback via
  `beep`, [ADR-0003](0003-beep-for-audio-playback.md)) when no Client ID is set.

## Consequences

**Positive**
- True **standalone full-song playback** in the terminal — no other Spotify app
  required.
- Reuses the existing ALSA→PulseAudio audio path; no new audio plumbing.
- The Web API client and UI are decoupled from audio; the player is just a
  device the UI commands.

**Negative / trade-offs**
- New external dependency: the **librespot** binary (`make librespot`, a ~10–15
  min Rust build). Pinned features: `alsa-backend,rustls-tls-webpki-roots` (the
  latter bundles CA roots, avoiding a system OpenSSL dependency).
- **Two one-time browser authorizations** on first run (Web API + librespot).
- We now **persist credentials** locally, changing the security posture (see
  [SECURITY.md](../../SECURITY.md)). Mitigated by `0600` perms and PKCE (no
  secret stored).
- Requires **Premium** (a librespot/Spotify constraint).

## Alternatives considered
- **Remote-control only** — rejected as the default because it needs another
  Spotify client open; it remains a possible future mode.
- **Spotify's own SDKs** — there is no official terminal/Go playback SDK;
  librespot is the de-facto approach (used by ncspot, spotify-player).
- **Keep Deezer only** — does not meet the full-song requirement.
