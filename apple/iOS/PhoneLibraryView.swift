import SwiftUI

struct PhoneLibraryView: View {
    @Environment(AppRouter.self) private var router
    @Environment(PlayerModel.self) private var player
    @Environment(LibraryStore.self) private var library
    @Environment(PhoneSpotifySession.self) private var spotify

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                HStack {
                    Text("Library").font(PulseFont.display(40)).tracking(-0.81).foregroundStyle(Pulse.text)
                    Spacer()
                    PulseMenuButton(items: accountMenu, accessibilityLabel: "Account and settings") {
                        Text(library.userInitials).font(.system(size: 14, weight: .bold)).foregroundStyle(Pulse.text)
                            .frame(width: 40, height: 40)
                            .background(Circle().fill(Pulse.forest))
                    }
                    .buttonStyle(PlainPressStyle())
                }

                VStack(spacing: 0) {
                    LibraryRow(icon: "heart", title: "Liked Songs", trailing: library.likedCount.formatted(), selected: true) {
                        router.phonePath.append(.liked)
                    }
                    LibraryRow(icon: "music.note.list", title: "Playlists")
                    LibraryRow(icon: "clock", title: "Recently Played")
                    LibraryRow(icon: "mic", title: "Podcasts", dot: true)
                    LibraryRow(icon: "square.grid.2x2", title: "Genres", trailing: library.genreCount > 0 ? "\(library.genreCount)" : nil)
                    LibraryRow(icon: "arrow.down.to.line", title: "Downloads")
                }
                .padding(6)
                .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Pulse.panel))
                .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).strokeBorder(Pulse.hairline))

                HStack {
                    Text("PLAYLISTS").sectionLabel()
                    Spacer()
                    Button("See all") {}
                        .font(.pulseBody(14, weight: .medium))
                        .foregroundStyle(Pulse.accent)
                        .buttonStyle(PlainPressStyle())
                }
                .padding(.horizontal, 6)
                .padding(.top, 4)

                VStack(spacing: 0) {
                    ForEach(library.playlists) { p in
                        Button { router.phonePath.append(.playlist(p.id)) } label: {
                            HStack(spacing: 12) {
                                PlaylistAvatar(playlist: p, size: 34)
                                Text(p.name).font(.pulseBody(16, weight: .medium)).foregroundStyle(Pulse.text)
                                Spacer()
                                if player.context?.id == p.id && player.isPlaying {
                                    PulseBadge(text: "PLAYING")
                                } else if p.prompt != nil {
                                    PulseBadge(text: "NEW", filled: false)
                                } else if p.updated {
                                    UnreadDot()
                                }
                            }
                            .padding(.horizontal, 6)
                            .frame(height: 50)
                            .contentShape(Rectangle())
                        }
                        .buttonStyle(PlainPressStyle())
                    }
                }
            }
            .padding(.horizontal, 20)
            .padding(.top, 8)
        }
        .scrollIndicators(.never)
        .contentMargins(.bottom, PhoneLayout.bottomClearance, for: .scrollContent)
        .background(GlowBackdrop(primary: UnitPoint(x: 0.95, y: 0), secondary: UnitPoint(x: 0, y: 1), strength: 0.55))
    }
}

extension PhoneLibraryView {
    var accountMenu: [PulseMenuItem] {
        var items: [PulseMenuItem] = [
            PulseMenuItem(title: library.userName, systemImage: "person.crop.circle") {},
        ]
        if spotify.isConnected {
            items.append(PulseMenuItem(title: "Sign out of Spotify", systemImage: "rectangle.portrait.and.arrow.right",
                                       destructive: true) { spotify.signOut() })
        } else {
            items.append(PulseMenuItem(title: "Connect Spotify…", systemImage: "person.crop.circle.badge.checkmark") {
                spotify.dismissed = false
                spotify.retry()
            })
        }
        return items
    }
}

private struct LibraryRow: View {
    let icon: String
    let title: String
    var trailing: String? = nil
    var dot = false
    var selected = false
    var action: () -> Void = {}

