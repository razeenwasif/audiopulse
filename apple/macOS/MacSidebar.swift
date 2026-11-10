import SwiftUI

struct MacSidebar: View {
    @Environment(AppRouter.self) private var router
    @Environment(PlayerModel.self) private var player

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            // Row reserved for the system traffic lights + our hide-sidebar button.
            HStack {
                Spacer()
                IconButton(systemName: "sidebar.left", label: "Hide sidebar", size: 32, glyph: 14, tint: Pulse.textSecondary) {
                    withAnimation(.spring(response: 0.35, dampingFraction: 0.85)) { router.sidebarVisible = false }
                }
            }
            .frame(height: 32)
            .padding(.horizontal, 4)

            brand

            ScrollView {
            VStack(alignment: .leading, spacing: 16) {
            VStack(spacing: 2) {
                SidebarRow(icon: "house", title: "Home", selected: router.route == .home) { router.go(.home) }
                SidebarRow(icon: "safari", title: "Discover", selected: router.route == .discover) { router.go(.discover) }
                SidebarRow(icon: "sparkles", title: "Ask Pulse", selected: false, trailing: .shortcut("⌘K")) { router.openAsk() }
            }

            VStack(alignment: .leading, spacing: 2) {
                Text("LIBRARY").sectionLabel().padding(.horizontal, 12).padding(.bottom, 6)
                SidebarRow(icon: "heart", title: "Liked Songs", selected: router.route == .liked,
                           trailing: .count(MockLibrary.likedCount.formatted())) { router.go(.liked) }
                SidebarRow(icon: "music.note.list", title: "Playlists", selected: false) {}
                SidebarRow(icon: "clock", title: "Recently Played", selected: false) {}
                SidebarRow(icon: "mic", title: "Podcasts", selected: false, trailing: .dot) {}
                SidebarRow(icon: "square.grid.2x2", title: "Genres", selected: false,
                           trailing: .count("\(MockLibrary.genres.count)")) {}
                SidebarRow(icon: "arrow.down.to.line", title: "Downloads", selected: false) {}
            }

            VStack(alignment: .leading, spacing: 2) {
                HStack {
                    Text("PLAYLISTS").sectionLabel()
                    Spacer()
                    Button("See all") {}
                        .font(.pulseBody(13, weight: .medium))
                        .foregroundStyle(Pulse.accent)
                        .buttonStyle(PlainPressStyle())
                }
                .padding(.horizontal, 12)
                .padding(.bottom, 6)

                ForEach(MockLibrary.playlists) { p in
                    PlaylistSidebarRow(playlist: p,
                                       selected: router.route == .playlist(p.id),
                                       playing: player.context?.id == p.id && player.isPlaying) {
                        router.go(.playlist(p.id))
                    }
                }
            }
            }
            }
            .scrollIndicators(.never)
            .frame(maxHeight: .infinity, alignment: .top)

