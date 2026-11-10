import SwiftUI

struct PhoneHomeView: View {
    @Environment(AppRouter.self) private var router
    @Environment(AskModel.self) private var ask
    @Environment(PlayerModel.self) private var player
    @Environment(LibraryStore.self) private var library

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                HStack {
                    Text(greeting)
                        .font(PulseFont.display(38)).tracking(-0.77)
                        .foregroundStyle(Pulse.text)
                    Spacer()
                    Button { router.tab = .library } label: {
                        Text(library.userInitials).font(.system(size: 14, weight: .bold)).foregroundStyle(Pulse.text)
                            .frame(width: 40, height: 40)
                            .background(Circle().fill(Pulse.forest))
                    }
                    .buttonStyle(PlainPressStyle())
                    .accessibilityLabel("Account and settings")
                }

                Button {
                    ask.startListening()
                    router.showAsk = true
                } label: {
                    HStack(spacing: 10) {
                        SparkleIcon(size: 17)
                        Text("Search, or ask Pulse anything").font(.pulseBody(15)).foregroundStyle(Pulse.textSecondary)
                        Spacer()
                        Image(systemName: "mic.fill").font(.system(size: 17, weight: .semibold))
                            .foregroundStyle(Pulse.onAccent)
                            .frame(width: 44, height: 44)
                            .background(RoundedRectangle(cornerRadius: 15, style: .continuous).fill(Pulse.accent))
                    }
                    .padding(.leading, 16).padding(.trailing, 6)
                    .frame(height: 56)
                    .background(RoundedRectangle(cornerRadius: 20, style: .continuous).fill(Color.white.opacity(0.07)))
                    .overlay(RoundedRectangle(cornerRadius: 20, style: .continuous).strokeBorder(Pulse.hairlineStrong))
                }
                .buttonStyle(PlainPressStyle())
                .accessibilityLabel("Search or ask Pulse")

                LazyVGrid(columns: [GridItem(.flexible(), spacing: 8), GridItem(.flexible(), spacing: 8)], spacing: 8) {
                    QuickTile(name: "Liked Songs", art: Artwork(colors: [0x1E8F62, 0xB8F5CF], angle: 135), playing: false) {
                        router.phonePath.append(.liked)
                    }
                    ForEach(library.playlists.prefix(5)) { p in
                        QuickTile(name: p.name, art: p.artwork, playing: player.context?.id == p.id && player.isPlaying) {
                            router.phonePath.append(.playlist(p.id))
                        }
                    }
                }

                if !library.homeCards.isEmpty {
                VStack(alignment: .leading, spacing: 10) {
                    Text(library.source == .sample ? "Built from your library" : "Your playlists")
                        .font(PulseFont.display(22, weight: .bold)).tracking(-0.32)
                        .foregroundStyle(Pulse.text)
                    ScrollView(.horizontal) {
                        HStack(spacing: 12) {
                            ForEach(library.homeCards) { item in
                                Button { router.phonePath.append(.playlist(item.playlistID)) } label: {
                                    VStack(alignment: .leading, spacing: 8) {
                                        ArtworkView(artwork: item.artwork, cornerRadius: 14).frame(width: 140, height: 140)
                                        Text(item.name).font(.pulseBody(14, weight: .semibold)).foregroundStyle(Pulse.text).lineLimit(1)
                                        Text(item.detail).font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary).lineLimit(1)
                                    }
                                    .frame(width: 140, alignment: .leading)
                                }
                                .buttonStyle(PlainPressStyle())
                            }
                        }
                    }
                    .scrollIndicators(.never)
                }
                }

                if !library.recent.isEmpty {
                    VStack(alignment: .leading, spacing: 10) {
                        Text(library.source == .sample ? "Because you played Low Tide" : "Jump back in")
                            .font(PulseFont.display(22, weight: .bold)).tracking(-0.32)
                            .foregroundStyle(Pulse.text)
                        ScrollView(.horizontal) {
                            LazyHStack(spacing: 12) {
                                ForEach(library.recent) { t in
                                    RecentCard(track: t, isCurrent: player.current.id == t.id && player.isPlaying) {
                                        playRecent(t)
                                    }
                                }
                            }
                            .scrollTargetLayout()
                        }
                        .scrollTargetBehavior(.viewAligned)
                        .scrollIndicators(.never)
                        .contentMargins(.horizontal, 20, for: .scrollContent)
                        .padding(.horizontal, -20)
                    }
                }
            }
            .padding(.horizontal, 20)
            .padding(.top, 8)
        }
        .scrollIndicators(.never)
        .contentMargins(.bottom, PhoneLayout.bottomClearance, for: .scrollContent)
        .background(GlowBackdrop(primary: UnitPoint(x: 0, y: 0), secondary: UnitPoint(x: 1, y: 0.6), strength: 0.6))
    }

    private func playRecent(_ t: Track) {
        if library.source == .sample {
            player.play(t, in: MockLibrary.nightDrive)
        } else {
            player.play(t, in: Playlist(id: "recent", name: "Recently played", initials: "RP", badgeColor: 0x1E8F62,
                                        badgeOnDark: true, artwork: t.artwork, tracks: library.recent))
        }
    }

    private var greeting: String {
        let h = Calendar.current.component(.hour, from: .now)
        return h < 12 ? "Good morning" : (h < 18 ? "Good afternoon" : "Good evening")
    }
}

struct QuickTile: View {
    let name: String
    let art: Artwork
    let playing: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 10) {
                ArtworkView(artwork: art, cornerRadius: 0).frame(width: 52, height: 52)
                Text(name).font(.pulseBody(13, weight: .semibold)).foregroundStyle(Pulse.text)
                    .lineLimit(2).multilineTextAlignment(.leading)
                Spacer(minLength: 0)
                if playing { NowPlayingBars().padding(.trailing, 8) }
            }
            .background(Color.white.opacity(0.06))
            .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
    }
}

/// A song card in the horizontal "Jump back in" row.
struct RecentCard: View {
    let track: Track
    let isCurrent: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            VStack(alignment: .leading, spacing: 8) {
                ArtworkView(artwork: track.artwork, cornerRadius: 14)
                    .frame(width: 140, height: 140)
                    .overlay(alignment: .bottomTrailing) {
                        if isCurrent { NowPlayingBars().padding(10) }
                    }
                Text(track.title).font(.pulseBody(14, weight: .semibold))
                    .foregroundStyle(isCurrent ? Pulse.accent : Pulse.text).lineLimit(1)
                Text(track.artist).font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary).lineLimit(1)
            }
            .frame(width: 140, alignment: .leading)
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
        .accessibilityLabel("Play \(track.title) by \(track.artist)")
    }
}

/// Space kept below scrolling content so it clears the mini-player and tab bar.
enum PhoneLayout {
    static let bottomClearance: CGFloat = 170
}
