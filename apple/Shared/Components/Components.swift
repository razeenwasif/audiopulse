import SwiftUI

// MARK: - Artwork

struct ArtworkView: View {
    let artwork: Artwork
    var cornerRadius: CGFloat = 12

    var body: some View {
        RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
            .fill(LinearGradient(
                colors: artwork.colors.map { Color(hex: $0) },
                startPoint: UnitPoint(angle: artwork.angle, start: true),
                endPoint: UnitPoint(angle: artwork.angle, start: false)))
            .overlay {
                if let h = artwork.highlight {
                    RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
                        .fill(RadialGradient(colors: [Color(hex: h), Color(hex: h).opacity(0)],
                                             center: UnitPoint(x: 0.3, y: 0.28), startRadius: 0, endRadius: 140))
                        .blendMode(.screen)
                        .opacity(0.9)
                }
            }
            .accessibilityHidden(true)
    }
}

private extension UnitPoint {
    /// CSS-style gradient angle (0° = up, 90° = right).
    init(angle: Double, start: Bool) {
        let r = (angle - 90) * .pi / 180
        let dx = cos(r) * 0.5, dy = sin(r) * 0.5
        self = start ? UnitPoint(x: 0.5 - dx, y: 0.5 - dy) : UnitPoint(x: 0.5 + dx, y: 0.5 + dy)
    }
}

// MARK: - Soft green glow backdrop

struct GlowBackdrop: View {
    var primary = UnitPoint(x: 0.8, y: 0.15)
    var secondary = UnitPoint(x: 0.3, y: 1.1)
    var strength: Double = 1

    var body: some View {
        GeometryReader { geo in
            let m = max(geo.size.width, geo.size.height)
            ZStack {
                Pulse.background
                RadialGradient(colors: [Pulse.accent.opacity(0.20 * strength), .clear],
                               center: primary, startRadius: 0, endRadius: m * 0.55)
                RadialGradient(colors: [Pulse.accentDeep.opacity(0.10 * strength), .clear],
                               center: secondary, startRadius: 0, endRadius: m * 0.5)
            }
        }
        .ignoresSafeArea()
        .allowsHitTesting(false)
    }
}

// MARK: - Waveform scrubber (Console's contribution)

/// Deterministic bar heights per track so the shape is stable across renders.
func waveformSamples(for id: String, count: Int) -> [Double] {
    var seed = UInt64(abs(id.hashValue)) | 1
    var out: [Double] = []
    out.reserveCapacity(count)
    for i in 0..<count {
        seed = seed &* 6364136223846793005 &+ 1442695040888963407
        let noise = Double(seed >> 33) / Double(UInt32.max)
        let shape = abs(sin(Double(i) * 0.37) * cos(Double(i) * 0.11))
        out.append(0.2 + 0.8 * (0.65 * shape + 0.35 * noise))
    }
    return out
}

struct WaveformScrubber: View {
    let trackID: String
    let progress: Double
    var bars = 120
    var spacing: CGFloat = 2
    var onSeek: ((Double) -> Void)? = nil
    @State private var hoverX: CGFloat?
    @State private var dragFraction: Double?

    var body: some View {
        let samples = waveformSamples(for: trackID, count: bars)
        GeometryReader { geo in
            let shown = dragFraction ?? progress
            Canvas { ctx, size in
                let barW = max(1, (size.width - spacing * CGFloat(bars - 1)) / CGFloat(bars))
                let played = Int((shown * Double(bars)).rounded())
                for (i, s) in samples.enumerated() {
                    let h = size.height * s
                    let rect = CGRect(x: CGFloat(i) * (barW + spacing), y: (size.height - h) / 2, width: barW, height: h)
                    let color: Color
                    if i < played {
                        let t = played > 1 ? Double(i) / Double(played - 1) : 1
                        color = Color(hex: 0x1E8F62).mix(with: Pulse.accent, by: t)
                    } else if let hx = hoverX, rect.minX <= hx {
                        color = Color.white.opacity(0.32)
                    } else {
                        color = Color.white.opacity(0.16)
                    }
                    ctx.fill(Path(roundedRect: rect, cornerRadius: barW / 2), with: .color(color))
                }
            }
            .contentShape(Rectangle())
            .onContinuousHover { phase in
                if case .active(let p) = phase { hoverX = p.x } else { hoverX = nil }
            }
            .gesture(DragGesture(minimumDistance: 0)
                .onChanged { g in
                    guard onSeek != nil else { return }
                    dragFraction = min(1, max(0, g.location.x / geo.size.width))
                }
                .onEnded { _ in
                    if let f = dragFraction { onSeek?(f) }
                    dragFraction = nil
                })
        }
        .accessibilityElement()
        .accessibilityLabel("Playback position")
        .accessibilityValue("\(Int(progress * 100)) percent")
        .accessibilityAdjustableAction { dir in
            switch dir {
            case .increment: onSeek?(min(1, progress + 0.05))
            case .decrement: onSeek?(max(0, progress - 0.05))
            @unknown default: break
            }
        }
    }
}