    var body: some View {
        Button(action: action) {
            HStack(spacing: 14) {
                Image(systemName: icon).font(.system(size: 17, weight: .medium)).frame(width: 24)
                Text(title).font(.pulseBody(16, weight: .medium))
                Spacer()
                if let trailing { Text(trailing).font(.pulseBody(14)).monospacedDigit().foregroundStyle(Pulse.textTertiary) }
                if dot { UnreadDot() }
                Image(systemName: "chevron.right").font(.system(size: 12, weight: .semibold)).foregroundStyle(Pulse.textTertiary)
            }
            .foregroundStyle(selected ? Pulse.accent : Pulse.text)
            .padding(.horizontal, 12)
            .frame(height: 48)
            .background(RoundedRectangle(cornerRadius: 12, style: .continuous).fill(selected ? Pulse.accentPill : .clear))
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
    }
}

struct PhonePlaylistView: View {
    let playlist: Playlist
    @Environment(AppRouter.self) private var router
    @Environment(PlayerModel.self) private var player

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                HStack {
                    IconButton(systemName: "chevron.left", label: "Back", glyph: 17, kind: .chip) {
                        _ = router.phonePath.popLast()
                    }
                    Spacer()
                    MoreMenuButton(items: [
                        PulseMenuItem(title: "Add to queue", systemImage: "text.line.last.and.arrowtriangle.forward") {},
                        PulseMenuItem(title: "Download", systemImage: "arrow.down.circle") {},
                        PulseMenuItem(title: "Share link", systemImage: "link") {},
                    ], size: 44)
                }

                ArtworkView(artwork: playlist.artwork, cornerRadius: 22)
                    .frame(width: 220, height: 220)
                    .frame(maxWidth: .infinity)
                    .shadow(color: Pulse.accent.opacity(0.18), radius: 40, y: 20)

                if playlist.prompt != nil {
                    Label("MADE BY PULSE", systemImage: "sparkles")
                        .font(.pulseCaption).tracking(1.1).foregroundStyle(Pulse.accent)
                }
                Text(playlist.name).font(PulseFont.display(52)).tracking(-1.17).foregroundStyle(Pulse.text)
                    .lineLimit(1).minimumScaleFactor(0.6)
                if let prompt = playlist.prompt {
                    Text("You asked: “\(prompt)”").font(.pulseBody(14)).foregroundStyle(Color(hex: 0xCDD3D0))
                }
                Text("\(playlist.trackCount) songs" + (playlist.tracksLoaded ? " · \(playlist.totalDuration.longDuration)" : ""))
                    .font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)

                HStack(spacing: 10) {
                    IconButton(systemName: "play.fill", label: "Play \(playlist.name)", size: 56, glyph: 22, kind: .primary) {
                        if let t = playlist.tracks.first { player.play(t, in: playlist) }
                    }
                    Button {
                        player.shuffle = true
                        if let t = playlist.tracks.randomElement() { player.play(t, in: playlist) }
                    } label: {
                        Label("Smart shuffle", systemImage: "shuffle").font(.pulseBody(14, weight: .medium))
                            .padding(.horizontal, 16).frame(height: 44)
                    }
                    .pulse(.outline)
                }

                if !playlist.tracksLoaded {
                    HStack(spacing: 12) {
                        NowPlayingBars()
                        Text("Loading songs…").font(.pulseBody(14)).foregroundStyle(Pulse.textSecondary)
                    }
                    .frame(maxWidth: .infinity, minHeight: 100)
                }
                VStack(spacing: 2) {
                    ForEach(playlist.tracks) { t in
                        Button { player.play(t, in: playlist) } label: {
                            HStack(spacing: 12) {
                                ArtworkView(artwork: t.artwork, cornerRadius: 8).frame(width: 46, height: 46)
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(t.title).font(.pulseBody(15, weight: .semibold))
                                        .foregroundStyle(player.current.id == t.id ? Pulse.accent : Pulse.text)
                                    Text(t.why ?? t.artist).font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary).lineLimit(1)
                                }
                                Spacer()
                                if player.current.id == t.id && player.isPlaying { NowPlayingBars() }
                            }
                            .padding(.vertical, 6)
                            .contentShape(Rectangle())
                        }
                        .buttonStyle(PlainPressStyle())
                    }
                }
            }
            .padding(.horizontal, 20)
            .padding(.top, 8)
        }
        .scrollIndicators(.never)
        .contentMargins(.bottom, PhoneLayout.bottomClearance, for: .scrollContent)
        .background(GlowBackdrop(primary: UnitPoint(x: 0.5, y: 0.1), secondary: UnitPoint(x: 0, y: 1), strength: 0.9))
    }
}
