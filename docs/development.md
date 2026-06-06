# Development Guide

How to set up, build, test, and extend AudioPulse.

## Contents

- [Environment](#environment)
- [Project layout](#project-layout)
- [Build tags and backends](#build-tags-and-backends)
- [Make targets](#make-targets)
- [Testing](#testing)
- [Code style](#code-style)
- [Working without ALSA headers](#working-without-alsa-headers)
- [Adding a feature](#adding-a-feature)
- [Releasing](#releasing)

## Environment

See [Getting Started → Prerequisites](getting-started.md#prerequisites). In
short: Go 1.25+, a C toolchain, and (on Linux, for the audio backend) the ALSA
development headers.

```bash
git clone <repository-url> AudioPulse
cd AudioPulse
make test           # fastest way to confirm a working toolchain
```

## Project layout

```
main.go                       Program entry point
go.mod / go.sum               Module definition and checksums
Makefile                      Build/test/format targets

internal/
  config/                     Client ID resolution + cache paths (Spotify)
  auth/                       Spotify OAuth 2.0 PKCE + token cache
  spotify/                    Spotify Web API wrapper (zmb3/spotify)
  librespot/                  librespot device supervisor
  deezer/
    deezer.go                 Deezer API client (Search) — guest mode
    deezer_live_test.go       Opt-in live API test (skipped with -short)
  player/
    player_beep.go            Real audio backend            (build: !nosound)
    player_silent.go          Silent simulation backend     (build: nosound)
  ui/
    model.go / update.go / view.go / styles.go   Deezer guest UI
    spotify_model.go / spotify_update.go / spotify_view.go   Spotify UI
    *_test.go                 Render and interaction tests

docs/                         This documentation set
```

`internal/` is used deliberately: nothing here is a supported public API.

## Build tags and backends

The audio layer has two implementations chosen at compile time:

| Build                         | File               | Audio | Native deps        |
| ----------------------------- | ------------------ | ----- | ------------------ |
| default                       | `player_beep.go`   | yes   | cgo + ALSA/CoreAudio |
| `-tags nosound`               | `player_silent.go` | no    | none               |

Both files declare the **same exported `Player` API**, guarded by
`//go:build !nosound` and `//go:build nosound` respectively. The rest of the
codebase imports `internal/player` and is unaware of which is compiled in. This
keeps the UI testable and the project buildable on machines without audio.

Rationale: [ADR-0004](adr/0004-build-tag-fallback-strategy.md).

## Make targets

| Target            | Command                                  |
| ----------------- | ---------------------------------------- |
| `make build`      | `go build -o audiopulse .`               |
| `make run`        | build (audio) and launch                 |
| `make silent`     | `go build -tags nosound -o audiopulse .` |
| `make run-silent` | build (silent) and launch                |
| `make test`       | `go test -tags nosound ./...`            |
| `make vet`        | `go vet -tags nosound ./...`             |
| `make fmt`        | `gofmt -w .`                             |
| `make clean`      | remove the built binary                  |

## Testing

```bash
make test                                   # unit + render tests, no device
go test ./internal/deezer/ -run Live -v     # live Deezer API (network required)
```

**Testing philosophy**

- **UI tests drive the model, not a terminal.** Tests construct the `Model`, send
  it `tea.Msg` values (window size, key presses, synthetic result messages), and
  assert on resulting state and on the string returned by `View()`. This gives
  fast, deterministic coverage of input handling and layout with no TTY.
- **The silent backend is the test backend.** Running tests with `-tags nosound`
  means they need neither audio hardware nor ALSA headers, so they pass in CI and
  on any contributor's machine.
- **The live test is opt-in.** `TestSearchLive` hits the real API and is skipped
  under `-short` (and in CI). Run it when you change the Deezer client.

When adding behaviour to `Update`, add a test that sends the relevant message and
asserts the state transition. When adding to `View`, add a render assertion.

## Code style

- `gofmt` is mandatory; `make fmt` before every commit. CI rejects unformatted
  code.
- `go vet` must be clean.
- Exported identifiers have doc comments.
- Wrap errors with context using `%w`.
- Respect the player lock ordering documented in
  [Architecture → Concurrency model](architecture.md#concurrency-model).

## Working without ALSA headers

If you cannot install `libasound2-dev` (e.g. no root) but still want to verify
that the **real** backend *compiles*, you can stage the headers locally and point
`pkg-config` at them. This is a development convenience only:

```bash
# Download and extract the dev package without installing it system-wide
cd /tmp && apt-get download libasound2-dev
dpkg-deb -x libasound2-dev_*.deb alsadev

# Point the local .pc at the extracted headers + the system runtime lib,
# and make the linker symlink resolvable
sed -i 's#^prefix=.*#prefix=/tmp/alsadev/usr#' \
    alsadev/usr/lib/x86_64-linux-gnu/pkgconfig/alsa.pc
ln -sf /lib/x86_64-linux-gnu/libasound.so.2.0.0 \
    alsadev/usr/lib/x86_64-linux-gnu/libasound.so

# Build against it
cd -
PKG_CONFIG_PATH=/tmp/alsadev/usr/lib/x86_64-linux-gnu/pkgconfig \
    go build -o /tmp/audiopulse .
```

For day-to-day work, prefer the silent backend (`make run-silent`) or install the
package properly.

## Adding a feature

A typical change touches three layers:

1. **State** — add fields to `Model` (`model.go`).
2. **Behaviour** — handle new keys/messages in `Update` (`update.go`), adding a
   command if I/O is involved (`model.go`).
3. **Presentation** — render the new state in `View` (`view.go`), styling via
   `styles.go`.

Then: add tests, run `make fmt vet test`, and note the change in
`CHANGELOG.md`. See [Architecture → Extension points](architecture.md#extension-points)
for the larger seams (alternative catalogues and audio sources).

## Releasing

1. Update `CHANGELOG.md`: move `[Unreleased]` entries under a new version with a
   date.
2. Ensure `make fmt vet test` and a real `make build` succeed.
3. Tag the release (`vMAJOR.MINOR.PATCH`) following
   [Semantic Versioning](https://semver.org/).
