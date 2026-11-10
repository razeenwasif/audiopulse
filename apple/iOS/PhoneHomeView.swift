import SwiftUI

struct PhoneHomeView: View {
    @Environment(AppRouter.self) private var router
    @Environment(AskModel.self) private var ask
    @Environment(PlayerModel.self) private var player

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                HStack {
                    Text(greeting)
                        .font(PulseFont.display(38)).tracking(-0.77)
                        .foregroundStyle(Pulse.text)
                    Spacer()
                    Button { router.tab = .library } label: {
                        Text("MH").font(.system(size: 14, weight: .bold)).foregroundStyle(Pulse.text)
                            .frame(width: 40, height: 40)
                            .background(Circle().fill(Pulse.forest))
                    }
                    .buttonStyle(PlainPressStyle())
                    .accessibilityLabel("Account and settings")
                }

                Button {
                    ask.startListening()
                    router.showAsk = true
                } label: {
                    HStack(spacing: 10) {
                        SparkleIcon(size: 17)
                        Text("Search, or ask Pulse anything").font(.pulseBody(15)).foregroundStyle(Pulse.textSecondary)
                        Spacer()
                        Image(systemName: "mic.fill").font(.system(size: 17, weight: .semibold))
                            .foregroundStyle(Pulse.onAccent)
                            .frame(width: 44, height: 44)
                            .background(RoundedRectangle(cornerRadius: 15, style: .continuous).fill(Pulse.accent))
                    }
                    .padding(.leading, 16).padding(.trailing, 6)
                    .frame(height: 56)
                    .background(RoundedRectangle(cornerRadius: 20, style: .continuous).fill(Color.white.opacity(0.07)))
                    .overlay(RoundedRectangle(cornerRadius: 20, style: .continuous).strokeBorder(Pulse.hairlineStrong))
                }
                .buttonStyle(PlainPressStyle())
                .accessibilityLabel("Search or ask Pulse")

                LazyVGrid(columns: [GridItem(.flexible(), spacing: 8), GridItem(.flexible(), spacing: 8)], spacing: 8) {
                    QuickTile(name: "Liked Songs", art: Artwork(colors: [0x1E8F62, 0xB8F5CF], angle: 135), playing: false) {
                        router.tab = .library
                    }
                    ForEach(MockLibrary.playlists) { p in
                        QuickTile(name: p.name, art: p.artwork, playing: player.context?.id == p.id && player.isPlaying) {
                            router.phonePath.append(.playlist(p.id))
                        }
                    }
                }

                VStack(alignment: .leading, spacing: 10) {
                    Text("Built from your library")
                        .font(PulseFont.display(22, weight: .bold)).tracking(-0.32)
                        .foregroundStyle(Pulse.text)
                    ScrollView(.horizontal) {
                        HStack(spacing: 12) {
                            ForEach(MockLibrary.builtFromLibrary, id: \.name) { item in
                                Button { router.phonePath.append(.playlist(MockLibrary.rainyIndie.id)) } label: {
                                    VStack(alignment: .leading, spacing: 8) {
                                        ArtworkView(artwork: item.art, cornerRadius: 14).frame(width: 140, height: 140)
                                        Text(item.name).font(.pulseBody(14, weight: .semibold)).foregroundStyle(Pulse.text)
                                        Text(item.why).font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary).lineLimit(1)
                                    }
                                    .frame(width: 140, alignment: .leading)
                                }
                                .buttonStyle(PlainPressStyle())
                            }
                        }
                    }
                    .scrollIndicators(.never)
                }

                VStack(alignment: .leading, spacing: 10) {
                    Text("Because you played Low Tide")
                        .font(PulseFont.display(22, weight: .bold)).tracking(-0.32)
                        .foregroundStyle(Pulse.text)
                    ForEach(MockLibrary.picks.prefix(3)) { t in
                        QueueRow(track: t) { player.play(t, in: MockLibrary.nightDrive) }
                    }
                }
            }
            .padding(.horizontal, 20)
            .padding(.top, 8)
        }
        .scrollIndicators(.never)
        .background(GlowBackdrop(primary: UnitPoint(x: 0, y: 0), secondary: UnitPoint(x: 1, y: 0.6), strength: 0.6))
    }

    private var greeting: String {
        let h = Calendar.current.component(.hour, from: .now)
        return h < 12 ? "Good morning" : (h < 18 ? "Good afternoon" : "Good evening")
    }
}

struct QuickTile: View {
    let name: String
    let art: Artwork
    let playing: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 10) {
                ArtworkView(artwork: art, cornerRadius: 0).frame(width: 52, height: 52)
                Text(name).font(.pulseBody(13, weight: .semibold)).foregroundStyle(Pulse.text)
                    .lineLimit(2).multilineTextAlignment(.leading)
                Spacer(minLength: 0)
                if playing { NowPlayingBars().padding(.trailing, 8) }
            }
            .background(Color.white.opacity(0.06))
            .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
    }
}
