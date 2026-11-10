import SwiftUI

struct QueueRow: View {
    let track: Track
    let action: () -> Void
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: 12) {
                ArtworkView(artwork: track.artwork, cornerRadius: 8)
                    .frame(width: 40, height: 40)
                    .overlay {
                        if hovering {
                            Image(systemName: "play.fill").font(.system(size: 13, weight: .bold)).foregroundStyle(Pulse.text)
                        }
                    }
                VStack(alignment: .leading, spacing: 1) {
                    Text(track.title).font(.pulseBody(14, weight: .medium)).foregroundStyle(Pulse.text)
                    Text(track.artist).font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
                }
                Spacer()
            }
            .padding(4)
            .background(RoundedRectangle(cornerRadius: 10).fill(hovering ? Pulse.fill : .clear))
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
        .onHover { hovering = $0 }
        .accessibilityLabel("Play \(track.title) by \(track.artist)")
    }
}
