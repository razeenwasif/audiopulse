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

- AudioPulse stores **no credentials**. The Deezer search endpoints it uses
  require no API key or login.
- All network traffic is HTTPS to `api.deezer.com` (metadata/search) and
  Deezer's preview CDN (audio). No user data is transmitted.
- No telemetry, analytics, or background network calls are made.
- Nothing is written to disk at runtime; previews are buffered in memory and
  discarded.

**Trust boundaries**

- Untrusted input is the JSON returned by the Deezer API and the audio bytes of
  previews. These are parsed by the standard library JSON decoder and the
  `go-mp3`/`beep` decoders respectively. Decode errors are handled and surfaced,
  not fatal.
- User keystrokes are handled by Bubble Tea and never evaluated as shell
  commands; AudioPulse spawns no subprocesses.

**Known considerations**

- AudioPulse depends on third-party Go modules (Bubble Tea, Lip Gloss, beep,
  oto, go-mp3). Supply-chain risk is mitigated by pinned versions in `go.mod`/
  `go.sum` and Go module checksum verification.
- The audio backend links against the system ALSA library via cgo. The silent
  backend (`-tags nosound`) removes this native dependency entirely.

If you believe any of the above assumptions are violated, please report it using
the process above.
