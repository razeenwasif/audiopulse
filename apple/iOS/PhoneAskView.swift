import SwiftUI

struct PhoneAskView: View {
    @Environment(AskModel.self) private var ask
    @Environment(AppRouter.self) private var router
    @Environment(LibraryStore.self) private var library
    @FocusState private var typing: Bool
    @State private var showField = false

    var body: some View {
        @Bindable var ask = ask
        ZStack {
            GlowBackdrop(primary: UnitPoint(x: 0.5, y: 0.62), secondary: UnitPoint(x: 0, y: 0), strength: 1.2)

            VStack(alignment: .leading, spacing: 22) {
                HStack {
                    IconButton(systemName: "xmark", label: "Close", glyph: 16, kind: .chip) { close() }
                    Spacer()
                    HStack(spacing: 8) {
                        SparkleIcon(size: 16)
                        Text("Ask Pulse").font(.pulseBody(16, weight: .semibold)).foregroundStyle(Pulse.text)
                    }
                    Spacer()
                    IconButton(systemName: "keyboard", label: "Type instead", glyph: 16, kind: .chip) {
                        ask.stopListening()
                        showField = true
                        typing = true
                    }
                }

                if showField {
                    TextField("", text: $ask.query, prompt: Text("Ask for anything").foregroundStyle(Pulse.textTertiary), axis: .vertical)
                        .textFieldStyle(.plain)
                        .font(PulseFont.display(34))
                        .tracking(-0.59)
                        .foregroundStyle(Pulse.text)
                        .focused($typing)
                        .submitLabel(.go)
                        .onSubmit { ask.submit() }
                } else {
                    transcript
                }

                Spacer(minLength: 0)

                VStack(spacing: 14) {
                    if ask.phase == .listening {
                        VoiceLevelBars(level: ask.level).frame(height: 64)
                    } else if ask.phase == .thinking {
                        ProgressDotsPhone().frame(height: 64)
                    }
                    Button {
                        ask.phase == .listening ? ask.stopListening() : ask.startListening()
                        showField = false
                    } label: {
                        Image(systemName: ask.phase == .listening ? "stop.fill" : "mic.fill")
                            .font(.system(size: 30, weight: .semibold))
                            .foregroundStyle(Pulse.onAccent)
                            .frame(width: 92, height: 92)
                            .background(Circle().fill(Pulse.accent))
                            .background(Circle().fill(Pulse.accent.opacity(0.18)).padding(-12))
                            .background(Circle().fill(Pulse.accent.opacity(0.07)).padding(-26))
                            .scaleEffect(ask.phase == .listening ? 1 + ask.level * 0.06 : 1)
                            .animation(.easeOut(duration: 0.1), value: ask.level)
                    }
                    .buttonStyle(PlainPressStyle())
                    .accessibilityLabel(ask.phase == .listening ? "Stop listening" : "Start listening")
                    .padding(.top, 14)
                    Text(ask.phase == .listening ? "Listening · speech stays on this iPhone" : "Tap to speak")
                        .font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)
                        .padding(.top, 14)
                }
                .frame(maxWidth: .infinity)

                if ask.phase == .results {
                    VStack(alignment: .leading, spacing: 8) {
                        Text("PULSE CAN").sectionLabel().padding(.horizontal, 4)
                        AskCard(glyph: "plus", title: "Make “Rainy Indie”", detail: "25 songs from your library", primary: true) {
                            close()
                            router.tab = .library
                            if library.playlist(MockLibrary.rainyIndie.id) != nil {
                                router.phonePath = [.playlist(MockLibrary.rainyIndie.id)]
                            }
                        }
                        AskCard(glyph: "text.line.first.and.arrowtriangle.forward", title: "Queue 10 like this",
                                detail: "After Low Tide, lower energy", primary: false) { close() }
                    }
                    .transition(.move(edge: .bottom).combined(with: .opacity))
                }
            }
            .padding(.horizontal, 22)
            .padding(.bottom, 16)
            .animation(.spring(response: 0.4, dampingFraction: 0.85), value: ask.phase)
        }
        .task {
            try? await Task.sleep(for: .seconds(2.2))
            if ask.phase == .listening { ask.stopListening() }
        }
    }

    private var transcript: some View {
        let words = ask.query.split(separator: " ").map(String.init)
        let head = words.dropLast().joined(separator: " ")
        let last = words.last ?? ""
        return (Text(head.isEmpty ? "" : head + " ").foregroundStyle(Pulse.text)
                + Text(last).foregroundStyle(ask.phase == .listening ? Pulse.accent : Pulse.text))
            .font(PulseFont.display(40))
            .tracking(-0.72)
            .frame(maxWidth: .infinity, minHeight: 100, alignment: .topLeading)
            .accessibilityLabel(ask.query.isEmpty ? "Listening" : ask.query)
    }

    private func close() {
        ask.stopListening()
        router.showAsk = false
    }
}

private struct AskCard: View {
    let glyph: String
    let title: String
    let detail: String
    let primary: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 14) {
                Image(systemName: glyph).font(.system(size: 16, weight: .bold))
                    .foregroundStyle(primary ? Pulse.onAccent : Pulse.text)
                    .frame(width: 36, height: 36)
                    .background(RoundedRectangle(cornerRadius: 10, style: .continuous).fill(primary ? Pulse.accent : Color.white.opacity(0.08)))
                VStack(alignment: .leading, spacing: 2) {
                    Text(title).font(.pulseBody(15, weight: .semibold)).foregroundStyle(Pulse.text)
                    Text(detail).font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)
                }
                Spacer()
            }
            .padding(.horizontal, 14).padding(.vertical, 12)
            .background(RoundedRectangle(cornerRadius: 16, style: .continuous).fill(primary ? Pulse.accentPill : Color.white.opacity(0.05)))
            .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous)
                .strokeBorder(primary ? Pulse.accent.opacity(0.3) : Pulse.hairline))
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
    }
}

private struct ProgressDotsPhone: View {
    var body: some View {
        TimelineView(.animation) { tl in
            let t = tl.date.timeIntervalSinceReferenceDate
            HStack(spacing: 8) {
                ForEach(0..<3) { i in
                    Circle().fill(Pulse.accent).frame(width: 10, height: 10)
                        .opacity(0.3 + 0.7 * abs(sin(t * 3 + Double(i) * 0.6)))
                }
            }
        }
        .accessibilityLabel("Working")
    }
}
