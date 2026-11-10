import SwiftUI

struct PhoneRootView: View {
    @Environment(AppRouter.self) private var router
    @Environment(AskModel.self) private var ask
    @Environment(PlayerModel.self) private var player
    @Environment(LibraryStore.self) private var library
    @Environment(PhoneSpotifySession.self) private var spotify
    @Environment(\.scenePhase) private var scenePhase

    var body: some View {
        @Bindable var router = router
        ZStack(alignment: .bottom) {
            Pulse.background.ignoresSafeArea()

            NavigationStack(path: $router.phonePath) {
                Group {
                    switch router.tab {
                    case .library: PhoneLibraryView()
                    case .discover: PhoneDiscoverPlaceholder()
                    default: PhoneHomeView()
                    }
                }
                .toolbar(.hidden, for: .navigationBar)
                .navigationDestination(for: Route.self) { route in
                    switch route {
                    case .playlist(let id):
                        if let p = library.playlist(id) {
                            PhonePlaylistView(playlist: p)
                                .toolbar(.hidden, for: .navigationBar)
                                .task(id: id) { await library.ensureTracks(for: id) }
                        }
                    case .liked:
                        PhonePlaylistView(playlist: library.liked)
                            .toolbar(.hidden, for: .navigationBar)
                            .task { await library.ensureTracks(for: LibraryStore.likedID) }
                    default:
                        EmptyView()
                    }
                }
            }

            VStack(spacing: 8) {
                PhoneMiniPlayer()
                PhoneTabBar()
            }
            .padding(.top, 28)
            .background {
                VStack(spacing: 0) {
                    LinearGradient(colors: [Pulse.background.opacity(0), Pulse.background],
                                   startPoint: .top, endPoint: .bottom)
                        .frame(height: 40)
                    Pulse.background
                }
                .ignoresSafeArea(edges: .bottom)
                .allowsHitTesting(false)
            }
        }
        .fullScreenCover(isPresented: $router.showNowPlaying) {
            PhoneNowPlayingView()
        }
        .fullScreenCover(isPresented: $router.showAsk) {
            PhoneAskView()
        }
        .overlay {
            if let notice = player.notice {
                VStack {
                    HStack(spacing: 10) {
                        Image(systemName: "exclamationmark.circle.fill").foregroundStyle(Pulse.accent)
                        Text(notice).font(.pulseBody(13, weight: .medium)).foregroundStyle(Pulse.text).lineLimit(2)
                    }
                    .padding(.horizontal, 16).padding(.vertical, 10)
                    .background(Capsule().fill(Color(hex: 0x0E0F0E)))
                    .overlay(Capsule().strokeBorder(Pulse.hairlineStrong))
                    .padding(.top, 8)
                    Spacer()
                }
                .transition(.move(edge: .top).combined(with: .opacity))
                .task(id: notice) {
                    try? await Task.sleep(for: .seconds(4))
                    withAnimation { player.notice = nil }
                }
            }
        }
        .overlay {
            if spotify.showConnectScreen {
                PhoneConnectView().transition(.opacity)
            }
        }
        .animation(.easeOut(duration: 0.25), value: spotify.showConnectScreen)
        .pulseMenuHost()
        .onAppear {
            ask.configure(player: player, library: library)
            ask.openPlaylist = { id in
                router.tab = .library
                router.phonePath = [.playlist(id)]
            }
            spotify.start()
        }
        .onOpenURL { spotify.handle($0) }
        .onChange(of: scenePhase) { _, phase in
            switch phase {
            case .active: spotify.becameActive()
            case .background: spotify.resignedActive()
            default: break
            }
        }
    }
}

struct PhoneTabBar: View {
    @Environment(AppRouter.self) private var router
    @Environment(AskModel.self) private var ask

    var body: some View {
        HStack(spacing: 0) {
            tab(.home, "Home", "house")
            tab(.discover, "Discover", "safari")
            Button {
                ask.startListening()
                router.showAsk = true
            } label: { tabLabel("Ask", "sparkles", selected: false) }
                .buttonStyle(PlainPressStyle())
            tab(.library, "Library", "books.vertical")
        }
        .padding(.horizontal, 8)
    }

    private func tab(_ t: AppRouter.Tab, _ title: String, _ icon: String) -> some View {
        Button {
            router.phonePath = []
            withAnimation(.easeOut(duration: 0.15)) { router.tab = t }
        } label: { tabLabel(title, icon, selected: router.tab == t) }
            .buttonStyle(PlainPressStyle())
            .accessibilityAddTraits(router.tab == t ? .isSelected : [])
    }

    private func tabLabel(_ title: String, _ icon: String, selected: Bool) -> some View {
        VStack(spacing: 4) {
            Image(systemName: selected ? icon + ".fill" : icon)
                .font(.system(size: 20, weight: .medium))
                .frame(height: 24)
            Text(title).font(.system(size: 11, weight: selected ? .semibold : .medium))
        }
        .foregroundStyle(selected ? Pulse.accent : Pulse.textSecondary)
        .frame(maxWidth: .infinity, minHeight: 50)
        .contentShape(Rectangle())
    }
}

struct PhoneMiniPlayer: View {
    @Environment(PlayerModel.self) private var player
    @Environment(AppRouter.self) private var router

    var body: some View {
        VStack(spacing: 6) {
            HStack(spacing: 12) {
                Button { router.showNowPlaying = true } label: {
                    HStack(spacing: 12) {
                        ArtworkView(artwork: player.current.artwork, cornerRadius: 10).frame(width: 44, height: 44)
                        VStack(alignment: .leading, spacing: 1) {
                            Text(player.current.title)
                                .font(PulseFont.display(17, weight: .bold)).tracking(-0.4)
                                .foregroundStyle(Pulse.text)
                            Text(player.current.artist).font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)
                        }
                        Spacer(minLength: 0)
                    }
                    .contentShape(Rectangle())
                }
                .buttonStyle(PlainPressStyle())
                .accessibilityLabel("Now playing: \(player.current.title) by \(player.current.artist). Open player.")

                IconButton(systemName: player.isPlaying ? "pause.fill" : "play.fill",
                           label: player.isPlaying ? "Pause" : "Play", glyph: 17, kind: .primary) { player.togglePlay() }
            }
            WaveformScrubber(trackID: player.current.id, progress: player.progress, bars: 80, spacing: 1)
                .frame(height: 14)
                .padding(.horizontal, 4)
        }
        .padding(8)
        .background(RoundedRectangle(cornerRadius: 16, style: .continuous).fill(Color(hex: 0x0D0E0D)))
        .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous).strokeBorder(Pulse.hairlineStrong))
        .shadow(color: Pulse.accent.opacity(0.08), radius: 24)
        .padding(.horizontal, 10)
    }
}

struct PhoneDiscoverPlaceholder: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Discover").font(PulseFont.display(40)).tracking(-0.81).foregroundStyle(Pulse.text)
            Text("New releases and Pulse picks will live here.").font(.pulseBody(15)).foregroundStyle(Pulse.textSecondary)
            Spacer()
        }
        .padding(20)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(GlowBackdrop(primary: UnitPoint(x: 0.9, y: 0), secondary: UnitPoint(x: 0, y: 1), strength: 0.8))
    }
}
