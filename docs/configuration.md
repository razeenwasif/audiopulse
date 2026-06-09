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

## Library export (spotDL)

AudioPulse can export your Spotify library to local audio files using
[spotDL](https://github.com/spotDL/spotify-downloader) (install with `make spotdl`;
needs Python + FFmpeg — `make doctor` checks both). Downloads go to the **music
directory**, set in `~/.config/audiopulse/config.json`:

```json
{ "client_id": "…", "music_dir": "/mnt/e/Music/audiopulse" }
```

- Default (when unset): `~/Music/audiopulse`. A leading `~/` is expanded.
- On WSL, point it at a mounted drive (e.g. `/mnt/d/Music`) to keep the library
  off the WSL virtual disk. Drive speed doesn't affect playback or export — only
  the initial library scan, where an SSD helps.

spotDL sources audio from YouTube (lossy; occasional mismatches) and skips files
that already exist, so an interrupted export is resumable.

## Environment variables

AudioPulse itself reads no custom environment variables. The variables that
*indirectly* affect it come from its dependencies and the OS:

| Variable            | Consumed by   | Effect                                            |
| ------------------- | ------------- | ------------------------------------------------- |
| `SPOTIFY_CLIENT_ID` | AudioPulse    | Enables Spotify mode (PKCE public Client ID)      |
| `AUDIOPULSE_GUEST`  | AudioPulse    | If set, forces Deezer guest mode even with a Client ID |
| `AUDIOPULSE_CELL_ASPECT` | AudioPulse | Terminal cell height/width ratio for album art (default auto-detected, else `2.0`). Raise it (e.g. `2.2`) if art looks too tall, lower it (e.g. `1.8`) if too wide |
| `AUDIOPULSE_TRANSPARENT` | AudioPulse | If set, keeps the terminal's background transparency. By default the player paints an opaque backdrop so it's readable over translucent/acrylic terminals |
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

| Token           | Value     | Used for                                       |
| --------------- | --------- | ---------------------------------------------- |
| `colorAccent`   | `#1DB954` | Accent: focused border, progress, play button  |
| `colorAccentHi` | `#1ED760` | Highlights, selected text, active shuffle/repeat |
| `colorText`     | `#FFFFFF` | Primary text                                   |
| `colorMuted`    | `#B3B3B3` | Secondary text                                 |
| `colorFaint`    | `#535353` | Tertiary text, empty bar segments              |
| `colorCard`     | `#1F1F1F` | Search pill / chip backgrounds                 |
| `colorBorder`   | `#2A2A2A` | Subtle panel border (unfocused)                |
| `colorErr`      | `#F15E6C` | Error messages                                 |

In Spotify mode the theme follows the desktop client: borders are the subtle
`colorBorder` and turn `colorAccent` (green) only on the **focused** panel; green
is otherwise reserved as an accent for active state (see
[ADR-0006](adr/0006-spotify-desktop-ui.md)). To re-theme, change these constants
and rebuild.
