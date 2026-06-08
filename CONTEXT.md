# AudioPulse — Working Context / Session Handoff

> A running context doc so work can resume across sessions. For user-facing docs
> see `README.md` and `docs/`. Last updated: 2026-06-07.

## TL;DR — where things stand

AudioPulse is a terminal music player in **Go + Bubble Tea**. It began as a
Deezer preview player and was pivoted to **play full songs from Spotify** (via an
embedded **librespot** device controlled through the Spotify Web API), with a
**Spotify-desktop-style UI**. Deezer remains as a no-login "guest" fallback.

- **Repo:** https://github.com/razeenwasif/audiopulse (public)
- **Local path:** `/home/amaterasu/AudioPulse`
- **Latest commit at handoff:** `8a44101` (Spotlight search). CI green.
- **Installed binary:** `~/.local/bin/audiopulse` (on PATH). Re-run `make install` after changes.
- **Platform:** WSL2 (Ubuntu) under **Windows Terminal**; Go 1.25, librespot 0.8.0.

To resume: `cd ~/AudioPulse`, read this file + `git log --oneline -20`, then
`make doctor` and `make test`.

## How to build / run / test

| Command | What |
| --- | --- |
| `make run` | Build (real audio) and launch — Spotify mode if a Client ID is configured, else Deezer guest. |
| `make run-silent` | `-tags nosound` build (no audio deps); simulates playback. |
| `make install` | Build + copy to `~/.local/bin/audiopulse`. **Do this after changes**, then relaunch. |
| `make librespot` | `cargo install librespot --locked --no-default-features --features "alsa-backend,rustls-tls-webpki-roots"` (one-time, done). |
| `make doctor` | Toolchain + audio + Spotify status check. |
| `make test` | `go test -tags nosound ./...` (no device/TTY needed). |
| `make fmt` / `make vet` | format / static analysis. |

> After `make install`, a **running** instance is still the old binary — tell the
> user to quit (`q`) and `hash -r && audiopulse`.

## Two run modes

- **Spotify mode** (when `SPOTIFY_CLIENT_ID` env or `~/.config/audiopulse/config.json` has a client_id): full songs via librespot + Web API.
- **Deezer guest mode** (no client id, or `AUDIOPULSE_GUEST=1`): 30s previews via `faiface/beep`.

`main.go` chooses the mode and does **all interactive auth before** entering the
alt-screen TUI.

## Architecture / packages

```
main.go                  entry; mode selection; cellAspect(); pre-TUI auth
term_{linux,other}.go    terminal cell aspect via TIOCGWINSZ (linux)
stderr_{linux,other}.go  redirect fd2 to $TMPDIR/audiopulse.log (keep ALSA noise off screen)
internal/config/         Client ID (env/config.json), paths, OAuth scopes, RedirectURI
internal/auth/           OAuth 2.0 PKCE, token cache ~/.config/audiopulse/token.json (0600), browser open
internal/spotify/        Web API wrapper over zmb3/spotify/v2 (control + metadata only)
internal/librespot/      supervise librespot child = Connect device "AudioPulse"
internal/deezer/         Deezer client (guest mode)
internal/player/         player_beep.go (real, default) + player_silent.go (-tags nosound)
internal/ui/
  model/update/view/styles.go      Deezer guest UI
  spotify_model/update/view/mouse  Spotify UI (3 panels + player bar)
  albumart.go                      cover art → 24-bit half-blocks
  styles.go                        palette, panelBox(focused), fillBG, solidify
docs/, docs/adr/                   user docs + Architecture Decision Records (0001–0007)
```

**Control vs audio split:** librespot decodes/plays audio (ALSA backend →
PulseAudio); the Web API only sends play/pause/seek/next/volume and reads
now-playing. The "AudioPulse" device is discovered via `WaitForDevice`.

## Environment specifics (so audio works on this machine)

