import SwiftUI

struct AskPalette: View {
    @Environment(AskModel.self) private var ask
    @Environment(AppRouter.self) private var router
    @Environment(PlayerModel.self) private var player
    @FocusState private var fieldFocused: Bool

    var body: some View {
        @Bindable var ask = ask
        VStack(spacing: 0) {
            HStack(spacing: 12) {
                SparkleIcon(size: 20)
                TextField("", text: $ask.query, prompt: Text("Search, or ask anything").foregroundStyle(Pulse.textTertiary))
                    .textFieldStyle(.plain)
                    .font(PulseFont.display(22, weight: .semibold))
                    .tracking(-0.4)
                    .foregroundStyle(Pulse.text)
                    .focused($fieldFocused)
                    .onSubmit { runHighlighted() }
                    .onKeyPress(.downArrow) { ask.moveHighlight(1); return .handled }
                    .onKeyPress(.upArrow) { ask.moveHighlight(-1); return .handled }
                    .onKeyPress(.escape) { router.closeAsk(); return .handled }
                    .accessibilityLabel("Search or ask Pulse")
                IconButton(systemName: ask.phase == .listening ? "stop.fill" : "mic.fill",
                           label: ask.phase == .listening ? "Stop listening" : "Speak to Pulse",
                           size: 36, glyph: 14, kind: ask.phase == .listening ? .primary : .chip) {
                    ask.phase == .listening ? ask.stopListening() : ask.startListening()
                }
                Button { router.closeAsk() } label: {
                    Text("esc").font(.pulseBody(12)).foregroundStyle(Pulse.textSecondary)
                        .padding(.horizontal, 8).frame(height: 24)
                }
                .pulse(.outline, shape: .rounded(6))
            }
            .padding(.leading, 20)
            .padding(.trailing, 14)
            .frame(height: 64)
            .overlay(alignment: .bottom) { Rectangle().fill(Pulse.hairline).frame(height: 1) }

            HStack {
                PulseChips(options: AskModel.Filter.allCases.map { ($0, $0.rawValue) }, selection: $ask.filter)
                Spacer()
            }
            .padding(.horizontal, 20)
            .padding(.top, 12)

            Group {
                switch ask.phase {
                case .listening:
                    VoiceLevelBars(level: ask.level).frame(height: 56).padding(.horizontal, 24).padding(.vertical, 28)
                case .thinking:
                    HStack(spacing: 10) {
                        ProgressDots()
                        Text("Pulse is reading your library…").font(.pulseBody(14)).foregroundStyle(Pulse.textSecondary)
                    }
                    .frame(maxWidth: .infinity, minHeight: 120)
                default:
                    results
                }
            }

            HStack {
                HStack(spacing: 8) {
                    Circle().fill(Pulse.accent).frame(width: 7, height: 7)
                    Text("Runs on this Mac · nothing leaves it")
                }
                Spacer()
                Text("↑↓ move · ↵ run · ⇥ ask instead · hold V to speak")
            }
            .font(.pulseBody(12))
            .foregroundStyle(Pulse.textTertiary)
            .padding(.horizontal, 20)
            .frame(height: 44)
            .overlay(alignment: .top) { Rectangle().fill(Pulse.hairline).frame(height: 1) }
        }
        .background(RoundedRectangle(cornerRadius: 20, style: .continuous).fill(Color(hex: 0x0E0F0E).opacity(0.97)))
        .overlay(RoundedRectangle(cornerRadius: 20, style: .continuous).strokeBorder(Pulse.hairlineStrong))
        .shadow(color: .black.opacity(0.6), radius: 50, y: 30)
        .shadow(color: Pulse.accent.opacity(0.10), radius: 80)
        .onAppear { fieldFocused = true }
    }

    private var results: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("PULSE CAN").sectionLabel().padding(.horizontal, 8).padding(.top, 10).padding(.bottom, 4)
            ForEach(Array(ask.actions.enumerated()), id: \.element.id) { i, action in
                AskActionRow(action: action, highlighted: ask.highlighted == i) {
                    ask.highlighted = i
                } run: {
                    ask.highlighted = i
                    runHighlighted()
                }
            }

            HStack(alignment: .firstTextBaseline) {
                Text("PREVIEW · RAINY INDIE").sectionLabel()
                Spacer()
                Text("25 songs · 1 h 38 m · all from your library").font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
            }
            .padding(.horizontal, 8)
            .padding(.top, 14)
            .padding(.bottom, 4)

            ForEach(ask.preview) { t in
                HStack(spacing: 12) {
                    ArtworkView(artwork: t.artwork, cornerRadius: 6).frame(width: 32, height: 32)
                    Text(t.title).font(.pulseBody(14, weight: .medium)).foregroundStyle(Pulse.text)
                    Text(t.artist).font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)
                    Spacer()
                    Text(t.why ?? "").font(.pulseBody(12)).foregroundStyle(Pulse.textSecondary)
                        .padding(.horizontal, 8).padding(.vertical, 2)
                        .background(Capsule().fill(Pulse.fill))
                        .lineLimit(1)
                    Text(t.duration.clock).font(.pulseBody(12)).monospacedDigit().foregroundStyle(Pulse.textSecondary)
                        .frame(width: 40, alignment: .trailing)
                }
                .padding(.horizontal, 12)
                .frame(height: 44)
            }
        }
        .padding(.horizontal, 12)
        .padding(.bottom, 10)
    }

    private func runHighlighted() {
        switch ask.highlighted {
        case 0: router.go(.playlist(MockLibrary.rainyIndie.id))
        case 1: break
        default: player.shuffle = true
        }
        router.closeAsk()
    }
}

private struct AskActionRow: View {
    let action: AskModel.Action
    let highlighted: Bool
    let hover: () -> Void
    let run: () -> Void

    var body: some View {
        Button(action: run) {
            HStack(spacing: 14) {
                Image(systemName: action.glyph)
                    .font(.system(size: 14, weight: .bold))
                    .foregroundStyle(highlighted ? Pulse.onAccent : Pulse.text)
                    .frame(width: 32, height: 32)
                    .background(RoundedRectangle(cornerRadius: 9, style: .continuous)
                        .fill(highlighted ? Pulse.accent : Color.white.opacity(0.08)))
                VStack(alignment: .leading, spacing: 2) {
                    Text(action.title).font(.pulseBody(15, weight: .semibold)).foregroundStyle(Pulse.text)
                    Text(action.detail).font(.pulseBody(12)).foregroundStyle(Pulse.textSecondary)
                }
                Spacer()
                Text(action.shortcut).font(.pulseBody(12)).foregroundStyle(Pulse.textSecondary)
                    .padding(.horizontal, 7).padding(.vertical, 3)
                    .overlay(RoundedRectangle(cornerRadius: 6).strokeBorder(Pulse.hairlineStrong))
            }
            .padding(.horizontal, 12)
            .frame(minHeight: 52)
            .background(RoundedRectangle(cornerRadius: 12, style: .continuous).fill(highlighted ? Pulse.accentPill : .clear))
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
        .onHover { if $0 { hover() } }
        .animation(.easeOut(duration: 0.1), value: highlighted)
    }
}

struct ProgressDots: View {
    var body: some View {
        TimelineView(.animation) { tl in
            let t = tl.date.timeIntervalSinceReferenceDate
            HStack(spacing: 5) {
                ForEach(0..<3) { i in
                    Circle().fill(Pulse.accent)
                        .frame(width: 7, height: 7)
                        .opacity(0.3 + 0.7 * abs(sin(t * 3 + Double(i) * 0.6)))
                }
            }
        }
        .accessibilityLabel("Working")
    }
}
