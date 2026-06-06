# Getting Started

This guide takes you from a clean machine to a running AudioPulse.

## Contents

- [Prerequisites](#prerequisites)
- [Installation by platform](#installation-by-platform)
  - [Debian / Ubuntu](#debian--ubuntu)
  - [Fedora / RHEL](#fedora--rhel)
  - [Arch Linux](#arch-linux)
  - [macOS](#macos)
  - [Windows (WSL2)](#windows-wsl2)
- [Building and running](#building-and-running)
- [The silent build](#the-silent-build)
- [Verifying your installation](#verifying-your-installation)
- [Next steps](#next-steps)

## Prerequisites

| Requirement      | Version | Notes                                              |
| ---------------- | ------- | -------------------------------------------------- |
| Go               | 1.22+   | <https://go.dev/dl/>                               |
| C toolchain      | any     | `gcc`/`clang`; required by the audio backend (cgo) |
| ALSA dev headers | —       | Linux only; required to compile the audio backend  |
| Audio output     | —       | PulseAudio, PipeWire, or ALSA at runtime           |
| `make`           | any     | Optional but recommended                           |

> The C toolchain and ALSA headers are needed **only** for the real-audio build.
> The [silent build](#the-silent-build) requires neither.

## Installation by platform

### Debian / Ubuntu

```bash
sudo apt-get update
sudo apt-get install -y golang-go build-essential libasound2-dev
```

### Fedora / RHEL

```bash
sudo dnf install -y golang gcc make alsa-lib-devel
```

### Arch Linux

```bash
sudo pacman -S --needed go gcc make alsa-lib
```

### macOS

The default Core Audio backend used by `oto` needs no extra system packages.

```bash
brew install go
xcode-select --install   # if you don't already have the command-line tools
```

### Windows (WSL2)

AudioPulse runs in WSL2. Install the Linux dependencies as for
[Debian/Ubuntu](#debian--ubuntu). Audio from WSL2 requires a working PulseAudio/
PipeWire bridge to the Windows host; if you do not have one, use the
[silent build](#the-silent-build). See
[Troubleshooting → WSL audio](troubleshooting.md#no-sound-under-wsl2).

## Building and running

```bash
git clone <repository-url> AudioPulse
cd AudioPulse

make run            # compiles with audio and launches
```

Without `make`:

```bash
go build -o audiopulse .
./audiopulse
```

The first build downloads Go module dependencies; subsequent builds are cached.

## Installing on your PATH

To run `audiopulse` from any directory, install it to a `bin` directory on your
`PATH`:

```bash
make install                       # installs to ~/.local/bin by default
```

`make install` reports whether the target directory is on your `PATH` and, if
not, prints the line to add to your shell config. To install elsewhere (system
wide), override the prefix:

```bash
sudo make install PREFIX=/usr/local # installs to /usr/local/bin
```

Remove it again with:

```bash
make uninstall                      # honour the same PREFIX you installed with
```

## Spotify mode (full songs)

By default — with no configuration — AudioPulse runs in **Deezer guest mode**
(30-second previews, no login). To play **full songs** from your own Spotify
**Premium** account:

### 1. Install the playback backend (one-time)

AudioPulse plays Spotify audio through an embedded **librespot** device:

```bash
make librespot      # cargo build, ~10-15 minutes the first time
```

Requires the Rust toolchain (`cargo`) and the ALSA dev headers (already needed
for the audio build).

### 2. Create a Spotify app and copy the Client ID

1. Open the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard) → **Create app**.
2. Add this Redirect URI **exactly**: `http://127.0.0.1:8888/callback`.
3. Copy the **Client ID**. (The Client *Secret* is **not** needed — AudioPulse uses PKCE.)

### 3. Give AudioPulse the Client ID

Either set an environment variable:

```bash
export SPOTIFY_CLIENT_ID=your_client_id_here
```

or create `~/.config/audiopulse/config.json`:

```json
{ "client_id": "your_client_id_here" }
```

### 4. Run

```bash
make run
```

On first run you authorize **twice** in the browser (a one-time step): once for
AudioPulse's Web API access, and once for the librespot playback device. After
that, sign-in is remembered at `~/.config/audiopulse/`.

> Force Deezer guest mode anytime with `AUDIOPULSE_GUEST=1 audiopulse`.

## The silent build

If you cannot install the ALSA headers, have no audio device, or are running in
CI, build the **silent backend**. It compiles with no native dependencies and
fully simulates playback — the progress bar advances, and pause/resume and
autoplay behave normally — but produces no sound.

```bash
make run-silent
# or
go build -tags nosound -o audiopulse .
./audiopulse
```

See [ADR-0004](adr/0004-build-tag-fallback-strategy.md) for why this exists.

## Verifying your installation

```bash
make doctor         # checks the toolchain + audio stack and tests sound output
make test           # unit + render tests; should report "ok"
```

`make doctor` is the fastest way to confirm audio is set up correctly: it
reports the Go/cgo toolchain, ALSA headers, the PulseAudio runtime, and whether
the speaker can actually be opened — with a targeted fix for anything missing.

Then launch the app, press `/`, type an artist (e.g. `daft punk`), press
`Enter`, select a track with `↓`, and press `Enter` to play. You should see the
now-playing bar populate and the progress bar advance.

If the terminal is too small you will see a prompt to resize — AudioPulse needs
at least **64×18** characters.

## Next steps

- Learn every control in the **[User Guide](user-guide.md)**.
- Hit a problem? See **[Troubleshooting](troubleshooting.md)**.
- Want to contribute? Start with **[Development](development.md)**.
