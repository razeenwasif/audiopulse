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

## AI assistant (local Ollama/Gemma)

Press `:` to open a prompt and control playback in plain language — "play
bohemian rhapsody", "turn shuffle on", "loop this song", "skip", "pause", "set
the volume to 30". The request is interpreted by a **local** model served by
[Ollama](https://ollama.com); nothing leaves your machine
([ADR-0013](adr/0013-local-nl-control.md)).

Setup (all optional — every other feature works without it):

1. Install Ollama and start it (`ollama serve`).
2. Pull a model: `make ollama-model` (defaults to `gemma3`; override with
   `make ollama-model OLLAMA_MODEL=gemma3:12b`), or `ollama pull gemma3`.
3. Run AudioPulse and press `:`.

By default the assistant auto-detects the first installed `gemma*` chat model
(embedding-only models such as `embeddinggemma` and `nomic-embed-text` are
skipped — they can't serve chat). Pin a specific model or point at a remote
Ollama in `~/.config/audiopulse/config.json`:

```json
{ "client_id": "…", "ollama_model": "gemma3:12b", "ollama_url": "http://localhost:11434" }
```

| Field          | Default                  | Meaning                                            |
| -------------- | ------------------------ | -------------------------------------------------- |
| `ollama_model` | auto-detect first gemma* | Which local model interprets your requests         |
| `ollama_url`   | `http://localhost:11434` | The Ollama HTTP endpoint                           |
| `ollama_embed_model` | `nomic-embed-text` | Embedding model for the library index (recommendations) |

`make doctor` reports whether Ollama is installed, reachable, and has a model.

### Library recommendations (RAG)

Ask the assistant to *"recommend something like Daft Punk"* or *"suggest chill
focus music"* and it plays a fresh queue. The first such request indexes your
library once (playlists + Liked Songs → local embeddings, cached at
`~/.config/audiopulse/library-index.gob`); later requests are fast. Say *"reindex
my library"* to rebuild after adding playlists. It needs the embedding model:
`ollama pull nomic-embed-text` (or set `ollama_embed_model` to one you have).
Spotify's own recommendation API is unavailable to new apps, so suggestions are
generated locally from your taste and resolved to playable tracks via Search
([ADR-0015](adr/0015-library-rag.md)).

### Create a playlist

Ask the assistant to *"create a playlist of the top classics from the 90s to early
2000s"* (or *"make me a playlist for studying"*) and it curates a themed tracklist
with the local model, **creates and saves a real playlist** on your account, adds
the resolved tracks, and plays it. Distinct from *recommend*, which only plays a
temporary queue. Uses the `playlist-modify-public` / `playlist-modify-private`
scopes (shared with add-to-playlist; existing users re-authorize once — see
[ADR-0012](adr/0012-library-mutations.md)). No library index needed.

### Organize Liked Songs by genre

Ask *"group every song in my Liked Songs into playlists by genre"* and AudioPulse
groups your saved tracks into coarse genre buckets (genre comes from each track's
primary **artist** — Spotify doesn't tag tracks), previews the playlists it would
create, and on your confirmation makes one playlist per genre (`Liked: Rock`,
`Liked: Hip-Hop`, …). Deterministic — the model only routes the request. Buckets
smaller than 4 tracks fold into `Liked: Other`. Reuses the `playlist-modify-*`
scopes ([ADR-0016](adr/0016-genre-organize.md)).

### Smart shuffle

Press **`S`** with a playlist (or any track list) open to build a *smart shuffle*:
a fresh queue of songs that fit that playlist's vibe but **aren't already in it**.
The open playlist is sampled as the taste seed, the local model suggests similar
songs, each is resolved via Search, and tracks already in the playlist are
filtered out. It works on whatever you're viewing (a playlist, Liked Songs, search
results) and needs only Ollama — **no library index**, since the playlist itself
is the seed. You can also trigger it by voice/`:` (*"smart shuffle this
playlist"*). It reuses the assistant's chat model (`ollama_model`).

## Voice control (offline Vosk)

Press `v` to **speak** a command instead of typing it at the `:` prompt — the
microphone is transcribed by a **local** [Vosk](https://alphacephei.com/vosk/)
model and the text is fed into the same assistant pipeline
([ADR-0014](adr/0014-voice-control-vosk.md)). Speak after pressing `v`; capture
stops automatically when you pause.

This is an **opt-in build** (it needs a C toolchain, the Vosk native library, and
a model — none required for the normal build):

```sh
make voice        # downloads libvosk + a small English model, builds with -tags vosk
./audiopulse      # press v and talk
```

`make voice` fetches `libvosk.so` + the model into `third_party/vosk/`
(gitignored, ~50 MB) and links them with an embedded rpath, so no
`LD_LIBRARY_PATH` is needed.

To **install** a voice-enabled binary on your `PATH` (so you can run `audiopulse`
from anywhere), use `make install-voice` (builds with `-tags vosk` and installs
to `~/.local/bin`). Note: when launched **outside the repo**, set `voice_model`
to an **absolute** path (the embedded libvosk rpath and the model both live under
the repo's `third_party/vosk/`, so the repo must stay put). Everything else
(Spotify, the RAG index) uses the absolute config dir and works from anywhere.

Tune voice in `~/.config/audiopulse/config.json`:

```json
{ "client_id": "…", "voice_model": "third_party/vosk/model", "voice_source": "default" }
```

| Field          | Default                   | Meaning                                                     |
| -------------- | ------------------------- | ---------------------------------------------------------- |
| `voice_model`  | `third_party/vosk/model`  | Path to the Vosk model (swap in a larger one for accuracy) |
| `voice_source` | `default`                 | PulseAudio capture source (`RDPSource` under WSLg)         |

On **WSL2** the Windows mic is forwarded by WSLg as the `RDPSource` PulseAudio
source; the `default` source resolves to it. `make doctor` checks ffmpeg, the
Vosk files, and whether your capture source is present (and warns if it's muted).

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