- **WSL2 has no ALSA card.** Audio is routed ALSA → PulseAudio (WSLg) → Windows.
  - `~/.asoundrc` exists with `pcm.!default { type pulse }` / `ctl.!default { type pulse }`.
  - Needs `libasound2-dev` (build, installed) **and** `libasound2-plugins` (the ALSA→pulse bridge, installed by the user).
  - `PULSE_SERVER=unix:/mnt/wslg/PulseServer`.
- **librespot 0.8.0** at `~/.cargo/bin/librespot`. Built with `alsa-backend` + `rustls-tls-webpki-roots` (the `--no-default-features` strips TLS, which `librespot-oauth` requires; `webpki-roots` avoids a system OpenSSL dep). `--locked` was needed to avoid a transitive build break.
- **Spotify Client ID:** `d98c3f6fab33461a9c8f354f531b677b` in `~/.config/audiopulse/config.json`. Redirect URI in the Spotify app **must be exactly** `http://127.0.0.1:8888/callback`.
- **First run does two browser auths** (one-time): AudioPulse Web API OAuth, and librespot device login. Both cached under `~/.config/audiopulse/`.

### Env vars
- `SPOTIFY_CLIENT_ID` — enable Spotify mode.
- `AUDIOPULSE_GUEST=1` — force Deezer mode.
- `AUDIOPULSE_TRANSPARENT=1` — keep terminal transparency (default = opaque backdrop).
- `AUDIOPULSE_CELL_ASPECT` — album-art aspect; raise (e.g. 2.2) if art looks too tall, lower (1.8) if too wide. Default 2.0 (auto-detect usually fails on Windows Terminal).

## What was done today (chronological)

