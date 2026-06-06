<div align="center">

# ♫ AudioPulse

**A Spotify-style music player for your terminal.**

[![CI](https://github.com/razeenwasif/audiopulse/actions/workflows/ci.yml/badge.svg)](https://github.com/razeenwasif/audiopulse/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Status](https://img.shields.io/badge/status-beta-orange)](#project-status)

Search the Deezer catalogue and play track previews — without leaving your shell.

</div>

---

AudioPulse is a terminal user interface (TUI) built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea). It searches the public
[Deezer](https://developers.deezer.com/api) catalogue (no API key, no login) and
streams 30-second previews through your speakers via
[faiface/beep](https://github.com/faiface/beep).

```
 ♫ AudioPulse                                                       powered by Deezer
╭──────────────────────╮╭──────────────────────────────────────────────────────────╮
│  LIBRARY             ││   get lucky                                                │
│                      ││ ────────────────────────────────────────────────────────  │
│    Search            ││ 50 results                                                 │
│  ▌ Results           ││ ♪ Get Lucky — Daft Punk feat. Pharrell Williams     4:08   │
│                      ││ ▶ One More Time — Daft Punk                         5:20   │
│  NOW PLAYING         ││   Instant Crush — Daft Punk                         5:37   │
│  Get Lucky           ││   Lose Yourself to Dance — Daft Punk                5:53   │
╰──────────────────────╯╰──────────────────────────────────────────────────────────╯
╭──────────────────────────────────────────────────────────────────────────────────╮
│  ▶  Get Lucky  —  Daft Punk feat. Pharrell Williams                                │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  0:12 / 0:30   │
│  vol ━━━━━━━━────   autoplay on                                                     │
╰──────────────────────────────────────────────────────────────────────────────────╯
  ↑↓/jk move  •  enter play  •  space pause  •  n/b next/prev  •  +/- vol  •  q quit
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

- 🔎 **Live search** across Deezer — songs, artists, albums.
- ▶️ **Real audio playback** of 30-second previews: play, pause, stop.
- ⏭️ **Autoplay** — rolls into the next result when a track ends.
- 🔊 **Volume control**, live progress bar, and a now-playing panel.
- 🟢 **Spotify-green** accent theme throughout.
- 🧰 **Runs anywhere** — a silent fallback build needs no audio device at all.
- 🔐 **Zero credentials** — no API key, no login, no data stored.

## Quick start

```bash
# 1. Install the audio backend's build dependency (one time)
sudo apt-get install -y libasound2-dev      # Debian/Ubuntu

# 2. Build and run
make run
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
| `make run`        | Build with audio and launch.                                  |
| `make silent`     | Compile the no-audio fallback (`-tags nosound`).              |
| `make run-silent` | Build the silent fallback and launch.                         |
| `make test`       | Run unit + render tests (no device required).                 |
| `make vet`        | Static analysis.                                              |
| `make fmt`        | Format all Go sources.                                        |

## Keybindings

| Key             | Action                                  |
| --------------- | --------------------------------------- |
| `/`             | Focus the search box                    |
| `enter`         | (search) run search · (list) play       |
| `esc` / `tab`   | Toggle between search and results       |
| `↑`/`↓` `j`/`k` | Move selection                          |
| `space` / `p`   | Pause / resume                          |
| `s`             | Stop                                    |
| `n` / `b`       | Next / previous track                   |
| `+` / `-`       | Volume up / down                        |
| `a`             | Toggle autoplay                         |
| `g` / `G`       | Jump to top / bottom of results         |
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
