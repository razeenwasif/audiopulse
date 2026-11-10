import SwiftUI

struct MacRootView: View {
    @Environment(AppRouter.self) private var router
    @Environment(PlayerModel.self) private var player

    var body: some View {
        ZStack {
            GlowBackdrop(primary: UnitPoint(x: 0.82, y: 0.12), secondary: UnitPoint(x: 0.35, y: 1.15))

            VStack(spacing: 0) {
                HStack(spacing: 0) {
                    if router.sidebarVisible {
                        MacSidebar()
                            .padding([.leading, .top, .bottom], 10)
                            .transition(.move(edge: .leading).combined(with: .opacity))
                    }
                    content
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                }
                MacPlayerBar()
            }

            if !router.sidebarVisible {
                VStack {
                    HStack {
                        IconButton(systemName: "sidebar.left", label: "Show sidebar", size: 32, glyph: 14, tint: Pulse.textSecondary) {
                            withAnimation(.spring(response: 0.35, dampingFraction: 0.85)) { router.sidebarVisible = true }
                        }
                        .padding(.leading, 84)
                        .padding(.top, 8)
                        Spacer()
                    }
                    Spacer()
                }
            }

            if router.showAsk {
                Color.black.opacity(0.55)
                    .ignoresSafeArea()
                    .onTapGesture { router.closeAsk() }
                    .transition(.opacity)
                VStack {
                    AskPalette()
                        .frame(width: 700)
                        .padding(.top, 110)
                    Spacer()
                }
                .transition(.scale(scale: 0.97, anchor: .top).combined(with: .opacity))
            }

            if router.showLyrics {
                MacFullScreenLyrics()
                    .transition(.opacity.combined(with: .scale(scale: 1.02)))
            }
        }
        .ignoresSafeArea()
        .pulseMenuHost()
        .onAppear { keys.install(player: player, router: router) }
        .onDisappear { keys.remove() }
    }

    @State private var keys = WindowKeys()

    @ViewBuilder private var content: some View {
        switch router.route {
        case .playlist(let id):
            if let p = MockLibrary.playlists.first(where: { $0.id == id }) {
                MacPlaylistView(playlist: p)
            }
        default:
            HStack(spacing: 0) {
                MacHomeView()
                MacNowPlayingPanel()
                    .frame(width: 350)
            }
        }
    }
}
