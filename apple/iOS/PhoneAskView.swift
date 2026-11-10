import SwiftUI

struct PhoneAskView: View {
    @Environment(AskModel.self) private var ask
    @Environment(AppRouter.self) private var router
    @FocusState private var typing: Bool
    @State private var showField = false
    @State private var showSettings = false

    var body: some View {
        @Bindable var ask = ask
        ZStack {
            GlowBackdrop(primary: UnitPoint(x: 0.5, y: 0.62), secondary: UnitPoint(x: 0, y: 0), strength: 1.0)

            VStack(alignment: .leading, spacing: 18) {
                HStack {
                    IconButton(systemName: "xmark", label: "Close", glyph: 16, kind: .chip) { close() }
                    Spacer()
                    HStack(spacing: 8) {
                        SparkleIcon(size: 16)
                        Text("Ask Pulse").font(.pulseBody(16, weight: .semibold)).foregroundStyle(Pulse.text)
                    }
                    Spacer()
                    IconButton(systemName: "slider.horizontal.3", label: "Assistant settings", glyph: 16, kind: .chip) {
                        ask.stopListening(submit: false)
                        showSettings = true
                    }
                }

                if showField {
                    TextField("", text: $ask.query, prompt: Text("Ask for music").foregroundStyle(Pulse.textTertiary), axis: .vertical)
                        .textFieldStyle(.plain)
                        .font(PulseFont.display(30))
                        .tracking(-0.8)
                        .foregroundStyle(Pulse.text)
                        .focused($typing)
                        .submitLabel(.go)
                        .onSubmit { ask.submit(); typing = false }
                } else {
                    transcript
                }

                ScrollView {
                    VStack(alignment: .leading, spacing: 14) { content }
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .scrollIndicators(.never)

                if ask.phase == .idle || ask.phase == .listening {
                    micControl
                }
            }
            .padding(.horizontal, 22)
            .padding(.bottom, 16)
            .animation(.spring(response: 0.4, dampingFraction: 0.85), value: ask.phase)
        }
        .sheet(isPresented: $showSettings) {
            PhoneAssistantSettingsView()
                .presentationDetents([.large])
                .presentationBackground(Pulse.background)
        }
    }

    @ViewBuilder private var content: some View {
        switch ask.phase {
        case .thinking:
            HStack(spacing: 12) {
                NowPlayingBars()
                Text("Pulse is thinking it through…").font(.pulseBody(15)).foregroundStyle(Pulse.textSecondary)
            }
            .padding(.top, 20)
        case .failed:
            VStack(alignment: .leading, spacing: 14) {
                HStack(alignment: .top, spacing: 10) {
                    Image(systemName: "exclamationmark.triangle.fill").foregroundStyle(Color(hex: 0xFF8A7A))
                    Text(ask.errorMessage ?? "Something went wrong.").font(.pulseBody(15)).foregroundStyle(Pulse.text)
                        .fixedSize(horizontal: false, vertical: true)
                }
                HStack(spacing: 10) {
                    Button { ask.submit() } label: {
                        Text("Try again").font(.pulseBody(15, weight: .semibold)).padding(.horizontal, 18).frame(height: 44)
                    }
                    .pulse(.primary)
                    Button { showSettings = true } label: {
                        Text("Assistant settings").font(.pulseBody(15, weight: .medium)).padding(.horizontal, 16).frame(height: 44)
                    }
                    .pulse(.outline)
                }
            }
            .padding(.top, 10)
        case .results:
            results
        default:
            EmptyView()
        }
    }

    @ViewBuilder private var results: some View {
        if let plan = ask.plan, !plan.answer.isEmpty {
            Text(plan.answer)
                .font(PulseFont.display(20, weight: .semibold))
                .foregroundStyle(Pulse.text)
                .lineSpacing(3)
                .fixedSize(horizontal: false, vertical: true)
        }
        if let outcome = ask.outcome {
            HStack(spacing: 8) {
                Image(systemName: "checkmark.circle.fill").foregroundStyle(Pulse.accent)
                Text(outcome).font(.pulseBody(15, weight: .medium)).foregroundStyle(Pulse.text)
            }
        }
        if !ask.actions.isEmpty {
            Text("PULSE CAN").sectionLabel().padding(.top, 4)
            ForEach(Array(ask.actions.enumerated()), id: \.element.id) { i, action in
                AskCard(glyph: action.glyph, title: action.title, detail: action.detail, primary: i == 0) {
                    Task { if await ask.run(i) { close() } }
                }
            }
        }
        if !ask.preview.isEmpty || ask.resolving {
            HStack {
                Text("PREVIEW").sectionLabel()
                Spacer()
                Text(ask.resolving ? "Finding them on Spotify…" : "\(ask.preview.count) songs")
                    .font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
            }
            .padding(.top, 6)
            ForEach(ask.preview) { t in
                HStack(spacing: 12) {
                    ArtworkView(artwork: t.artwork, cornerRadius: 8).frame(width: 40, height: 40)
                    VStack(alignment: .leading, spacing: 1) {
                        Text(t.title).font(.pulseBody(15, weight: .semibold)).foregroundStyle(Pulse.text).lineLimit(1)
                        Text(t.artist).font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary).lineLimit(1)
                    }
                }
            }
        }
        if !ask.backendLabel.isEmpty {
            Text("Answered by \(ask.backendLabel)").font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary).padding(.top, 6)
        }
    }

    private var micControl: some View {
        VStack(spacing: 14) {
            if ask.phase == .listening {
                VoiceLevelBars(level: ask.level).frame(height: 56)
            }
            HStack(spacing: 28) {
                IconButton(systemName: "keyboard", label: "Type instead", glyph: 18, kind: .chip) {
                    ask.stopListening(submit: false)
                    showField = true
                    typing = true
                }
                Button {
                    ask.phase == .listening ? ask.stopListening() : ask.startListening()
                    showField = false
                } label: {
                    Image(systemName: ask.phase == .listening ? "stop.fill" : "mic.fill")
                        .font(.system(size: 30, weight: .semibold))
                        .foregroundStyle(Pulse.onAccent)
                        .frame(width: 88, height: 88)
                        .background(Circle().fill(Pulse.accent))
                        .background(Circle().fill(Pulse.accent.opacity(0.18)).padding(-12))
                        .background(Circle().fill(Pulse.accent.opacity(0.07)).padding(-24))
                        .scaleEffect(ask.phase == .listening ? 1 + ask.level * 0.08 : 1)
                        .animation(.easeOut(duration: 0.1), value: ask.level)
                }
                .buttonStyle(PlainPressStyle())
                .accessibilityLabel(ask.phase == .listening ? "Stop and ask" : "Start listening")
                IconButton(systemName: "arrow.up", label: "Ask", glyph: 18, kind: ask.query.isEmpty ? .chip : .tinted) {
                    ask.phase == .listening ? ask.stopListening() : ask.submit()
                }
                .disabled(ask.query.trimmingCharacters(in: .whitespaces).isEmpty)
            }
            Text(ask.phase == .listening ? "Listening · tap the button when you’re done" : "Tap to speak")
                .font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)
        }
        .frame(maxWidth: .infinity)
        .padding(.top, 8)
    }

    private var transcript: some View {
        let words = ask.query.split(separator: " ").map(String.init)
        let head = words.dropLast().joined(separator: " ")
        let last = words.last ?? ""
        return Group {
            if ask.query.isEmpty {
                Text(ask.phase == .listening ? "Listening…" : "What do you want to hear?")
                    .foregroundStyle(Pulse.textDim)
            } else {
                (Text(head.isEmpty ? "" : head + " ").foregroundStyle(Pulse.text)
                    + Text(last).foregroundStyle(ask.phase == .listening ? Pulse.accent : Pulse.text))
            }
        }
        .font(PulseFont.display(34))
        .tracking(-0.9)
        .frame(maxWidth: .infinity, minHeight: 80, alignment: .topLeading)
        .accessibilityLabel(ask.query.isEmpty ? "Listening" : ask.query)
    }

    private func close() {
        ask.reset()
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
                    Text(title).font(.pulseBody(15, weight: .semibold)).foregroundStyle(Pulse.text).multilineTextAlignment(.leading)
                    Text(detail).font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary).multilineTextAlignment(.leading)
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
