# Changelog

All notable changes to AudioPulse are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Smart shuffle (`S`)** — a from-scratch recommendation shuffle. With a playlist
  (or any track list) open, `S` builds a fresh queue of songs that fit that
  playlist's vibe but **aren't already in it**, then plays it. The open list is
  sampled as a taste seed, a local Ollama model suggests similar songs (same
  genres/era/mood), each is resolved to a playable track via Search, and any that
  are in fact already in the playlist are dropped. Also reachable by voice / the
  `:` prompt (*"smart shuffle this playlist"*, *"shuffle in some similar songs"*).
  Needs only Ollama — no library index, since the playlist is the seed. Replaces
  the old `S` stub that reported smart shuffle as unavailable.

### Fixed
- **AI assistant model auto-detect no longer picks an embedding model.** The
  first installed `gemma*` model was selected, but `embeddinggemma` is
  embedding-only and 400s on `/api/chat` — breaking `:` / voice / recommend /
  smart shuffle for anyone who had it installed without pinning `ollama_model`.
  Auto-detect now skips models whose name contains `embed`.

### Changed
- The visualizer bars are now thin (one cell wide, separated by a one-cell gap)
  for a cleaner spectrum look.
- The player-bar transport now uses monochrome text symbols (`⇄ |< > || >| ↻`,
  with `↻1` for loop-one) instead of color emoji. Emoji are drawn by the terminal
  in their own colors regardless of styling, so they could never show the green
  "active" state; the monochrome symbols respect the foreground color. Volume is
  shown as `vol N%`.

### Changed
- **Lower idle cost**: the player now polls **faster while playing (1s)** and
  **slower while paused/idle (4s)**, and the heavier up-next `Queue()` call is
  fetched only on track changes / playback actions plus a slow keep-alive instead
  of every second. Roughly halves steady-state API traffic and idle redraws
  (`docs/performance.md` #1–2).
- **librespot is now supervised**: if the playback device process crashes it is
  automatically restarted with exponential backoff (capped at 30s) for the life
  of the app, instead of playback silently dying. Logs append across restarts.
- **Playback recovers from a lost device**: a "device not found / no active
  device" error now re-resolves the "AudioPulse" Connect device by name and
  transfers playback back to it (handles librespot restarts and device drops),
  rather than every command failing against a stale device id.

### Fixed
- Export counts are now **per distinct song**, not per output line. spotDL retries
  failed lookups (printing the error several times) and re-reports the same song
  reached via multiple playlists, which massively inflated the "skipped" and
  "failed" tallies (e.g. 756 skipped / 795 failed when far fewer distinct songs
  were involved). A dedup tally keeps each song's best outcome
  (downloaded > already-have > not-found), so the numbers and the
  `_export-failures.txt` list are honest. Labels reworded to "new / already had /
  couldn't find".
- Saved podcasts now load at startup, so the Podcasts pane is populated even in
  the side-by-side layout where it's visible without being focused (previously it
  only loaded when you Tabbed into it). The empty state now explains that you need
  to **Follow a show in Spotify** for it to appear (the API's `/me/shows` only
  returns followed/saved shows).
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
- **Ask your library (multi-turn chat)** — ask the `:`/`v` assistant questions
  like *"what kind of music is in my library?"*, *"how many Radiohead songs do I
  have?"*, or *"which playlists have sad songs?"* and it opens a scrollable chat
  panel and answers — **grounded in your actual library** (it retrieves relevant
  tracks + your playlist names as context). Keep the conversation going with
  follow-ups (`↵` to ask, `↑↓` to scroll, `esc` to close); history is kept within
  the session. Runs entirely on your local model ([ADR-0015](docs/adr/0015-library-rag.md)).
- **AI recommendations grounded in your library** — ask the `:`/`v` assistant to
  *"recommend something like Daft Punk"*, *"suggest some chill study music"*, or
  *"play something like my workout playlist"* and it builds a queue and plays it.
  How it works, all **local**: your playlists + Liked Songs are indexed once into
  a semantic index (embeddings via your local `nomic-embed-text`, ~30 s for a few
  thousand tracks, cached at `~/.config/audiopulse/library-index.gob`); a request
  retrieves your closest-matching tracks as a *taste* signal, a local Gemma model
  suggests songs to **discover**, and each is resolved to a playable track via
  Spotify Search. (Spotify's own recommendation API is dead for newly-created
  apps — 403/404 — so this is fully library + LLM driven.) The index builds
  automatically on first use behind a progress overlay; say *"reindex my library"*
  to rebuild after big changes. New `ollama_embed_model` config (default
  `nomic-embed-text`) ([ADR-0015](docs/adr/0015-library-rag.md)).
- **`make install-voice`** — build with voice control + RAG (`-tags vosk`) and
  install to `~/.local/bin` so `audiopulse` runs from anywhere (set an absolute
  `voice_model` for `v` when launching outside the repo).
