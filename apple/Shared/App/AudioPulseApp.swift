import SwiftUI

@main
struct AudioPulseApp: App {
    @State private var player: PlayerModel
    @State private var library: LibraryStore
    @State private var ask = AskModel()
    @State private var router = AppRouter()
    #if os(macOS)
    @State private var spotify: SpotifySession
    #else
    @State private var spotify: PhoneSpotifySession
    #endif

    init() {
        PulseFont.register()
        let player = PlayerModel()
        let library = LibraryStore()
        _player = State(initialValue: player)
        _library = State(initialValue: library)
        #if os(macOS)
        _spotify = State(initialValue: SpotifySession(player: player, library: library))
        #else
        _spotify = State(initialValue: PhoneSpotifySession(player: player, library: library))
        #endif
    }

    var body: some Scene {
        #if os(macOS)
        WindowGroup {
            MacRootView()
                .environment(player)
                .environment(library)
                .environment(spotify)
                .environment(ask)
                .environment(router)
                .preferredColorScheme(.dark)
                .frame(minWidth: 1100, minHeight: 720)
        }
        .windowStyle(.hiddenTitleBar)
        .defaultSize(width: 1440, height: 900)
        .commands {
            CommandMenu("Playback") {
                Button(player.isPlaying ? "Pause" : "Play") { player.togglePlay() }
                Button("Next") { player.next() }.keyboardShortcut(.rightArrow, modifiers: .command)
                Button("Previous") { player.previous() }.keyboardShortcut(.leftArrow, modifiers: .command)
                Divider()
                Button("Full-screen Lyrics") { router.toggleLyrics() }.keyboardShortcut("l", modifiers: .command)
            }
            CommandMenu("Pulse") {
                Button("Ask Pulse…") { router.openAsk() }.keyboardShortcut("k", modifiers: .command)
                Divider()
                if spotify.isConnected {
                    Button("Sign Out of Spotify") { spotify.signOut() }
                } else {
                    Button("Connect Spotify…") { spotify.dismissed = false; spotify.retry() }
                }
            }
        }
        #else
        WindowGroup {
            PhoneRootView()
                .environment(player)
                .environment(library)
                .environment(spotify)
                .environment(ask)
                .environment(router)
                .preferredColorScheme(.dark)
        }
        #endif
    }
}

enum Route: Hashable {
    case home
    case discover
    case playlist(String)
    case liked
}

@MainActor @Observable
final class AppRouter {
    var route: Route = .home
    var showAsk = false
    var showLyrics = false
    var sidebarVisible = true

    // iPhone
    enum Tab: Hashable { case home, discover, ask, library }
    var tab: Tab = .home
    var showNowPlaying = false
    var phonePath: [Route] = []

    func openAsk() {
        withAnimation(.spring(response: 0.3, dampingFraction: 0.85)) { showAsk = true }
    }

    func closeAsk() {
        withAnimation(.easeOut(duration: 0.18)) { showAsk = false }
    }

    func toggleLyrics() {
        withAnimation(.spring(response: 0.45, dampingFraction: 0.9)) { showLyrics.toggle() }
    }

    func go(_ r: Route) {
        withAnimation(.easeOut(duration: 0.2)) { route = r }
    }
}
