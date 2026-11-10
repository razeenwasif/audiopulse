import SwiftUI

struct MacHomeView: View {
    @Environment(AppRouter.self) private var router
    @Environment(AskModel.self) private var ask
    @Environment(PlayerModel.self) private var player

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 28) {
                Text(greeting)
                    .font(PulseFont.display(44))
                    .tracking(-0.81)
                    .foregroundStyle(Pulse.text)
                    .padding(.top, 34)

                VStack(alignment: .leading, spacing: 14) {
                    AskBar { router.openAsk() }
                    ViewThatFits(in: .horizontal) {
                        promptRow(MockLibrary.prompts)
                        promptRow(Array(MockLibrary.prompts.prefix(2)))
                        promptRow(Array(MockLibrary.prompts.prefix(1)))
                    }
                }

                section("Because you played Low Tide all week", trailing: "See all") {
                    LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 18), count: 5), spacing: 18) {
                        ForEach(MockLibrary.picks) { track in
                            PickCard(track: track) { player.play(track, in: MockLibrary.nightDrive) }
                        }
                    }
                }

                section("Built from your library") {
                    LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 14), count: 3), spacing: 14) {
                        ForEach(MockLibrary.builtFromLibrary, id: \.name) { item in
                            BuiltCard(name: item.name, why: item.why, art: item.art) {
                                router.go(.playlist(MockLibrary.rainyIndie.id))
                            }
                        }
                    }
                }
            }
            .padding(.horizontal, 36)
            .padding(.bottom, 32)
        }
        .scrollIndicators(.never)
    }

    private func promptRow(_ prompts: [String]) -> some View {
        HStack(spacing: 8) {
            ForEach(prompts, id: \.self) { prompt in
                Button {
                    ask.query = prompt
                    ask.submit()
                    router.openAsk()
                } label: {
                    Text(prompt)
                        .font(.pulseBody(13))
                        .foregroundStyle(Pulse.text.opacity(0.85))
                        .lineLimit(1)
                        .fixedSize()
                        .padding(.horizontal, 14)
                        .frame(height: 34)
                }
                .pulse(.outline)
            }
        }
    }

    private var greeting: String {
        let h = Calendar.current.component(.hour, from: .now)
        return h < 12 ? "Good morning" : (h < 18 ? "Good afternoon" : "Good evening")
    }

    private func section<C: View>(_ title: String, trailing: String? = nil, @ViewBuilder content: () -> C) -> some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack(alignment: .firstTextBaseline) {
                Text(title)
                    .font(PulseFont.display(26, weight: .bold))
                    .tracking(-0.36)
                    .foregroundStyle(Pulse.text)
                Spacer()
                if let trailing {
                    Button(trailing) {}
                        .font(.pulseBody(13, weight: .medium))
                        .foregroundStyle(Pulse.textSecondary)
                        .buttonStyle(PlainPressStyle())
                }
            }
            content()
        }
    }
}

/// The search/ask entry point. Looks like a field; opens the Ask palette.
struct AskBar: View {
    let action: () -> Void
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: 12) {
                SparkleIcon(size: 18)
                Text("Search, or ask: “something like this, but slower”")
                    .font(.pulseBody(16))
                    .foregroundStyle(Pulse.textSecondary)
                Spacer()
                Text("⌘K")
                    .font(.pulseBody(12))
                    .foregroundStyle(Pulse.textTertiary)
                    .padding(.horizontal, 8).padding(.vertical, 4)
                    .overlay(RoundedRectangle(cornerRadius: 6).strokeBorder(Pulse.hairlineStrong))
                Image(systemName: "mic.fill")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(Pulse.onAccent)
                    .frame(width: 44, height: 44)
                    .background(RoundedRectangle(cornerRadius: 12, style: .continuous).fill(Pulse.accent))
            }
            .padding(.leading, 18)
            .padding([.trailing, .vertical], 8)
            .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Color.white.opacity(hovering ? 0.08 : 0.06)))
            .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous)
                .strokeBorder(hovering ? Pulse.accent.opacity(0.35) : Pulse.hairlineStrong))
            .shadow(color: .black.opacity(0.35), radius: 20, y: 10)
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
        .onHover { h in withAnimation(.easeOut(duration: 0.15)) { hovering = h } }
        .accessibilityLabel("Search or ask Pulse")
    }
}

struct PickCard: View {
    let track: Track
    let action: () -> Void
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            VStack(alignment: .leading, spacing: 10) {
                ArtworkView(artwork: track.artwork, cornerRadius: 14)
                    .aspectRatio(1, contentMode: .fit)
                    .overlay(alignment: .bottomTrailing) {
                        Image(systemName: "play.fill")
                            .font(.system(size: 16, weight: .bold))
                            .foregroundStyle(Pulse.onAccent)
                            .frame(width: 44, height: 44)
                            .background(Circle().fill(Pulse.accent))
                            .shadow(color: .black.opacity(0.4), radius: 10, y: 6)
                            .padding(10)
                            .opacity(hovering ? 1 : 0)
                            .offset(y: hovering ? 0 : 8)
                    }
                    .shadow(color: .black.opacity(hovering ? 0.5 : 0.4), radius: hovering ? 22 : 15, y: hovering ? 22 : 14)
                    .offset(y: hovering ? -4 : 0)
                VStack(alignment: .leading, spacing: 2) {
                    Text(track.title).font(.pulseBody(14, weight: .semibold)).foregroundStyle(Pulse.text)
                    Text(track.artist).font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
                }
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
        .onHover { h in withAnimation(.spring(response: 0.25, dampingFraction: 0.8)) { hovering = h } }
        .accessibilityLabel("Play \(track.title) by \(track.artist)")
    }
}

struct BuiltCard: View {
    let name: String
    let why: String
    let art: Artwork
    let action: () -> Void
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: 14) {
                ArtworkView(artwork: art, cornerRadius: 12).frame(width: 56, height: 56)
                VStack(alignment: .leading, spacing: 3) {
                    Text(name).font(.pulseBody(14, weight: .semibold)).foregroundStyle(Pulse.text).lineLimit(1)
                    Text(why).font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary).lineLimit(2)
                }
                Spacer(minLength: 0)
            }
            .padding(12)
            .background(RoundedRectangle(cornerRadius: 16, style: .continuous).fill(Color.white.opacity(hovering ? 0.07 : 0.04)))
            .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous).strokeBorder(hovering ? Pulse.hairlineStrong : Pulse.hairline))
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
        .onHover { h in withAnimation(.easeOut(duration: 0.15)) { hovering = h } }
    }
}
