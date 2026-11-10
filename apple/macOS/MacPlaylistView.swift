import SwiftUI

struct MacPlaylistView: View {
    let playlist: Playlist
    @Environment(PlayerModel.self) private var player
    @Environment(AppRouter.self) private var router

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 26) {
                header
                actions
                trackTable
            }
            .padding(.horizontal, 40)
            .padding(.top, 34)
            .padding(.bottom, 24)
        }
        .scrollIndicators(.never)
        .background(alignment: .top) {
            RadialGradient(colors: [Pulse.accentDeep.opacity(0.16), .clear], center: .top, startRadius: 0, endRadius: 700)
                .frame(height: 520)
                .allowsHitTesting(false)
        }
    }

    private var header: some View {
        HStack(alignment: .bottom, spacing: 32) {
            ArtworkView(artwork: playlist.artwork, cornerRadius: 22)
                .frame(width: 220, height: 220)
                .shadow(color: .black.opacity(0.5), radius: 35, y: 30)
            VStack(alignment: .leading, spacing: 14) {
                if playlist.prompt != nil {
                    HStack(spacing: 8) {
                        Image(systemName: "sparkles").font(.system(size: 12, weight: .bold))
                        Text("MADE BY PULSE · JUST NOW").font(.pulseCaption).tracking(1.1)
                    }
                    .foregroundStyle(Pulse.accent)
                } else {
                    Text("PLAYLIST").sectionLabel()
                }
                Text(playlist.name)
                    .font(PulseFont.display(96))
                    .tracking(-2.25)
                    .lineLimit(1)
                    .minimumScaleFactor(0.5)
                    .foregroundStyle(Pulse.text)
                if let prompt = playlist.prompt {
                    Text("You asked: “\(prompt)”")
                        .font(.pulseBody(14))
                        .foregroundStyle(Color(hex: 0xCDD3D0))
                        .padding(.horizontal, 12).padding(.vertical, 6)
                        .background(Capsule().fill(Pulse.fill))
                        .overlay(Capsule().strokeBorder(Pulse.hairline))
                }
                Text("\(playlist.trackCount) songs" + (playlist.tracksLoaded ? " · \(playlist.totalDuration.longDuration)" : "") +
                     (playlist.prompt != nil ? " · every song already in your library · saved to Spotify" : ""))
                    .font(.pulseBody(14))
                    .foregroundStyle(Pulse.textSecondary)
            }
        }
    }

    private var actions: some View {
        HStack(spacing: 10) {
            IconButton(systemName: "play.fill", label: "Play \(playlist.name)", size: 56, glyph: 22, kind: .primary) {
                if let first = playlist.tracks.first { player.play(first, in: playlist) }
            }
            .shadow(color: Pulse.accent.opacity(0.35), radius: 18)
            Button {
                player.shuffle = true
                if let t = playlist.tracks.randomElement() { player.play(t, in: playlist) }
            } label: {
                Label("Smart shuffle", systemImage: "shuffle")
                    .font(.pulseBody(14, weight: .medium))
                    .padding(.horizontal, 16).frame(height: 40)
            }
            .pulse(.outline)
            Button { router.openAsk() } label: {
                Label("Tweak with Pulse", systemImage: "sparkles")
                    .font(.pulseBody(14, weight: .medium))
                    .padding(.horizontal, 16).frame(height: 40)
            }
            .pulse(.outline)
            MoreMenuButton(items: [
                PulseMenuItem(title: "Add to queue", systemImage: "text.line.last.and.arrowtriangle.forward") {},
                PulseMenuItem(title: "Download for offline", systemImage: "arrow.down.circle") {},
                PulseMenuItem(title: "Rename", systemImage: "pencil") {},
                PulseMenuItem(title: "Share link", systemImage: "link") {},
                PulseMenuItem(title: "Delete playlist", systemImage: "trash", destructive: true) {},
            ])
        }
    }

    private var showGenre: Bool { playlist.tracks.contains { !$0.genre.isEmpty } }

    private var trackTable: some View {
        VStack(spacing: 0) {
            if !playlist.tracksLoaded {
                HStack(spacing: 12) {
                    ProgressDots()
                    Text("Loading songs…").font(.pulseBody(14)).foregroundStyle(Pulse.textSecondary)
                }
                .frame(maxWidth: .infinity, minHeight: 120)
            }
            HStack(spacing: 12) {
                Text("#").frame(width: 28, alignment: .leading)
                Text("TITLE").frame(maxWidth: .infinity, alignment: .leading)
                Text(showGenre ? "GENRE" : "ALBUM").frame(width: showGenre ? 130 : 220, alignment: .leading)
                if playlist.prompt != nil {
                    Text("WHY PULSE PICKED IT").frame(width: 300, alignment: .leading)
                }
                Text("TIME").frame(width: 50, alignment: .trailing)
                Color.clear.frame(width: 36)
            }
            .sectionLabel()
            .padding(.horizontal, 12)
            .padding(.vertical, 8)
            .overlay(alignment: .bottom) { Rectangle().fill(Pulse.hairline).frame(height: 1) }
            .padding(.bottom, 6)

            ForEach(Array(playlist.tracks.enumerated()), id: \.element.id) { i, track in
                TrackRow(index: i + 1, track: track, showWhy: playlist.prompt != nil, showGenre: showGenre,
                         isCurrent: player.current.id == track.id, isPlaying: player.isPlaying) {
                    player.play(track, in: playlist)
                }
            }
        }
    }
}