- **Voice control (offline, press `v`)** — speak your commands. A microphone
  capture (ffmpeg → PulseAudio) is transcribed by a **local** [Vosk](https://alphacephei.com/vosk/)
  model and the text is fed into the same assistant pipeline as `:` — so *"play
  bohemian rhapsody"*, *"shuffle on"*, or *"skip"* work spoken or typed. Press
  `v`, **wait for the green "Listening…" cue** (it shows "Starting mic…" until
  capture is actually live, so the mic's ~1 s startup can't clip your first word —
  the reason short commands like "pause"/"skip" were getting dropped), then speak;
  it finalizes ~0.6 s after you stop (transcript-stability endpoint, since
  terminals can't do hold-to-talk). Fully offline, no Python, no API. It's an
  **opt-in build**: `make voice` downloads the Vosk library + a small English
  model (gitignored, ~50 MB) into `third_party/vosk/` and builds with `-tags
  vosk`; the default build, tests, and CI need none of it. Configure the model
  path / capture source with `voice_model` / `voice_source`; `make doctor` checks
  ffmpeg, the Vosk files, and your mic ([ADR-0014](docs/adr/0014-voice-control-vosk.md)).
- **AI assistant (local, press `:`)** — control playback in plain language:
  *"play bohemian rhapsody"*, *"turn shuffle on"*, *"loop this song"*, *"skip"*,
  *"go back"*, *"pause"*, *"set the volume to 30"*. A floating prompt sends the
  request to a **local** [Ollama](https://ollama.com) model (auto-detects your
  first installed `gemma*` model; override with `ollama_model` / `ollama_url` in
  `config.json`) — **nothing leaves the machine**. The model returns a small JSON
  intent (Ollama JSON mode + a few-shot prompt — no fine-tuning, works across
  Gemma variants), which maps to the existing transport controls; "play" requests
  search Spotify and play the top hit. Optional and self-contained: if Ollama
  isn't running the prompt says so and nothing else is affected. Install a model
  with `make ollama-model`; `make doctor` checks Ollama's status
  ([ADR-0013](docs/adr/0013-local-nl-control.md)).
- **Export your library to local files** — press `e` to download your entire
  Spotify library (Liked Songs + all playlists) to local audio via
  [spotDL](https://github.com/spotDL/spotify-downloader). It gathers every track
  URI, shows a confirmation (count + destination), then runs a background,
  cancelable, **resumable** job with a live progress bar (downloaded / skipped /
  failed + current track). Install with `make spotdl` (checked by `make doctor`);
  destination is the `music_dir` config setting (default `~/Music/audiopulse`,
  point it at a drive). Podcasts aren't included — spotDL can't do them.
  The exporter writes `_export.log` (raw spotDL output) and `_export-failures.txt`
  (every track it couldn't find — mostly Spotify-exclusive Singles/Sessions/Live
  recordings, which simply aren't on YouTube) into the music dir, and a **stall
  watchdog** kills a batch that goes silent for 3 min (a throttled/hung download)
  so it can't freeze the whole run — those tracks are retried on the next run.
- **Like / unlike** — press `L` to save or remove the selected (or playing) track
  in your Liked Songs; the now-playing panel shows a `♥` when the current track is
  saved. **Unfollow** — press `F` on a highlighted show to unfollow it (the list
  refreshes). These use a new `user-library-modify` scope, so AudioPulse
  re-authorizes **once** on first launch after updating
  ([ADR-0012](docs/adr/0012-library-mutations.md)).
- **Keybinding cheatsheet** — press `?` for a floating overlay listing every
  shortcut (the bottom help line is truncated on narrow terminals). Any key closes.
- **Episode preview** — in the Podcasts pane, a show's episodes now load into the
  detail box **as you move the cursor** over the show list (debounced, without
  moving focus); `enter` still opens and focuses them. The first saved show is
  previewed automatically.
- **Add to queue** — press `a` on a selected track to queue it after the current
  one (shows in Up Next on the next poll). Track-only for now (the Web API queue
  helper doesn't take episode URIs).
- **Podcasts** — the center now shows **Music** and **Podcasts** side by side when
  the terminal is wide enough, and collapses to a single pane with a Music/Podcasts
  toggle (the chips) when narrow. The podcast pane is a **master/detail split**: your
  saved shows on top and the opened show's episodes below, both visible at once.
  `enter` on a show opens its episodes (and focuses them); `enter` on an episode
  plays it; `esc` moves focus back to the shows list. Episodes that are
  region-locked or externally hosted are marked `⊘` and dimmed.
  Powered by new `SavedShows`/`ShowEpisodes` client methods (no new OAuth scope —
  `user-library-read` already covers it) ([ADR-0010](docs/adr/0010-podcasts.md)).
  Note: librespot's podcast playback is best-effort — some episodes won't decode.
- **Lyrics panel** — a panel below "Your Library" shows the current track's
  lyrics, fetched from [lrclib.net](https://lrclib.net) (free, no auth; the
  Spotify Web API has no lyrics endpoint). When time-synced (LRC) lyrics are
  available the current line is highlighted in green and follows playback;
  otherwise plain lyrics are shown. Falls back gracefully to "No lyrics found"
  or "instrumental" ([ADR-0009](docs/adr/0009-lyrics-via-lrclib.md)).
- **Lyrics panel is Tab-focusable**, and pressing `enter` on it opens a
  **floating full-lyrics pane** centered over the UI with the words
  **word-wrapped** (no mid-word truncation like the narrow side panel). It
  scrolls (`↑↓`, `g`/`G`), keeps the current synced line highlighted, auto-follows
  playback (toggle with `f`), and closes with `esc`. `shift+tab` cycles panels
  backwards.
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
