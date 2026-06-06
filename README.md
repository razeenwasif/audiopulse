<div align="center">

# ♫ AudioPulse

**A Spotify-style music player for your terminal.**

[![CI](https://github.com/razeenwasif/audiopulse/actions/workflows/ci.yml/badge.svg)](https://github.com/razeenwasif/audiopulse/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Status](https://img.shields.io/badge/status-beta-orange)](#project-status)

Play full songs from your Spotify account — without leaving your shell.

</div>

---

AudioPulse is a terminal user interface (TUI) built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea), with a Spotify
desktop-style layout (library · feed · now-playing · player bar).

- **Spotify mode** (Premium): plays **full songs** through an embedded
  [librespot](https://github.com/librespot-org/librespot) device, controlled via
  the Spotify Web API. Browse your playlists, liked songs, and queue.
- **Deezer guest mode** (no login): search the public
  [Deezer](https://developers.deezer.com/api) catalogue and play 30-second
  previews via [faiface/beep](https://github.com/faiface/beep) — the automatic
  fallback when no Spotify Client ID is configured.

```
 ♫ AudioPulse                                                                       Razeen · Spotify
╭──────────────────────────╮╭──────────────────────────────────────╮╭────────────────────────────────╮
│  LIBRARY                 ││ Chill Vibes                          ││ NOW PLAYING                    │
│   ♥ Liked Songs          ││ 24 tracks                            ││ ╭────────────────────────────╮ │
│   ◷ Recently Played      ││                                      ││ │             ♫              │ │
│   ▸ Discover Weekly      ││   Midnight City — M83          4:04  ││ ╰────────────────────────────╯ │
│  ▌▸ Chill Vibes          ││ ▶ Instant Crush — Daft Punk    5:37  ││ Instant Crush                  │
│   ▸ Focus Flow           ││   Redbone — Childish Gambino   5:27  ││ Daft Punk, Julian Casablancas  │
│                          ││   Nights — Frank Ocean         5:07  ││ ── Up Next ──                  │
╰──────────────────────────╯╰──────────────────────────────────────╯╰────────────────────────────────╯
╭──────────────────────────────────────────────────────────────────────────────────────────────────╮
│  ▶  Instant Crush  —  Daft Punk, Julian Casablancas                                       🔀 🔁    │
│  1:12 / 5:37 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  🔊 65%       │
╰──────────────────────────────────────────────────────────────────────────────────────────────────╯
  tab panel  •  ↑↓ move  •  enter open/play  •  space pause  •  n/b next/prev  •  ←→ seek  •  +/- vol
```

## Contents

- [Features](#features)
- [Quick start](#quick-start)
- [Building](#building)
- [Keybindings](#keybindings)
- [Documentation](#documentation)
- [Project status](#project-status)
- [Contributing](#contributing)
- [License](#license)

## Features

- 🎧 **Full-song playback** from your Spotify Premium account via embedded librespot.
- 🗂️ **Desktop-style layout** — library, track feed, now-playing, and a player bar.
- ▶️ **Full transport control** — play/pause, next/prev, seek, volume, shuffle, repeat.
- 📚 **Your library** — playlists, Liked Songs, Recently Played, and live queue.
- 🟢 **Spotify-green** accent theme throughout.
- 🆓 **Deezer guest mode** — no-login preview playback when Spotify isn't configured.
- 🔐 **PKCE OAuth** — public Client ID only, token cached locally with `0600` perms.

## Quick start

**Deezer guest mode** (no account, no setup):

```bash
sudo apt-get install -y libasound2-dev      # one-time (Debian/Ubuntu)
make run
```

**Spotify mode** (full songs, Premium): install the playback backend, set your
Client ID, and run — see **[docs/getting-started.md → Spotify mode](docs/getting-started.md#spotify-mode-full-songs)**:

```bash
make librespot                              # one-time (~10-15 min Rust build)
export SPOTIFY_CLIENT_ID=your_client_id     # from developer.spotify.com
make run                                    # authorizes in your browser, once
```

No audio device or can't install the headers? Run the silent build, which has
**no native dependencies** and simulates playback faithfully:

```bash
make run-silent
```

Full, platform-by-platform instructions live in
**[docs/getting-started.md](docs/getting-started.md)**.

## Building

| Command           | Result                                                        |
| ----------------- | ------------------------------------------------------------- |
| `make build`      | Compile with the real audio backend (needs `libasound2-dev`). |
| `make librespot`  | Build & install the librespot backend for Spotify full-song playback. |
| `make install`    | Build and install to `~/.local/bin` (run `audiopulse` anywhere). |
| `make uninstall`  | Remove the installed binary.                                  |
| `make run`        | Build with audio and launch.                                  |
| `make silent`     | Compile the no-audio fallback (`-tags nosound`).              |
| `make run-silent` | Build the silent fallback and launch.                         |
| `make test`       | Run unit + render tests (no device required).                 |
| `make vet`        | Static analysis.                                              |
| `make fmt`        | Format all Go sources.                                        |
| `make doctor`     | Check the toolchain and audio stack, and test sound output.   |

## Keybindings

**Spotify mode**

| Key             | Action                                       |
| --------------- | -------------------------------------------- |
| `tab`           | Switch between the library and track panels  |
| `↑`/`↓` `j`/`k` | Move selection · `g`/`G` jump to top/bottom  |
| `enter`         | (library) open · (tracks) play full song     |
| `space` / `p`   | Pause / resume                               |
| `n` / `b`       | Next / previous track                        |
| `←` / `→`       | Seek −/+ 5s                                  |
| `+` / `-`       | Volume up / down                             |
| `s` / `r`       | Toggle shuffle / cycle repeat                |
| `q` / `ctrl+c`  | Quit                                         |

**Deezer guest mode**

| Key             | Action                                  |
| --------------- | --------------------------------------- |
| `/`             | Focus the search box                    |
| `enter`         | (search) run search · (list) play       |
| `esc` / `tab`   | Toggle between search and results       |
| `space` / `p`   | Pause / resume · `s` stop · `a` autoplay |
| `n` / `b`       | Next / previous · `+`/`-` volume         |
| `q` / `ctrl+c`  | Quit                                    |

## Documentation

Complete documentation lives in the **[`docs/`](docs/)** directory:

| Document                                             | What it covers                                              |
| ---------------------------------------------------- | ----------------------------------------------------------- |
| [Getting Started](docs/getting-started.md)           | Prerequisites, installation, and first run on each platform |
| [User Guide](docs/user-guide.md)                     | Every screen, control, and workflow                         |
| [Architecture](docs/architecture.md)                 | Components, data flow, concurrency, and diagrams            |
| [Development](docs/development.md)                    | Dev setup, project layout, build tags, testing              |
| [Configuration](docs/configuration.md)               | Build-time and runtime configuration knobs                  |
| [Troubleshooting](docs/troubleshooting.md)           | Audio, WSL, ALSA, and network issues                        |
| [Architecture Decision Records](docs/adr/)           | Why the key technical choices were made                     |

Project governance: [CONTRIBUTING](CONTRIBUTING.md) ·
[CODE_OF_CONDUCT](CODE_OF_CONDUCT.md) · [SECURITY](SECURITY.md) ·
[CHANGELOG](CHANGELOG.md)

## Project status

AudioPulse is **beta** (`0.1.x`). The feature set and module layout are stable;
the exported APIs under `internal/` may still change. See the
[CHANGELOG](CHANGELOG.md) for release history.

> **Note on previews:** Deezer previews are capped at 30 seconds by the API —
> that is a platform limitation, not a player one. See
> [ADR-0002](docs/adr/0002-deezer-as-data-source.md) for the rationale and
> alternatives (local files, Spotify Web API).

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) and the
[Code of Conduct](CODE_OF_CONDUCT.md) before opening an issue or pull request.

## License

Released under the [MIT License](LICENSE). AudioPulse is not affiliated with,
endorsed by, or certified by Deezer or Spotify; those names are trademarks of
their respective owners.