            profile
        }
        .padding(.horizontal, 12)
        .padding(.top, 6)
        .padding(.bottom, 12)
        .frame(width: 264)
        .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Pulse.panel))
        .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).strokeBorder(Pulse.hairline))
    }

    private var brand: some View {
        HStack(spacing: 12) {
            Image(systemName: "waveform")
                .font(.system(size: 20, weight: .bold))
                .foregroundStyle(Pulse.onAccent)
                .frame(width: 40, height: 40)
                .background(RoundedRectangle(cornerRadius: 11, style: .continuous).fill(Pulse.accent))
                .shadow(color: Pulse.accent.opacity(0.35), radius: 14)
            Text("AudioPulse")
                .font(PulseFont.display(24, weight: .heavy))
                .tracking(-0.36)
                .foregroundStyle(Pulse.text)
        }
        .padding(.horizontal, 8)
        .accessibilityElement(children: .combine)
    }

    private var profile: some View {
        HStack(spacing: 12) {
            Text("MH")
                .font(.system(size: 14, weight: .bold))
                .foregroundStyle(Pulse.text)
                .frame(width: 40, height: 40)
                .background(Circle().fill(Pulse.forest))
            VStack(alignment: .leading, spacing: 1) {
                Text("Mika Hart").font(.pulseBody(15, weight: .semibold)).foregroundStyle(Pulse.text)
                Text("Spotify Premium").font(.pulseBody(13)).foregroundStyle(Pulse.textTertiary)
            }
            Spacer()
            PulseMenuButton(items: [
                PulseMenuItem(title: "Settings", systemImage: "gearshape", shortcut: "⌘,") {},
                PulseMenuItem(title: "Audio quality: 320 kbps", systemImage: "dial.medium") {},
                PulseMenuItem(title: "Local AI model", systemImage: "cpu") {},
                PulseMenuItem(title: "Sign out", systemImage: "rectangle.portrait.and.arrow.right", destructive: true) {},
            ], alignTrailing: true, accessibilityLabel: "Settings") {
                Image(systemName: "slider.horizontal.3")
                    .font(.system(size: 16, weight: .medium))
                    .foregroundStyle(Pulse.textSecondary)
                    .frame(width: 40, height: 40)
            }
            .pulse(.ghost, shape: .rounded(10))
        }
        .padding(.horizontal, 8)
        .padding(.top, 14)
        .overlay(alignment: .top) { Rectangle().fill(Pulse.hairline).frame(height: 1) }
    }
}

enum SidebarTrailing {
    case none, count(String), dot, shortcut(String)
}

struct SidebarRow: View {
    let icon: String
    let title: String
    let selected: Bool
    var trailing: SidebarTrailing = .none
    let action: () -> Void
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: 14) {
                Image(systemName: selected ? icon + ".fill" : icon)
                    .font(.system(size: 16, weight: .medium))
                    .frame(width: 22)
                Text(title).font(.pulseBody(15, weight: selected ? .semibold : .medium))
                Spacer(minLength: 4)
                switch trailing {
                case .none: EmptyView()
                case .count(let c): Text(c).font(.pulseBody(13)).monospacedDigit().foregroundStyle(Pulse.textTertiary)
                case .dot: UnreadDot().accessibilityLabel("New")
                case .shortcut(let s):
                    Text(s).font(.pulseBody(11)).foregroundStyle(Pulse.textTertiary)
                        .padding(.horizontal, 6).padding(.vertical, 2)
                        .overlay(RoundedRectangle(cornerRadius: 5).strokeBorder(Pulse.hairlineStrong))
                }
            }
            .foregroundStyle(selected ? Pulse.accent : Pulse.text)
            .padding(.horizontal, 12)
            .frame(height: 40)
            .background(RoundedRectangle(cornerRadius: 10, style: .continuous)
                .fill(selected ? Pulse.accentPill : (hovering ? Pulse.fill : .clear)))
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
        .onHover { hovering = $0 }
        .accessibilityAddTraits(selected ? .isSelected : [])
    }
}

struct PlaylistSidebarRow: View {
    let playlist: Playlist
    let selected: Bool
    let playing: Bool
    let action: () -> Void
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: 12) {
                PlaylistAvatar(playlist: playlist)
                Text(playlist.name)
                    .font(.pulseBody(15, weight: selected ? .semibold : .medium))
                    .lineLimit(1)
                Spacer(minLength: 4)
                if playing {
                    PulseBadge(text: "PLAYING")
                } else if playlist.prompt != nil {
                    PulseBadge(text: "NEW", filled: false)
                } else if playlist.updated {
                    UnreadDot().accessibilityLabel("Updated")
                }
            }
            .foregroundStyle(selected ? Pulse.accent : Pulse.text)
            .padding(.horizontal, 12)
            .frame(height: 40)
            .background(RoundedRectangle(cornerRadius: 10, style: .continuous)
                .fill(selected ? Pulse.accentPill : (hovering ? Pulse.fill : .clear)))
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
        .onHover { hovering = $0 }
        .accessibilityAddTraits(selected ? .isSelected : [])
    }
}
