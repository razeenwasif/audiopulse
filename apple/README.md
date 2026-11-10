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

## Spotify (iPhone)

The iPhone app uses Spotify's official iOS SDK (App Remote, SwiftPM
`spotify/ios-sdk` 5.0.1): AudioPulse shows your library and controls
playback, and the Spotify app on the phone plays the audio.

1. In your Spotify developer app add the Redirect URI
   `audiopulse://spotify-callback` and the iOS bundle ID `dev.audiopulse.ios`.
2. Build with your Client ID baked in (or paste it in the app):

   ```bash
   xcodebuild -project AudioPulse.xcodeproj -scheme AudioPulse-iOS \
     -destination 'id=<device-udid>' -allowProvisioningUpdates \
     DEVELOPMENT_TEAM=<team> SPOTIFY_CLIENT_ID=<client-id> build
   ```

3. Tap **Connect Spotify** (web sign-in sheet), then play something: the first
   play opens Spotify once to connect, then returns to AudioPulse.

Sign-in, the Web API client and library loading are shared with macOS
(`Shared/Spotify/`); the iPhone-specific parts are in `iOS/Spotify/`.

## Ask Pulse (assistant)

Ask Pulse turns a typed or spoken request into a plan in one model call (the
same actions and rules as the terminal app) and carries it out with Spotify:
play a song, play or queue recommendations, smart shuffle the current
playlist, create and save a playlist, or answer a question about your library.
Speech is recognised with Apple's recogniser, on the device when supported.

It runs on, in order (Automatic) or as chosen:

- **Fairy** — the desktop assistant in `desktop-buddies/Lyra`. On the Mac,
  through her `audiopulse.sock`; on the iPhone, through her phone link's
  `/v1/assist` over Tailscale after pairing (Assistant settings → Scan the
  code), with her certificate pinned. Fairy runs each request once on Claude,
  Gemini or ChatGPT, with no persona, and nothing is saved as a chat.
- **Ollama** on a PC with a GPU, over Tailscale
  (`http://pc.tailnet.ts.net:11434`; the Mac reads `ollama_url` from the shared
  config).

Requests and a short summary of your library (what's playing, some Liked Songs,
recent plays, playlist names) go to whichever of these answers.

## Status

- macOS: real library, playback and synced lyrics when connected; the sample
  library otherwise.
- iOS: real library and playback through the Spotify app (App Remote) when
  connected; the sample library otherwise.
- Ask Pulse runs on Fairy or a PC's Ollama; organising Liked Songs by genre is
  still terminal-only.

Bricolage Grotesque is bundled under the SIL Open Font License
(`Shared/Resources/Fonts/OFL.txt`).
