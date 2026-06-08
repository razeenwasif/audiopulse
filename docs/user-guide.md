# User Guide

A complete tour of AudioPulse's interface and controls.

> AudioPulse has two modes. **Spotify mode** (when a Client ID is configured)
> shows the desktop-style interface described just below. **Deezer guest mode**
> (no login) shows the simpler search/preview interface documented in the rest of
> this guide. See [Getting Started → Spotify mode](getting-started.md#spotify-mode-full-songs).

## Spotify mode interface

```
 ⌂  ♫ AudioPulse            🔎  What do you want to play?            Razeen ▾   ← top bar
╭ Your Library ─────╮╭ Music ────────╮╭ Podcasts ─────╮╭ Now Playing ──────────╮
│  ██  Liked Songs  ││ Chill Vibes   ││ Your Shows    ││   ▀▀ album art ▀▀      │
│  ██  Playlist     ││ 1 Midnight…   ││ ▶ The Daily   ││ Instant Crush         │
│ ▌██  Chill Vibes  ││ ♪ Instant…    ││   Reply All   ││ Daft Punk             │
╰───────────────────╯│               ││               │╰── Up Next ────────────╯
╭ Lyrics ───────────╮│               ││               │╭ Visualizer ───────────╮
│ The city's a mess ││               ││               ││ ▃ ▅ ▂ ▆ █ ▄ ▇ ▃ ▅ ▁ ▆ │
│ but you're my home││               ││               ││ █ ▄ ▂ ▇ ▅ ▃ ▆ ▄ ▁ ▅ █ │
╰───────────────────╯╰───────────────╯╰───────────────╯╰───────────────────────╯
╭ Instant Crush — Daft Punk    ⇄  |<  ||  >|  ↻                  vol 65% ────╮
│ 1:12 ━━━━━━━━●──────────────────────────────────────────────────────  5:37 │
╰────────────────────────────────────────────────────────────────────────────╯
```

- **Top bar** — brand, a centered search field (press `/` to focus it), and your
  account.
- **Your Library** (top left) — Liked Songs, Recently Played, and your playlists,
  each with a colored thumbnail, title, and subtitle. The focused panel has a
  green border.
- **Lyrics** (bottom left) — lyrics for the current track, from
  [lrclib.net](https://lrclib.net) (the Spotify API has no lyrics endpoint). When
  time-synced lyrics exist the current line is highlighted in green and scrolls
  with playback; otherwise plain lyrics are shown. Shows "No lyrics found" or
  "instrumental" when appropriate. Press `tab` to focus it, then `enter` to open
  a **floating full-lyrics pane** where the words wrap instead of being cut off
  (scroll with `↑↓`/`g`/`G`, `f` toggles synced auto-follow, `esc` closes).
- **Center** — **Music** and **Podcasts** side by side when the terminal is wide
  enough; on narrower terminals it becomes a single pane with a **Music/Podcasts**
  toggle (the chips — click one or `tab` to it).
  - **Music** — the selected library source's tracks. The playing track is marked
    `♪`; the selected row is green.
  - **Podcasts** — your saved shows. `enter` opens a show's episodes, `enter` on
    an episode plays it, `esc` goes back to the show list. Episodes that are
    region-locked or hosted off Spotify are marked `⊘` and dimmed (and may not
    play — see below).
- **Now Playing** (top right) — album art, track/artist/album, and the up-next
  queue, in a light-green-bordered panel.
- **Visualizer** (bottom right) — a CAVA-style green spectrum that animates while
  a track plays and settles to a flat line when paused, also light-green-bordered.
  It's a synthesized animation: AudioPulse controls librespot but never sees the
  decoded audio, so the bars are driven by the play/pause state rather than a true
  FFT of the sound.
- **Player bar** (bottom) — the current track, centered transport controls
  (shuffle · prev · play/pause · next · repeat) with a green play button, and a
  full-width progress bar with volume.

**Controls:** `tab`/`shift+tab` cycle the focused panel (Library → Music → Podcasts → Lyrics);
`↑↓`/`j`/`k` move; `enter` opens a library entry, plays the selected track, opens
a show's episodes / plays an episode (Podcasts), or — on the Lyrics panel — opens
the full-lyrics pane; `esc` backs out of a show's episodes; `space` play/pause;
`n`/`b` next/prev; `←`/`→` seek; `+`/`-` volume; `s` shuffle; `r`/`R`
loop-all/loop-one (repeat); `/` opens search; `q` quit. The shuffle and repeat glyphs turn
**green** when active. (*Smart shuffle* has no Web API endpoint and can't be
toggled here; pressing `S` says so.) **Mouse:** wheel scrolls the panel under the
pointer; click a library entry to open it; click a track to play it; click the
lyrics panel to focus it; click or drag the progress bar to seek; click the
play/pause area to toggle.

**Spotlight search:** press `/` for a floating, macOS-Spotlight-style search box
that overlays the UI. Type and results update live; `↑`/`↓` select a result,
`enter` plays it (and loads the results into the center panel), and `esc` closes
the overlay.

> **Podcast playback is best-effort.** AudioPulse can list and queue episodes, but
> actual audio is decoded by librespot, whose podcast support is limited — some
> episodes (region-locked, or hosted off Spotify's CDN, marked `⊘`) won't play. If
> an episode doesn't start, that's the cause, not a bug in the queueing.

## Contents (Deezer guest mode)

- [Launching](#launching)
- [The interface](#the-interface)
- [Searching](#searching)
- [Browsing results](#browsing-results)
- [Playback](#playback)
- [Autoplay](#autoplay)
- [Volume](#volume)
- [Keybinding reference](#keybinding-reference)
- [Tips](#tips)

## Launching

```bash
make run            # with audio
make run-silent     # no audio device required
```

AudioPulse opens in the alternate screen buffer and restores your terminal on
exit. It needs a terminal at least **64×18** characters; smaller than that and it
shows a resize prompt.

## The interface

```
 ♫ AudioPulse                                                       powered by Deezer   ← title
╭──────────────────────╮╭──────────────────────────────────────────────────────────╮
│  LIBRARY             ││   search box…                                              │
│    Search            ││ ──────────────────────────────────────────────────────    │
│  ▌ Results           ││ status / result count                                      │  ← main
│                      ││ ♪ now-playing row   ▶ selected row   plain rows…            │
│  NOW PLAYING         ││                                                            │
│  <track>             ││                                                            │
╰──────────────────────╯╰──────────────────────────────────────────────────────────╯
╭──────────────────────────────────────────────────────────────────────────────────╮
│  ▶  Title — Artist                                                                 │  ← now
│  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  0:12 / 0:30   │    playing
│  vol ━━━━━━━━────   autoplay on                                                     │
╰──────────────────────────────────────────────────────────────────────────────────╯
  contextual help line                                                                  ← help
```

- **Title bar** — app name and data source.
- **Sidebar** — navigation (which pane has focus), the current track, and live
  counters (autoplay state, result count).
- **Main pane** — the search box, a status line, and the results list.
- **Now-playing bar** — playback state, a progress bar with elapsed/total times,
  a volume meter, and the autoplay indicator.
- **Help line** — changes to show the keys relevant to the focused pane.

Two panes can hold **focus**: the **search box** and the **results list**. The
focused pane is highlighted (a green underline under the search box; a green
`▌` marker beside the active sidebar entry).

## Searching

1. Press `/` to focus the search box (it has focus on launch).
2. Type a query — a song, artist, or album.
3. Press `Enter`.

The status line shows `Searching…`, then the result count. On a successful
search, focus moves automatically to the results list. Up to 50 results are
returned, filtered to tracks that have a playable preview.

If a search fails (e.g. no network), the error appears in the status line and the
app keeps running — fix the issue and search again.

## Browsing results

With the results list focused:

- `↑`/`↓` or `k`/`j` — move the selection.
- `g` / `G` — jump to the top / bottom.
- The list scrolls automatically to keep the selection visible.

Each row shows a title, artist, and the full track duration. The selected row is
marked `▶` in green; the currently playing track is marked `♪`.

## Playback

- `Enter` — play the selected track.
- `space` or `p` — pause / resume.
- `s` — stop and clear the now-playing bar.
- `n` — play the next result.
- `b` — play the previous result.

When a track starts, the now-playing bar shows its title and artist, the progress
bar begins to advance, and the play indicator turns to `▶` (or `⏸` when paused).

> Previews are 30 seconds — a Deezer API limitation. The progress bar reflects
> the actual preview length.

## Autoplay

Autoplay is **on** by default. When a track finishes, AudioPulse plays the next
result in the list automatically, stopping at the end of the list.

- `a` — toggle autoplay on/off. The current state is shown both in the sidebar
  and the now-playing bar.

## Volume

- `+` / `=` — increase volume.
- `-` / `_` — decrease volume.

The volume meter in the now-playing bar reflects the current level. Volume is a
playback gain; at the lowest setting output is effectively muted.

## Keybinding reference

| Context  | Key             | Action                          |
| -------- | --------------- | ------------------------------- |
| Global   | `ctrl+c`        | Quit                            |
| Search   | `enter`         | Run the search                  |
| Search   | `esc` / `tab`   | Switch to results               |
| Results  | `q`             | Quit                            |
| Results  | `/` / `tab`     | Switch to search                |
| Results  | `↑`/`↓` `k`/`j` | Move selection                  |
| Results  | `g` / `G`       | Jump to top / bottom            |
| Results  | `enter`         | Play selected                   |
| Results  | `space` / `p`   | Pause / resume                  |
| Results  | `s`             | Stop                            |
| Results  | `n` / `b`       | Next / previous track           |
| Results  | `+` / `-`       | Volume up / down                |
| Results  | `a`             | Toggle autoplay                 |

> `q` quits only while the **results** pane is focused, so it doesn't interfere
> with typing a query that contains the letter "q". From the search box, use
> `ctrl+c` to quit.

## Tips

- Searching for an **artist** is the quickest way to fill a queue, then leave
  autoplay on to listen straight through.
- Use `n`/`b` to skip without returning to the list.
- If sound doesn't work, you can still drive the whole UI with the
  [silent build](getting-started.md#the-silent-build); see
  [Troubleshooting](troubleshooting.md) to fix audio.
