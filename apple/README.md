# AudioPulse for macOS and iOS

SwiftUI apps implementing the **Pulse** design: near-black surfaces with green
accents and soft glows, Bricolage Grotesque display type, and a waveform
scrubber. Every control (buttons, menus, segmented controls, sliders, toggles,
tab bar) is custom-drawn; no stock SwiftUI control chrome is used.

## Build

```bash
brew install xcodegen      # once
cd apple
xcodegen generate          # creates AudioPulse.xcodeproj from project.yml
open AudioPulse.xcodeproj  # schemes: AudioPulse-macOS, AudioPulse-iOS
```

## Layout

| Path | What |
| --- | --- |
| `Shared/Theme` | Palette and type (`Theme.swift`), custom controls (`Controls.swift`), custom dropdown menus (`PulseMenu.swift`) |
| `Shared/Components` | Artwork, glow backdrop, waveform scrubber, voice level bars, synced lyrics |
| `Shared/Model` | Models, sample library, `PlayerModel`, `AskModel` |
| `macOS/` | Sidebar, Home, Now Playing panel, playlist, Ask palette (⌘K), full-screen lyrics (⌘L) |
| `iOS/` | Home, Library, playlist, Now Playing, voice Ask, custom tab bar and mini-player |

## Spotify (macOS)

The Mac app plays your own Spotify library (Premium) through librespot:

1. Install librespot once: `make librespot` in the repo root.
2. Launch the app and follow **Connect Spotify**: paste your Spotify app's
   Client ID (Redirect URI `http://127.0.0.1:8888/callback`), sign in, then
   approve the librespot device once in the browser.

Settings are shared with the terminal app in
`~/Library/Application Support/audiopulse/` (`config.json`, the `librespot/`
device cache). The Mac app keeps its own token in `mac-token.json` (0600),
because Spotify rotates refresh tokens. librespot output goes to
`$TMPDIR/audiopulse-mac-librespot.log` (Settings menu → Open librespot log).

Code lives in `macOS/Spotify/`: PKCE sign-in with a loopback server
(`SpotifyAuth`), the Web API client (`SpotifyAPI`), the librespot supervisor
(`LibrespotDevice`), the playback backend (`SpotifyPlayback`) and the
orchestrator (`SpotifySession`). Lyrics come from LRCLIB.

## Status

- macOS: real library, playback and synced lyrics when connected; the sample
  library otherwise.
- iOS: sample data only (librespot cannot run as a separate process there).
- Ask Pulse still returns canned results; the local Ollama assistant is not
  wired in yet.

Bricolage Grotesque is bundled under the SIL Open Font License
(`Shared/Resources/Fonts/OFL.txt`).
