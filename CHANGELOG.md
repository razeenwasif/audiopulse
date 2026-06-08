# Changelog

All notable changes to AudioPulse are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- The player-bar transport now uses monochrome text symbols (`⇄ |< > || >| ↻`,
  with `↻1` for loop-one) instead of color emoji. Emoji are drawn by the terminal
  in their own colors regardless of styling, so they could never show the green
  "active" state; the monochrome symbols respect the foreground color. Volume is
  shown as `vol N%`.

### Fixed
- Playlists and Liked Songs now list **all** their tracks. The Web API client
  followed only the first response page, so any playlist or the saved-tracks
  collection was capped at one page (≤100 / 50 items). Tracks now **stream in the
  background**: the first page renders immediately and the center panel shows
  `Loading… N/Total` while the remaining pages fill in (`LikedSongsPage` /
  `PlaylistTracksPage` in `internal/spotify/client.go`, streamed by
  `beginTrackLoad`/`loadTrackPageCmd` in `internal/ui`), bounded by a safety cap.
  Switching sources mid-stream cancels the stale stream (a load generation token).
  The sidebar playlist list is likewise fully paginated. Playback for
  context-less sources (Liked Songs / Recent / Search) sends a bounded window of
  track URIs starting at the selection, since Spotify's play endpoint rejects
  very large URI arrays.
- Shuffle/repeat glyphs now reflect the keypress immediately and stay lit. They
  start off and are tracked purely as local intent, instead of being read from
  the Web API, which doesn't reliably report these for a librespot device — that
  was both snapping the glyphs back to gray and seeding them in the wrong state
  (green-by-default when shuffle was actually off).
- The library list is now scroll-windowed and every panel is height-clipped, so a
  long playlist list no longer overflows and pushes the player bar and help line
  off-screen (which had hidden the shuffle/repeat feedback and key hints).
- The top bar, player bar, and help line are clamped to their exact heights and
  given horizontal slack so a glyph that a terminal sizes differently than
  computed can't make a line wrap and grow the layout past the screen.

### Added
- **CAVA-style visualizer** — a green spectrum panel sits below Now Playing on the
  right, animating while a track plays and flattening when paused. AudioPulse never
  sees librespot's decoded PCM, so the spectrum is a synthesized animation driven by
  playback state rather than a true FFT (a real one would need to tap the PulseAudio
  monitor source). It animates on a ~8 fps tick that runs only while playing and the
  right column is visible ([ADR-0008](docs/adr/0008-synthesized-visualizer.md)).
- **Light-green panel borders** on the Now Playing and Visualizer panels, setting
  the right column apart from the green-on-focus library/center panels.
- **Spotlight search** — `/` opens a floating, macOS-Spotlight-style search box
  that overlays the UI (composited with `x/ansi`), with live debounced results;
  `↑`/`↓` select, `enter` plays (and loads results into the center), `esc` closes
  ([ADR-0007](docs/adr/0007-spotlight-search-overlay.md)).
- **Shuffle / repeat shortcuts with feedback** — `s` toggles shuffle, `r` toggles
  loop-all (repeat context), and `R` toggles loop-one (repeat track). The player
  bar symbols (`⇄` shuffle, `↻`/`↻1` repeat) turn green when active. Smart shuffle
  has no Web API endpoint, so `S` shows an explanation instead.
- **Spotify-desktop visual redesign** — a top bar with a centered search field,
  two-line library rows with colored thumbnails and subtitles, a center feed with
  filter chips and a numbered track table, a 3-zone player bar with centered
  transport controls and a circular play button, and album art in the
  now-playing panel. Borders are now subtle gray, with green reserved as an
  accent that highlights the focused panel and active controls
  ([ADR-0006](docs/adr/0006-spotify-desktop-ui.md)).
- **Mouse support** — scroll wheel moves the selection in the panel under the
  pointer; click a library entry to open it; click a track to play it; click or
  drag the progress bar to seek; click the play/pause indicator to toggle.
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
  (downloaded and cached per cover URL). Art is kept square using the terminal's
  cell aspect ratio (auto-detected from pixel size, overridable with
  `AUDIOPULSE_CELL_ASPECT`).
- **Spotify search** — press `/` to search the catalogue and play results.
- **Opaque background** by default so the player is readable over translucent /
  acrylic terminals; set `AUDIOPULSE_TRANSPARENT=1` to keep transparency.
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