struct TrackRow: View {
    let index: Int
    let track: Track
    let showWhy: Bool
    var showGenre = true
    let isCurrent: Bool
    let isPlaying: Bool
    let play: () -> Void
    @State private var hovering = false

    var body: some View {
        HStack(spacing: 12) {
            Group {
                if hovering {
                    Image(systemName: "play.fill").font(.system(size: 12, weight: .bold)).foregroundStyle(Pulse.accent)
                } else if isCurrent && isPlaying {
                    NowPlayingBars()
                } else {
                    Text("\(index)").font(.pulseBody(14)).monospacedDigit()
                        .foregroundStyle(isCurrent ? Pulse.accent : Pulse.textTertiary)
                }
            }
            .frame(width: 28, alignment: .leading)

            HStack(spacing: 12) {
                ArtworkView(artwork: track.artwork, cornerRadius: 8).frame(width: 40, height: 40)
                VStack(alignment: .leading, spacing: 1) {
                    Text(track.title).font(.pulseBody(15, weight: .semibold))
                        .foregroundStyle(isCurrent ? Pulse.accent : Pulse.text).lineLimit(1)
                    Text(track.artist).font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary).lineLimit(1)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)

            if showGenre {
                HStack(spacing: 6) {
                    Circle().fill(genreColor).frame(width: 8, height: 8)
                    Text(track.genre).font(.pulseBody(13)).foregroundStyle(Color(hex: 0xCDD3D0))
                }
                .frame(width: 130, alignment: .leading)
            } else {
                Text(track.album).font(.pulseBody(13)).foregroundStyle(Color(hex: 0xCDD3D0))
                    .lineLimit(1)
                    .frame(width: 220, alignment: .leading)
            }

            if showWhy {
                Text(track.why ?? "")
                    .font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)
                    .lineLimit(1)
                    .frame(width: 300, alignment: .leading)
            }

            Text(track.duration.clock).font(.pulseBody(13)).monospacedDigit()
                .foregroundStyle(Pulse.textSecondary)
                .frame(width: 50, alignment: .trailing)

            MoreMenuButton(items: [
                PulseMenuItem(title: "Play next", systemImage: "text.line.first.and.arrowtriangle.forward") {},
                PulseMenuItem(title: "More like this", systemImage: "sparkles") {},
                PulseMenuItem(title: "Go to artist", systemImage: "person") {},
                PulseMenuItem(title: "Remove from playlist", systemImage: "minus.circle", destructive: true) {},
            ], size: 36)
            .opacity(hovering ? 1 : 0)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(RoundedRectangle(cornerRadius: 10, style: .continuous)
            .fill(hovering ? Pulse.fill : (isCurrent ? Pulse.accent.opacity(0.06) : .clear)))
        .contentShape(Rectangle())
        .onHover { hovering = $0 }
        .onTapGesture(count: 2, perform: play)
        .accessibilityElement(children: .combine)
        .accessibilityAddTraits(.isButton)
        .accessibilityAction(named: "Play", play)
    }

    private var genreColor: Color {
        switch track.genre {
        case "Indie": Pulse.accent
        case "Indie folk": Pulse.mintPale
        case "Dream pop": Color(hex: 0x2DD4BF)
        case "Lo-fi": Pulse.lime
        default: Color(hex: 0x86A873)
        }
    }
}
