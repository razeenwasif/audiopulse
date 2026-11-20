import SwiftUI

struct PhoneNowPlayingView: View {
    @Environment(PlayerModel.self) private var player
    @Environment(AppRouter.self) private var router
    @Environment(AskModel.self) private var ask
    @State private var dragOffset: CGFloat = 0

    var body: some View {
        GeometryReader { geo in
            let tall = geo.size.height >= 720
            let art = min(300, max(150, geo.size.height - (tall ? 560 : 470)))
            content(art: art, showLyrics: tall)
        }
        .pulseMenuHost()
    }

    private func content(art: CGFloat, showLyrics: Bool) -> some View {
        ZStack {
            GlowBackdrop(primary: UnitPoint(x: 0.5, y: 0.25), secondary: UnitPoint(x: 0, y: 1), strength: 0.9)

            VStack(spacing: 16) {
                VStack(spacing: 8) {
                    Capsule().fill(Color.white.opacity(0.3)).frame(width: 40, height: 5)
                    HStack {
                        IconButton(systemName: "chevron.down", label: "Collapse player", glyph: 18) { router.showNowPlaying = false }
                        Spacer()
                        VStack(spacing: 1) {
                            Text("PLAYING FROM").font(.pulseCaption).tracking(1.1).foregroundStyle(Pulse.textSecondary)
                            Text("\(player.context?.name ?? "Your library")\(player.shuffle ? " · Smart shuffle" : "")")
                                .font(.pulseBody(14, weight: .semibold)).foregroundStyle(Pulse.text)
                        }
                        Spacer()
                        MoreMenuButton(items: [
                            PulseMenuItem(title: "More like this", systemImage: "sparkles") {},
                            PulseMenuItem(title: "Add to playlist", systemImage: "plus") {},
                            PulseMenuItem(title: "Go to artist", systemImage: "person") {},
                            PulseMenuItem(title: "Share", systemImage: "square.and.arrow.up") {},
                        ], size: 44)
                    }
                }

                ArtworkView(artwork: player.current.artwork, cornerRadius: 24)
                    .frame(width: art, height: art)
                    .shadow(color: Pulse.accent.opacity(0.3), radius: 45, y: 30)
                    .scaleEffect(player.isPlaying ? 1 : 0.9)
                    .animation(.spring(response: 0.45, dampingFraction: 0.75), value: player.isPlaying)

                HStack(alignment: .bottom) {
                    VStack(alignment: .leading, spacing: 6) {
                        Text(player.current.title)
                            .font(PulseFont.display(54)).tracking(-1.22)
                            .lineLimit(1).minimumScaleFactor(0.5)
                            .foregroundStyle(Pulse.text)
                        Text(player.current.artist).font(.pulseBody(15)).foregroundStyle(Pulse.textSecondary)
                    }
                    Spacer()
                    IconButton(systemName: player.isLiked ? "heart.fill" : "heart",
                               label: player.isLiked ? "Remove from Liked Songs" : "Save to Liked Songs",
                               glyph: 18, kind: .tinted, tint: Pulse.accent) { player.isLiked.toggle() }
                }

                if showLyrics {
                    LyricsColumn(currentSize: 21, otherSize: 17, visible: 2, spacing: 4)
                        .frame(height: 62, alignment: .top)
                        .clipped()
                }

                VStack(spacing: 6) {
                    WaveformScrubber(trackID: player.current.id, progress: player.progress, bars: 64, spacing: 2) {
                        player.seek(fraction: $0)
                    }
                    .frame(height: 52)
                    HStack {
                        Text(player.position.clock)
                        Spacer()
                        Text("-" + (player.current.duration - player.position).clock)
                    }
                    .font(.pulseBody(12)).monospacedDigit().foregroundStyle(Pulse.textSecondary)
                }

                HStack {
                    IconButton(systemName: "shuffle", label: player.shuffle ? "Shuffle, on" : "Shuffle, off", glyph: 18,
                               tint: player.shuffle ? Pulse.accent : Pulse.textSecondary) { player.shuffle.toggle() }
                    Spacer()
                    IconButton(systemName: "backward.fill", label: "Previous", size: 56, glyph: 28) { player.previous() }
                    Spacer()
                    IconButton(systemName: player.isPlaying ? "pause.fill" : "play.fill", label: player.isPlaying ? "Pause" : "Play",
                               size: 74, glyph: 28, kind: .primary) { player.togglePlay() }
                        .shadow(color: Pulse.accent.opacity(0.35), radius: 20)
                    Spacer()
                    IconButton(systemName: "forward.fill", label: "Next", size: 56, glyph: 28) { player.next() }
                    Spacer()
                    IconButton(systemName: player.repeatMode == .one ? "repeat.1" : "repeat", label: "Repeat", glyph: 18,
                               tint: player.repeatMode == .off ? Pulse.textSecondary : Pulse.accent) { player.cycleRepeat() }
                }

                Spacer(minLength: 0)

                Button {
                    router.showNowPlaying = false
                    ask.startListening()
                    Task { try? await Task.sleep(for: .milliseconds(350)); router.showAsk = true }
                } label: {
                    HStack(spacing: 10) {
                        SparkleIcon(size: 16)
                        Text("More like this, but slower…").font(.pulseBody(15)).foregroundStyle(Pulse.textSecondary)
                        Spacer()
                        Image(systemName: "mic.fill").font(.system(size: 17, weight: .semibold))
                            .foregroundStyle(Pulse.onAccent)
                            .frame(width: 44, height: 44)
                            .background(RoundedRectangle(cornerRadius: 16, style: .continuous).fill(Pulse.accent))
                    }
                    .padding(.leading, 16).padding(.trailing, 6).frame(height: 56)
                    .background(RoundedRectangle(cornerRadius: 22, style: .continuous).fill(Color.white.opacity(0.08)))
                    .overlay(RoundedRectangle(cornerRadius: 22, style: .continuous).strokeBorder(Pulse.hairlineStrong))
                }
                .buttonStyle(PlainPressStyle())
                .accessibilityLabel("Ask Pulse")
            }
            .padding(.horizontal, 24)
            .padding(.bottom, 12)
            .offset(y: dragOffset)
            .gesture(DragGesture()
                .onChanged { dragOffset = max(0, $0.translation.height) }
                .onEnded { g in
                    if g.translation.height > 140 { router.showNowPlaying = false }
                    withAnimation(.spring(response: 0.35, dampingFraction: 0.8)) { dragOffset = 0 }
                })
        }
    }
}
