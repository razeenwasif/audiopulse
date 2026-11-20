import SwiftUI

@main
struct AudioPulseApp: App {
    @State private var player = PlayerModel()
    @State private var ask = AskModel()
    @State private var router = AppRouter()

    init() {
        PulseFont.register()
    }

    var body: some Scene {
        #if os(macOS)
        WindowGroup {
            MacRootView()
                .environment(player)
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
            }
        }
        #else
        WindowGroup {
            PhoneRootView()
                .environment(player)
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