/// Live microphone level while Ask Pulse is listening.
struct VoiceLevelBars: View {
    let level: Double
    var bars = 40

    var body: some View {
        TimelineView(.animation) { tl in
            let t = tl.date.timeIntervalSinceReferenceDate
            Canvas { ctx, size in
                let spacing: CGFloat = 3
                let w = (size.width - spacing * CGFloat(bars - 1)) / CGFloat(bars)
                for i in 0..<bars {
                    let env = sin(Double(i) / Double(bars - 1) * .pi)
                    let wobble = abs(sin(Double(i) * 1.7 + t * 6) * cos(Double(i) * 0.6 - t * 3))
                    let v = 0.08 + env * (0.2 + level * 0.8 * wobble)
                    let h = size.height * min(1, v)
                    let rect = CGRect(x: CGFloat(i) * (w + spacing), y: (size.height - h) / 2, width: w, height: h)
                    ctx.fill(Path(roundedRect: rect, cornerRadius: w / 2),
                             with: .color(env > 0.35 ? Pulse.accent : Pulse.accentDeep))
                }
            }
        }
        .accessibilityHidden(true)
    }
}

// MARK: - Small pieces

struct PlaylistAvatar: View {
    let playlist: Playlist
    var size: CGFloat = 28

    var body: some View {
        Text(playlist.initials)
            .font(.system(size: size * 0.38, weight: .bold))
            .foregroundStyle(playlist.badgeOnDark ? Pulse.text : Pulse.onAccent)
            .frame(width: size, height: size)
            .background(Circle().fill(Color(hex: playlist.badgeColor)))
            .accessibilityHidden(true)
    }
}

struct NowPlayingBars: View {
    var body: some View {
        TimelineView(.animation(minimumInterval: 0.12)) { tl in
            let t = tl.date.timeIntervalSinceReferenceDate
            HStack(alignment: .bottom, spacing: 2) {
                ForEach(0..<3) { i in
                    Capsule().fill(Pulse.accent)
                        .frame(width: 3, height: 4 + 8 * abs(sin(t * 4 + Double(i) * 1.3)))
                }
            }
            .frame(height: 12, alignment: .bottom)
        }
        .accessibilityLabel("Playing")
    }
}

struct SparkleIcon: View {
    var size: CGFloat = 18
    var body: some View {
        Image(systemName: "sparkles")
            .font(.system(size: size, weight: .semibold))
            .foregroundStyle(Pulse.accent)
    }
}

/// Synced lyrics column (Poster's big type). Tapping a line seeks to it.
struct LyricsColumn: View {
    @Environment(PlayerModel.self) private var player
    var currentSize: CGFloat = 26
    var otherSize: CGFloat = 18
    var visible = 3
    var spacing: CGFloat = 10

    var body: some View {
        let lines = player.lyrics
        let cur = player.currentLyricIndex
        let lo = max(0, cur - 1), hi = min(lines.count, lo + visible)
        VStack(alignment: .leading, spacing: spacing) {
            ForEach(Array(lines[lo..<hi].enumerated()), id: \.element.id) { offset, line in
                let i = lo + offset
                Button { player.seek(to: line.time) } label: {
                    Text(line.text)
                        .font(PulseFont.display(i == cur ? currentSize : otherSize, weight: i == cur ? .heavy : .bold))
                        .tracking(-0.5)
                        .foregroundStyle(i == cur ? Pulse.text : (i < cur ? Pulse.textPast : Pulse.textDim))
                        .multilineTextAlignment(.leading)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .fixedSize(horizontal: false, vertical: true)
                }
                .buttonStyle(PlainPressStyle())
                .animation(.spring(response: 0.4, dampingFraction: 0.85), value: cur)
            }
        }
    }
}