1. **Deezer TUI** (green accent, 30s previews, beep audio) + silent fallback build tag; full docs set, governance files, CI.
2. **WSL audio** fixed (`~/.asoundrc`, libasound2-plugins) + `make doctor` + stderr redirect so ALSA noise can't corrupt the TUI.
3. **`make install`** to run `audiopulse` from anywhere.
4. **Pivot to Spotify**: config/auth(PKCE)/spotify-client/librespot packages; Deezer kept as guest fallback (ADR-0005).
5. **Spotify-desktop UI redesign** (ADR-0006): top bar + search, "Your Library" two-line rows w/ thumbnails, center chips + track table, now-playing + album art, 3-zone player bar; **green-as-accent** (subtle borders, focused panel green).
6. **Album art** via 24-bit Unicode half-blocks; aspect-corrected (`AUDIOPULSE_CELL_ASPECT`).
7. **Opaque background** (`fillBG`) so the player is readable over Windows Terminal transparency.
8. **Mouse support** (issue #1, closed): wheel scroll, click library/track, click+drag progress bar to seek, click play/pause.
9. **Shuffle / repeat shortcuts**: `s`, `r` (loop-all), `R` (loop-one), with **green feedback**.
10. Bug fixes:
    - Library overflow pushed the player bar/help off-screen → windowed library + `clipLines` every panel + clamp fixed sections.
    - Glyph feedback didn't show → emoji ignore fg color; switched to **monochrome symbols** (`⇄ |< > || >| ↻`, `↻1`; `vol N%`).
    - Shuffle/repeat inverted-default + poll clobber → track **locally**, start off, don't seed from the (unreliable-for-librespot) API.
11. **Spotlight search** (ADR-0007): `/` opens a floating box composited over the UI (`x/ansi`), live debounced results, ↑↓/enter/esc.

## Gotchas & lessons (read before changing the UI)

- **Emoji ignore foreground color** — the terminal paints them in their own colors. For anything that needs a colored (e.g. green "active") state, use monochrome text/symbols, not emoji.
- **lipgloss `.Width()/.Height()` include padding, exclude border.** Use the `panelDims` helper to translate desired outer size.
- **lipgloss disables color in non-TTY tests.** To assert colors, `lipgloss.SetColorProfile(termenv.TrueColor)` in the test.
- **Panels must not overflow.** Every panel body is height-clipped (`clipLines`) and the library is scroll-windowed; the top bar/player bar/help are clamped to exact heights with `MaxWidth` slack — otherwise a long list or a wrapped line pushes the player bar + help off-screen.
- **Transparency:** Windows Terminal renders explicit-bg cells opaque; `fillBG` paints every cell. The *degree* of transparency is a terminal setting, not app-controllable.
- **Mouse hit-testing must match render geometry** — both use the shared helpers (`panelDims`, `trackWindow`, `centerGeom`, `progressMetrics`, `libVisible`). If you change layout offsets, update `spotify_mouse.go`.
- **Spotify Web API quirks:** shuffle/repeat aren't reliably reported for a librespot device (tracked locally instead); **smart shuffle has no Web API endpoint** (S shows a message); "No active device" is handled by transferring playback to the librespot device.
- **Visualizer is synthesized, not real audio** (ADR-0008): librespot never hands AudioPulse the PCM, so `vizLevels` generates the spectrum procedurally (sine layers advanced by `vizFrame`), driven only by play/pause. It animates on a separate ~8 fps `vizTickMsg` gated by `vizActive()` (playing **and** width ≥ 112), with a `vizTicking` guard; the loop ends on pause/hide and restarts from the 1 Hz poll. Right column is now two stacked **light-green-bordered** panels (`lightPanelBox`): Now Playing (`renderNowPlaying`) over Visualizer (`renderVisualizer`/`renderBars`). Real FFT would mean tapping the PulseAudio monitor source — the single seam to swap is `vizLevels`.
- **Pagination:** list endpoints return one page (Liked Songs ≤50, playlist items ≤100, playlists ≤50) — you must follow the cursor. Track lists are **streamed page-by-page in the background** (`LikedSongsPage`/`PlaylistTracksPage` + `beginTrackLoad`/`loadTrackPageCmd`) so the first page shows instantly and `Loading… N/Total` fills in; a `loadGen` token drops pages from a superseded source switch. Playlist offsets advance by the **raw** item count (unplayable items are filtered from the display but still consume a slot). Context-less playback (Liked/Recent/Search) sends a bounded 500-URI window (`maxPlayURIs`) since the play endpoint rejects huge arrays.
- **Standing user preferences (in agent memory):** (a) don't touch `/mnt/c` Windows host files unless asked; (b) **update all docs before every commit/push**.

## Performance backlog

Profiling-driven CPU/memory optimization ideas are documented (not yet
implemented) in **`docs/performance.md`** — idle-tick throttling, split Spotify
polling (State vs Queue), rendered-row caching, truncation/builder allocation
wins, an album-art LRU, smaller cover images, and more. Start there before any
perf work.

## Known open items / possible next steps

- Cover-art **card carousels** in the center feed (Spotify "Jump back in") — deferred; rendering many thumbnails per frame is heavy. A few cached small thumbs could work.
- Make **shuffle/repeat clickable** in the player bar (mouse).
- **256-color fallback** for album art (truecolor assumed today).
- Convert remaining decorative emoji `🔎` (search field) / `⏱` (column header) for consistency.
- Verify no **color bleed** at the Spotlight overlay seam on the user's terminal; add resets at splice points if seen.
- Optional: force shuffle/repeat to match local intent **on play** (covers the edge case where the account started shuffled).
- By design, external shuffle/repeat changes (e.g. from the phone) are **not** reflected (controller-style).
- Smart shuffle — blocked by the API.

## Key files to know

- `internal/ui/spotify_view.go` — all Spotify rendering, `panelDims`, `fillBG`, `solidify`, `overlay`, `renderSpotlight`, `renderTransport`, `progressMetrics`.
- `internal/ui/spotify_update.go` — key handling, Spotlight debounce, playback control commands.
- `internal/ui/spotify_mouse.go` — mouse hit-testing geometry (keep in sync with the view).
- `internal/ui/styles.go` — palette + `panelBox(focused)` + `fillBG`.
- `internal/spotify/client.go` — Web API surface.
- `internal/librespot/librespot.go` — child process lifecycle.
- `docs/adr/` — the "why" behind each major decision (0001–0007).
