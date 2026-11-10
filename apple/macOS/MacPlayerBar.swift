import SwiftUI

struct MacPlayerBar: View {
    @Environment(PlayerModel.self) private var player
    @Environment(AppRouter.self) private var router
    @State private var output = "MacBook"

    var body: some View {
        @Bindable var player = player
        HStack(spacing: 28) {
            HStack(spacing: 12) {
                ArtworkView(artwork: player.current.artwork, cornerRadius: 10).frame(width: 52, height: 52)
                VStack(alignment: .leading, spacing: 2) {
                    Text(player.current.title)
                        .font(PulseFont.display(17, weight: .bold))
                        .tracking(-0.4)
                        .foregroundStyle(Pulse.text)
                    Text(player.current.artist).font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
                }
                Spacer(minLength: 0)
            }
            .frame(minWidth: 200, maxWidth: 300)

            VStack(spacing: 6) {
                TransportControls(compact: true)
                HStack(spacing: 12) {
                    Text(player.position.clock)
                        .font(.pulseBody(12)).monospacedDigit().foregroundStyle(Pulse.textSecondary)
                        .frame(width: 36, alignment: .trailing)
                    WaveformScrubber(trackID: player.current.id, progress: player.progress, bars: 140) {
                        player.seek(fraction: $0)
                    }
                    .frame(height: 30)
                    Text(player.current.duration.clock)
                        .font(.pulseBody(12)).monospacedDigit().foregroundStyle(Pulse.textSecondary)
                        .frame(width: 36, alignment: .leading)
                }
            }
            .frame(maxWidth: 720)

            HStack(spacing: 4) {
                Spacer(minLength: 0)
                IconButton(systemName: "quote.bubble", label: "Full-screen lyrics", size: 40, glyph: 15,
                           tint: router.showLyrics ? Pulse.accent : Pulse.textSecondary) { router.toggleLyrics() }
                IconButton(systemName: "list.bullet", label: "Queue", size: 40, glyph: 15, tint: Pulse.textSecondary) {}
                PulseMenuButton(items: outputs, alignTrailing: true, accessibilityLabel: "Output device: \(output)") {
                    HStack(spacing: 8) {
                        Image(systemName: output == "MacBook" ? "laptopcomputer" : "airpods")
                            .font(.system(size: 13, weight: .medium))
                        Text(output).font(.pulseBody(12, weight: .medium)).lineLimit(1).fixedSize()
                        Image(systemName: "chevron.up.chevron.down").font(.system(size: 9, weight: .bold))
                    }
                    .foregroundStyle(Pulse.textSecondary)
                    .padding(.horizontal, 12)
                    .frame(height: 36)
                }
                .pulse(.chip, shape: .rounded(10))
                Image(systemName: player.volume == 0 ? "speaker.slash.fill" : "speaker.wave.2.fill")
                    .font(.system(size: 12))
                    .foregroundStyle(Pulse.textSecondary)
                    .padding(.leading, 8)
                    .accessibilityHidden(true)
                PulseSlider(value: $player.volume, label: "Volume").frame(width: 96)
            }
            .frame(minWidth: 200, maxWidth: 340)
        }
        .padding(.horizontal, 24)
        .padding(.vertical, 12)
        .background(Color(hex: 0x050505).opacity(0.86))
        .overlay(alignment: .top) { Rectangle().fill(Pulse.hairline).frame(height: 1) }
    }

    private var outputs: [PulseMenuItem] {
        [
            PulseMenuItem(title: "MacBook speakers", systemImage: output == "MacBook" ? "checkmark" : "laptopcomputer") { output = "MacBook" },
            PulseMenuItem(title: "AirPods Pro", systemImage: output == "AirPods" ? "checkmark" : "airpods") { output = "AirPods" },
            PulseMenuItem(title: "Spotify Connect devices…", systemImage: "hifispeaker.2") {},
        ]
    }
}

/// Shuffle · previous · play/pause · next · repeat.
struct TransportControls: View {
    @Environment(PlayerModel.self) private var player
    var compact = false

    var body: some View {
        let big: CGFloat = compact ? 48 : 64
        HStack(spacing: compact ? 10 : 14) {
            IconButton(systemName: "shuffle", label: player.shuffle ? "Shuffle, on" : "Shuffle, off",
                       size: 40, glyph: 15, tint: player.shuffle ? Pulse.accent : Pulse.textSecondary) { player.shuffle.toggle() }
            IconButton(systemName: "backward.fill", label: "Previous", size: compact ? 40 : 52, glyph: compact ? 18 : 24) { player.previous() }
            IconButton(systemName: player.isPlaying ? "pause.fill" : "play.fill",
                       label: player.isPlaying ? "Pause" : "Play",
                       size: big, glyph: compact ? 19 : 24, kind: .primary) { player.togglePlay() }
                .shadow(color: Pulse.accent.opacity(0.3), radius: 16)
            IconButton(systemName: "forward.fill", label: "Next", size: compact ? 40 : 52, glyph: compact ? 18 : 24) { player.next() }
            IconButton(systemName: player.repeatMode == .one ? "repeat.1" : "repeat",
                       label: "Repeat", size: 40, glyph: 15,
                       tint: player.repeatMode == .off ? Pulse.textSecondary : Pulse.accent) { player.cycleRepeat() }
        }
    }
}
