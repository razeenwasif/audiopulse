import SwiftUI

struct MacFullScreenLyrics: View {
    @Environment(PlayerModel.self) private var player
    @Environment(AppRouter.self) private var router
    @State private var tab = 0

    var body: some View {
        ZStack {
            Pulse.backgroundDeep
            GlowBackdrop(primary: UnitPoint(x: 0.18, y: 0.4), secondary: UnitPoint(x: 1, y: 1), strength: 1.1)

            VStack(spacing: 0) {
                HStack {
                    Button { router.toggleLyrics() } label: {
                        HStack(spacing: 8) {
                            Image(systemName: "arrow.down.right.and.arrow.up.left").font(.system(size: 13, weight: .semibold))
                            Text("Exit full screen").font(.pulseBody(14, weight: .medium))
                            Text("esc").font(.pulseBody(12)).foregroundStyle(Pulse.textSecondary)
                        }
                        .padding(.horizontal, 14).frame(height: 40)
                    }
                    .pulse(.chip)
                    Spacer()
                    PulseSegmented(options: [(0, "Lyrics"), (1, "Up next"), (2, "Credits")], selection: $tab)
                    Spacer()
                    Text("Click a line to jump to it").font(.pulseBody(12)).foregroundStyle(Pulse.textSecondary)
                }
                .padding(.horizontal, 32)
                .padding(.top, 44)

                HStack(alignment: .center, spacing: 56) {
                    VStack(alignment: .leading, spacing: 22) {
                        ArtworkView(artwork: player.current.artwork, cornerRadius: 26)
                            .aspectRatio(1, contentMode: .fit)
                            .shadow(color: Pulse.accent.opacity(0.25), radius: 60, y: 30)
                        Text(player.current.title)
                            .font(PulseFont.display(72))
                            .tracking(-1.57)
                            .lineLimit(2)
                            .minimumScaleFactor(0.5)
                            .foregroundStyle(Pulse.text)
                        Text("\(player.current.artist) · \(player.current.album)")
                            .font(.pulseBody(18)).foregroundStyle(Pulse.textSecondary)
                    }
                    .frame(maxWidth: 400)

                    Group {
                        switch tab {
                        case 0:
                            LyricsColumn(currentSize: 64, otherSize: 34, visible: 7, spacing: 18)
                        case 1:
                            VStack(alignment: .leading, spacing: 12) {
                                ForEach(player.upNext) { t in QueueRow(track: t) { player.play(t) } }
                            }
                        default:
                            VStack(alignment: .leading, spacing: 10) {
                                Text("Written by Marlow Vance").font(PulseFont.display(28, weight: .bold))
                                Text("Produced by Haru Sato · Mixed at Tidewater").font(.pulseBody(16)).foregroundStyle(Pulse.textSecondary)
                            }
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
                .padding(.horizontal, 56)
                .frame(maxHeight: .infinity)

                VStack(spacing: 12) {
                    HStack(spacing: 18) {
                        Text(player.position.clock)
                            .font(PulseFont.display(26, weight: .bold)).monospacedDigit().foregroundStyle(Pulse.text)
                        WaveformScrubber(trackID: player.current.id, progress: player.progress, bars: 160, spacing: 3) {
                            player.seek(fraction: $0)
                        }
                        .frame(height: 48)
                        Text(player.current.duration.clock)
                            .font(PulseFont.display(26, weight: .bold)).monospacedDigit().foregroundStyle(Pulse.textDim)
                    }
                    TransportControls()
                }
                .padding(.horizontal, 56)
                .padding(.bottom, 28)
            }
        }
        .ignoresSafeArea()
    }
}
