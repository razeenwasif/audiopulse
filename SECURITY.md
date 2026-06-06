# Security Policy

## Supported versions

AudioPulse is pre-1.0. Security fixes are applied to the latest released minor
version and to `main`.

| Version | Supported |
| ------- | --------- |
| 0.1.x   | ✅        |
| < 0.1   | ❌        |

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, report them privately via GitHub's
[private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
("Security" tab → "Report a vulnerability"), or by email to the maintainers.

Please include:

- A description of the issue and its impact.
- Steps to reproduce or a proof of concept.
- Affected version(s) and platform.

**Response targets** (best effort for a community project):

- Acknowledgement within 5 business days.
- An initial assessment and remediation plan within 15 business days.
- Coordinated disclosure once a fix is available.

## Security posture and threat model

AudioPulse is a local, single-user terminal application. Its attack surface is
small and intentionally kept that way.

**Data handling**

- **Deezer guest mode** stores no credentials and requires no login. All traffic
  is HTTPS to `api.deezer.com` and Deezer's preview CDN; previews are buffered in
  memory and discarded.
- **Spotify mode** authenticates with OAuth 2.0 **PKCE** (a public Client ID; no
  client secret is used or stored). The resulting token is cached at
  `~/.config/audiopulse/token.json` with `0600` permissions, and librespot's
  device credentials under `~/.config/audiopulse/librespot/`. These are never
  logged and are refreshed via the standard OAuth refresh flow.
- The OAuth redirect is captured by a short-lived server bound to **loopback
  only** (`127.0.0.1:8888`).
- No telemetry or analytics. Network traffic is HTTPS to Spotify
  (`accounts.spotify.com`, `api.spotify.com`) for control/metadata; audio is
  streamed by librespot directly from Spotify.

**Trust boundaries**

- Untrusted input is the JSON/audio returned by Deezer/Spotify. JSON is parsed by
  the standard decoder; preview audio by `go-mp3`/`beep`. Decode errors are
  handled, not fatal.
- User keystrokes are handled by Bubble Tea and never evaluated as shell
  commands.
- In Spotify mode AudioPulse spawns one subprocess — the **librespot** binary —
  located on `PATH` or in `~/.cargo/bin`, with arguments fixed by AudioPulse (no
  user-controlled command strings).

**Known considerations**

- Third-party Go modules (Bubble Tea, Lip Gloss, beep, oto, go-mp3,
  zmb3/spotify, x/oauth2) are pinned in `go.mod`/`go.sum` with checksum
  verification.
- librespot is an external binary built from source via `cargo install`
  (`--locked`, pinning its dependency versions). It is not bundled.
- The audio backend links against the system ALSA library via cgo. The silent
  backend (`-tags nosound`) removes this native dependency entirely.

If you believe any of the above assumptions are violated, please report it using
the process above.
