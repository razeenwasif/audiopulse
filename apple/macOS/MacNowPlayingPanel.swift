import SwiftUI

struct MacNowPlayingPanel: View {
    @Environment(PlayerModel.self) private var player
    @Environment(AppRouter.self) private var router

    var body: some View {
        @Bindable var player = player
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                ArtworkView(artwork: player.current.artwork, cornerRadius: 20)
                    .aspectRatio(1, contentMode: .fit)
                    .shadow(color: Pulse.accentGlow, radius: 40, y: 24)
                    .padding(.top, 34)

                HStack(alignment: .bottom, spacing: 12) {
                    VStack(alignment: .leading, spacing: 6) {
                        Text(player.current.title)
                            .font(PulseFont.display(44))
                            .tracking(-0.9)
                            .lineLimit(2)
                            .minimumScaleFactor(0.6)
                            .foregroundStyle(Pulse.text)
                        Text(player.current.album.isEmpty ? player.current.artist : "\(player.current.artist) · \(player.current.album)")
                            .font(.pulseBody(15))
                            .foregroundStyle(Pulse.textSecondary)
                    }
                    Spacer(minLength: 0)
                    IconButton(systemName: player.isLiked ? "heart.fill" : "heart",
                               label: player.isLiked ? "Remove from Liked Songs" : "Save to Liked Songs",
                               glyph: 18, kind: .tinted, tint: Pulse.accent) {
                        player.isLiked.toggle()
                    }
                }

                VStack(alignment: .leading, spacing: 10) {
                    HStack {
                        Text("LYRICS").sectionLabel()
                        Spacer()
                        Button("Full screen") { router.toggleLyrics() }
                            .font(.pulseBody(12, weight: .medium))
                            .foregroundStyle(Pulse.accent)
                            .buttonStyle(PlainPressStyle())
                    }
                    LyricsColumn(currentSize: 26, otherSize: 18, visible: 3)
                }

                VStack(alignment: .leading, spacing: 10) {
                    Text("UP NEXT").sectionLabel()
                    ForEach(player.upNext.prefix(2)) { t in
                        QueueRow(track: t) { player.play(t) }
                    }
                }
            }
            .padding(.horizontal, 28)
            .padding(.bottom, 20)
        }
        .scrollIndicators(.never)
        .overlay(alignment: .leading) { Rectangle().fill(Pulse.hairline).frame(width: 1) }
    }
}
