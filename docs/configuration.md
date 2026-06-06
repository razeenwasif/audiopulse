# Configuration

AudioPulse aims to "just run" with sensible defaults and minimal configuration.
This page documents the knobs that do exist — at build time, at run time, and in
the source — and the constants you would change to tune behaviour.

## Contents

- [Philosophy](#philosophy)
- [Build-time configuration](#build-time-configuration)
- [Runtime configuration](#runtime-configuration)
- [Environment variables](#environment-variables)
- [Tunable constants](#tunable-constants)
- [Theme](#theme)

## Philosophy

**Deezer guest mode** needs no configuration, no API key, and no login — it runs
out of the box. **Spotify mode** is opt-in: it requires a public Client ID
(`SPOTIFY_CLIENT_ID` or `~/.config/audiopulse/config.json`) and caches an OAuth
token locally. See [ADR-0005](adr/0005-spotify-via-librespot.md),
[getting-started → Spotify mode](getting-started.md#spotify-mode-full-songs), and
[SECURITY.md](../SECURITY.md).

## Build-time configuration

The one build-time switch is the **audio backend**, selected by a Go build tag:

| Build command                      | Backend | Audio |
| ---------------------------------- | ------- | ----- |
| `go build .`                       | beep    | yes   |
| `go build -tags nosound .`         | silent  | no    |

See [ADR-0004](adr/0004-build-tag-fallback-strategy.md) and
[Development → Build tags](development.md#build-tags-and-backends).

## Runtime configuration

All runtime configuration is interactive — there are no flags or config files.
The following are adjustable while the app runs and reset on restart:

| Setting   | Control          | Default |
| --------- | ---------------- | ------- |
| Volume    | `+` / `-`        | ~0.66   |
| Autoplay  | `a`              | on      |
| Search    | type + `Enter`   | —       |

## Environment variables

AudioPulse itself reads no custom environment variables. The variables that
*indirectly* affect it come from its dependencies and the OS:

| Variable            | Consumed by   | Effect                                            |
| ------------------- | ------------- | ------------------------------------------------- |
| `SPOTIFY_CLIENT_ID` | AudioPulse    | Enables Spotify mode (PKCE public Client ID)      |
| `AUDIOPULSE_GUEST`  | AudioPulse    | If set, forces Deezer guest mode even with a Client ID |
| `TERM`              | terminal libs | Colour and capability detection                   |
| `NO_COLOR`          | Lip Gloss     | If set, disables ANSI colour output               |
| `COLORTERM`         | Lip Gloss     | Enables truecolor when set (e.g. `truecolor`)     |
| `HTTP_PROXY` / `HTTPS_PROXY` | net/http | Routes API requests through a proxy          |
| `PULSE_SERVER`      | audio stack   | Selects the PulseAudio server (useful under WSL2) |

> Setting `NO_COLOR=1` yields a monochrome UI, which can help on terminals with
> poor colour support.

## Tunable constants

If you are building from source and want to change defaults, these are the
relevant constants:

| Constant / field   | Location              | Meaning                                  |
| ------------------ | --------------------- | ---------------------------------------- |
| `limit=50`         | `internal/deezer`     | Max search results requested             |
| `outputRate`       | `internal/player`     | Speaker sample rate (44.1 kHz)           |
| `previewLen`       | `internal/player` (silent) | Simulated track length (30 s)       |
| `volumeStep`       | `internal/player`     | Gain change per `+`/`-` press            |
| tick interval      | `internal/ui` (`tickCmd`) | Progress refresh rate (500 ms)       |
| `sidebarWidth`     | `internal/ui`         | Sidebar width in columns                 |
| min terminal size  | `internal/ui` (`View`)| 64×18 floor before the resize prompt     |
| HTTP timeouts      | `internal/deezer`, `internal/player` | Request timeouts          |

## Theme

The colour palette lives in `internal/ui/styles.go`. The accent is Spotify green:

| Token         | Value     | Used for                          |
| ------------- | --------- | --------------------------------- |
| `colorAccent` | `#1DB954` | Panel borders, markers, progress  |
| `colorAccentHi` | `#1ED760` | Highlights, selected text        |
| `colorText`   | `#FFFFFF` | Primary text                      |
| `colorMuted`  | `#B3B3B3` | Secondary text                    |
| `colorFaint`  | `#535353` | Tertiary text, empty bar segments |
| `colorErr`    | `#F15E6C` | Error messages                    |

To re-theme, change these constants and rebuild. Because all styles derive from
this single palette, an accent change propagates everywhere (borders, the
progress bar, selection markers) consistently.
