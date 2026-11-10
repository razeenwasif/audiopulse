import AppKit
import SwiftUI

struct MacRootView: View {
    @Environment(AppRouter.self) private var router
    @Environment(PlayerModel.self) private var player
    @Environment(LibraryStore.self) private var library
    @Environment(SpotifySession.self) private var spotify

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

            if let notice = player.notice {
                VStack {
                    Spacer()
                    HStack(spacing: 10) {
                        Image(systemName: "exclamationmark.circle.fill").foregroundStyle(Pulse.accent)
                        Text(notice).font(.pulseBody(13, weight: .medium)).foregroundStyle(Pulse.text)
                    }
                    .padding(.horizontal, 16).frame(height: 40)
                    .background(Capsule().fill(Color(hex: 0x0E0F0E)))
                    .overlay(Capsule().strokeBorder(Pulse.hairlineStrong))
                    .shadow(color: .black.opacity(0.5), radius: 20, y: 10)
                    .padding(.bottom, 108)
                }
                .transition(.move(edge: .bottom).combined(with: .opacity))
                .task(id: notice) {
                    try? await Task.sleep(for: .seconds(5))
                    withAnimation { player.notice = nil }
                }
            }

            if spotify.showConnectScreen {
                Color.black.opacity(0.6).ignoresSafeArea().transition(.opacity)
                ConnectSpotifyView()
                    .transition(.scale(scale: 0.97).combined(with: .opacity))
            }
        }
        .animation(.spring(response: 0.35, dampingFraction: 0.88), value: spotify.showConnectScreen)
        .ignoresSafeArea()
        .pulseMenuHost()
        .onAppear {
            keys.install(player: player, router: router)
            spotify.start()
        }
        .onDisappear { keys.remove() }
        .onReceive(NotificationCenter.default.publisher(for: NSApplication.willTerminateNotification)) { _ in
            spotify.shutdown()
        }
    }

    @State private var keys = WindowKeys()

    @ViewBuilder private var content: some View {
        switch router.route {
        case .playlist(let id):
            if let p = library.playlist(id) {
                MacPlaylistView(playlist: p)
                    .id(p.id)
                    .task(id: p.id) { await library.ensureTracks(for: p.id) }
            } else {
                home
            }
        case .liked:
            MacPlaylistView(playlist: library.liked)
                .id(LibraryStore.likedID)
                .task { await library.ensureTracks(for: LibraryStore.likedID) }
        default:
            home
        }
    }

    private var home: some View {
        HStack(spacing: 0) {
            MacHomeView()
            MacNowPlayingPanel()
                .frame(width: 350)
        }
    }
}
