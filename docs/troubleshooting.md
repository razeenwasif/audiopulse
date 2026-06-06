# Troubleshooting

Solutions to the problems you're most likely to hit, grouped by symptom.

## Contents

- [Build problems](#build-problems)
  - [`Package alsa was not found`](#package-alsa-was-not-found)
  - [`cannot find -lasound`](#cannot-find--lasound)
  - [cgo / C compiler errors](#cgo--c-compiler-errors)
- [Runtime problems](#runtime-problems)
  - [No sound (general)](#no-sound-general)
  - [No sound under WSL2](#no-sound-under-wsl2)
  - [`audio init failed`](#audio-init-failed)
  - [Search returns nothing / network errors](#search-returns-nothing--network-errors)
  - [Garbled layout or colours](#garbled-layout-or-colours)
  - [The UI says the terminal is too small](#the-ui-says-the-terminal-is-too-small)
- [Diagnostics](#diagnostics)

## Build problems

### `Package alsa was not found`

```
# github.com/hajimehoshi/oto
# pkg-config --cflags -- alsa
Package alsa was not found in the pkg-config search path.
```

The ALSA development headers are missing. Install them:

```bash
sudo apt-get install -y libasound2-dev    # Debian/Ubuntu
sudo dnf install alsa-lib-devel           # Fedora/RHEL
```

If you cannot install system-wide, either build the
[silent backend](getting-started.md#the-silent-build) (`make run-silent`) or
stage the headers locally as described in
[Development → Working without ALSA headers](development.md#working-without-alsa-headers).

### `cannot find -lasound`

The headers are present but the linker can't find the shared library, usually
because the `libasound.so` dev symlink is missing (only `libasound.so.2` exists).
Installing `libasound2-dev` provides the symlink. If you staged headers manually,
point the linker at the real library:

```bash
ln -sf /lib/x86_64-linux-gnu/libasound.so.2.0.0 <staging>/libasound.so
```

### cgo / C compiler errors

The audio backend uses cgo, which needs a C compiler. Install one:

```bash
sudo apt-get install -y build-essential   # Debian/Ubuntu
```

Or sidestep cgo entirely with the silent backend, which is pure Go:

```bash
go build -tags nosound -o audiopulse .
```

## Runtime problems

### No sound (general)

Work through these in order:

1. **Is this the silent build?** `make run-silent` and `-tags nosound` never
   produce sound by design. Rebuild with `make build`/`make run`.
2. **Does other audio work?** Test your stack independently:
   ```bash
   speaker-test -t sine -f 440 -l 1     # Ctrl+C to stop
   ```
3. **Is volume up?** Press `+` a few times in-app; check your system mixer.
4. **Is a device available?** See [`audio init failed`](#audio-init-failed).

### No sound under WSL2

WSL2 has **no ALSA sound card**, which is what AudioPulse's audio backend (via
`oto`) opens by default — hence errors like:

```
audio init failed: ... oto: ALSA error: No such file or directory
ALSA lib confmisc.c:855:(parse_card) cannot find card '0'
ALSA lib pcm.c:2721:(snd_pcm_open_noupdate) Unknown PCM default
```

However, **WSLg ships a PulseAudio server** (at `/mnt/wslg/PulseServer`, with
`$PULSE_SERVER` already set). The fix is to route ALSA's `default` device through
PulseAudio:

1. **Install the ALSA→PulseAudio bridge plugin:**

   ```bash
   sudo apt-get update
   sudo apt-get install -y libasound2-plugins
   ```

2. **Tell ALSA to use PulseAudio by default.** Create `~/.asoundrc`:

   ```
   pcm.!default { type pulse }
   ctl.!default { type pulse }
   ```

3. **Run AudioPulse** — `make run`. Audio now flows WSL → PulseAudio → Windows.

To confirm the chain works without launching the full app:

```bash
go test ./internal/player/ -run TestSpeakerInitLive -v
# PASS  →  the speaker opened successfully
```

If you only need the interface and not sound, the
[silent build](getting-started.md#the-silent-build) (`make run-silent`) avoids
the audio stack entirely.

> Even before this fix, AudioPulse does **not** crash on audio failure — it shows
> `audio init failed` in the status line and keeps working. The raw ALSA text is
> redirected to a log file (see [Diagnostics](#diagnostics)) so it cannot corrupt
> the UI.

### `audio init failed`

Shown in the status line when the speaker can't be initialised (no device, busy
device, or no audio server). AudioPulse treats this as **non-fatal** — the UI
keeps working without sound. To fix:

- Ensure an audio server is running (PulseAudio/PipeWire on Linux).
- Free the device if another application holds it exclusively.
- Under WSL2, see [No sound under WSL2](#no-sound-under-wsl2).

### Search returns nothing / network errors

- **`No playable results`**: the query matched nothing with a preview. Try a
  broader or differently spelled query.
- **A network error in the status line**: check connectivity to
  `https://api.deezer.com`. Behind a proxy, set `HTTPS_PROXY`.
  ```bash
  curl -s 'https://api.deezer.com/search?q=test&limit=1' | head -c 200
  ```
- Requests time out after 15 seconds rather than hanging.

### Garbled layout or colours

- Ensure a UTF-8 locale (box-drawing and note glyphs require it):
  ```bash
  locale    # LANG/LC_* should be UTF-8, e.g. en_US.UTF-8
  ```
- Use a terminal with truecolor support for the intended palette; set
  `COLORTERM=truecolor` if your terminal supports it but doesn't advertise it.
- For a monochrome fallback, set `NO_COLOR=1`.
- A font with good Unicode coverage (a Nerd Font or any modern monospace) avoids
  missing-glyph boxes.

### The UI says the terminal is too small

AudioPulse needs at least **64×18** characters. Enlarge the window or reduce the
font size; the UI redraws automatically on resize.

## Diagnostics

Native-library output (notably ALSA) is redirected away from the screen to a log
file so it can't corrupt the UI. Check it first when debugging audio:

```bash
cat "${TMPDIR:-/tmp}/audiopulse.log"
```

Useful commands when filing a bug report:

```bash
go version                       # Go toolchain
uname -a                         # kernel / platform
pkg-config --modversion alsa     # ALSA dev version (Linux)
echo "$PULSE_SERVER"             # PulseAudio server (Linux/WSL)
echo "$TERM / $COLORTERM / $LANG"
go test ./internal/player/ -run TestSpeakerInitLive -v   # can audio open?
```

If the problem persists, open an issue with the above output and clear
reproduction steps. For anything security-related, follow
[SECURITY.md](../SECURITY.md) instead of opening a public issue.
