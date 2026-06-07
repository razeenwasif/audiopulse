# Changelog

All notable changes to AudioPulse are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Spotify mode** — full-song playback from a Spotify Premium account:
  - OAuth 2.0 PKCE sign-in with a local loopback callback; token cached at
    `~/.config/audiopulse/token.json` (`0600`) and auto-refreshed.
  - Embedded **librespot** playback device ("AudioPulse"), supervised as a child
    process and controlled via the Spotify Web API.
  - Desktop-style three-panel layout (library · track feed · now-playing + queue)
    with a bottom player bar; transport controls for play/pause, next/prev, seek,
    volume, shuffle, and repeat.
  - `make librespot` target to build/install the playback backend.
  - `SPOTIFY_CLIENT_ID` / `~/.config/audiopulse/config.json` configuration and an
    `AUDIOPULSE_GUEST` override to force Deezer mode.
- **Album art** in the now-playing panel, rendered as 24-bit Unicode half-blocks
  (downloaded and cached per cover URL).
- **Spotify search** — press `/` to search the catalogue and play results.
- Deezer preview playback is retained as an automatic **no-login guest mode**.
- `make doctor` now also reports librespot, Client ID, and sign-in status.
- `make install` / `make uninstall` targets: install the binary to a `PATH`
  directory (default `~/.local/bin`, overridable via `PREFIX`).

## [0.1.0] - 2026-06-06

Initial release.

### Added
- Terminal UI built with Bubble Tea featuring a sidebar, results list, search
  box, now-playing bar, and contextual help line.
- Deezer-backed catalogue search (no API key or login required), returning up to
  50 results filtered to those with a playable preview.
- Real audio playback of 30-second previews via `faiface/beep`:
  play, pause, resume, stop, next/previous, and volume control.
- Autoplay: automatically advances to the next result when a track finishes
  (toggleable at runtime).
- Live progress bar and volume meter polled at 2 Hz.
- Spotify-green accent theme across all panels.
- Silent fallback backend (`-tags nosound`) that simulates playback with no
  audio dependencies, for headless/CI environments.
- Native stderr (e.g. ALSA diagnostics) is redirected to `$TMPDIR/audiopulse.log`
  on Linux so library noise can never corrupt the full-screen UI.
- Unit and render tests for the UI, plus an opt-in live integration test for the
  Deezer client.
- Documentation set: getting started, architecture, user guide, development,
  configuration, troubleshooting, and Architecture Decision Records.
- `Makefile` targets for build, run, silent build, test, vet, and format.
- Continuous integration workflow (build, vet, gofmt, test).

[Unreleased]: https://github.com/razeenwasif/audiopulse/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/razeenwasif/audiopulse/releases/tag/v0.1.0
