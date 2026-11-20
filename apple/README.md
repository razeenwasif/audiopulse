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

## Status

The UI runs on sample data (fictional artists). Playback goes through the
`PlaybackBackend` protocol in `PlayerModel.swift`; the current backend only
simulates a clock. Spotify (Web API + librespot) and the local AI assistant
are not wired in yet.

Bricolage Grotesque is bundled under the SIL Open Font License
(`Shared/Resources/Fonts/OFL.txt`).
