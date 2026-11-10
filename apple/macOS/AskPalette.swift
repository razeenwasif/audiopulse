import SwiftUI

struct AskPalette: View {
    @Environment(AskModel.self) private var ask
    @Environment(AppRouter.self) private var router
    @FocusState private var fieldFocused: Bool

    var body: some View {
        @Bindable var ask = ask
        VStack(spacing: 0) {
            HStack(spacing: 12) {
                SparkleIcon(size: 20)
                TextField("", text: $ask.query, prompt: Text("Ask for music: “rainy day indie, nothing too loud”").foregroundStyle(Pulse.textTertiary))
                    .textFieldStyle(.plain)
                    .font(PulseFont.display(22, weight: .semibold))
                    .tracking(-0.2)
                    .foregroundStyle(Pulse.text)
                    .focused($fieldFocused)
                    .onSubmit { submitOrRun() }
                    .onKeyPress(.downArrow) { ask.moveHighlight(1); return .handled }
                    .onKeyPress(.upArrow) { ask.moveHighlight(-1); return .handled }
                    .onKeyPress(.escape) { router.closeAsk(); return .handled }
                    .onChange(of: ask.query) { _, _ in
                        if ask.phase == .results || ask.phase == .failed { ask.phase = .idle }
                    }
                    .accessibilityLabel("Ask Pulse")
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

            Group {
                switch ask.phase {
                case .listening:
                    VoiceLevelBars(level: ask.level).frame(height: 56).padding(.horizontal, 24).padding(.vertical, 28)
                case .thinking:
                    status(dots: true, "Pulse is thinking it through…")
                case .failed:
                    failure
                case .results:
                    results
                case .idle:
                    suggestionsForEmpty
                }
            }

            HStack {
                HStack(spacing: 8) {
                    Circle().fill(Pulse.accent).frame(width: 7, height: 7)
                    Text(footerText)
                }
                Spacer()
                Text("↑↓ move · ↵ run · esc close")
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

    private var footerText: String {
        if !ask.backendLabel.isEmpty, ask.phase == .results { return "Answered by \(ask.backendLabel)" }
        return "Fairy on this Mac, or your PC’s Ollama · \(ask.settings.choice.title)"
    }

    private func submitOrRun() {
        if ask.phase == .results, !ask.actions.isEmpty {
            Task { if await ask.run() { router.closeAsk() } }
        } else {
            ask.submit()
        }
    }

    private func status(dots: Bool, _ text: String) -> some View {
        HStack(spacing: 10) {
            if dots { ProgressDots() }
            Text(text).font(.pulseBody(14)).foregroundStyle(Pulse.textSecondary)
        }
        .frame(maxWidth: .infinity, minHeight: 110)
    }

    private var suggestionsForEmpty: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("TRY").sectionLabel().padding(.horizontal, 8).padding(.top, 12)
            ForEach(["Make a playlist of rainy day indie, nothing too loud", "Something like this, but slower",
                     "Smart shuffle this playlist", "What have I been into lately?"], id: \.self) { example in
                Button {
                    ask.query = example
                    ask.submit()
                } label: {
                    HStack(spacing: 12) {
                        Image(systemName: "sparkles").font(.system(size: 13, weight: .semibold)).foregroundStyle(Pulse.accent)
                        Text(example).font(.pulseBody(14)).foregroundStyle(Pulse.text)
                        Spacer()
                    }
                    .padding(.horizontal, 12).frame(height: 40)
                    .contentShape(Rectangle())
                }
                .pulse(.ghost, shape: .rounded(10))
            }
        }
        .padding(.horizontal, 12)
        .padding(.bottom, 10)
    }

    private var failure: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .top, spacing: 10) {
                Image(systemName: "exclamationmark.triangle.fill").foregroundStyle(Color(hex: 0xFF8A7A))
                Text(ask.errorMessage ?? "Something went wrong.").font(.pulseBody(14)).foregroundStyle(Pulse.text)
                    .fixedSize(horizontal: false, vertical: true)
            }
            HStack(spacing: 8) {
                Button { ask.submit() } label: {
                    Text("Try again").font(.pulseBody(13, weight: .semibold)).padding(.horizontal, 14).frame(height: 34)
                }
                .pulse(.primary)
                PulseSegmented(options: AssistantChoice.allCases.map { ($0, $0.title) }, selection: Binding(
                    get: { ask.settings.choice }, set: { ask.settings.choice = $0 }))
            }
        }
        .padding(20)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var results: some View {
        VStack(alignment: .leading, spacing: 4) {
            if let plan = ask.plan, !plan.answer.isEmpty {
                Text(plan.answer)
                    .font(PulseFont.display(19, weight: .semibold))
                    .foregroundStyle(Pulse.text)
                    .lineSpacing(3)
                    .fixedSize(horizontal: false, vertical: true)
                    .textSelection(.enabled)
                    .padding(.horizontal, 8).padding(.vertical, 12)
            }
            if let outcome = ask.outcome {
                HStack(spacing: 8) {
                    Image(systemName: "checkmark.circle.fill").foregroundStyle(Pulse.accent)
                    Text(outcome).font(.pulseBody(14, weight: .medium)).foregroundStyle(Pulse.text)
                }
                .padding(.horizontal, 8).padding(.vertical, 10)
            }
            if !ask.actions.isEmpty {
                Text("PULSE CAN").sectionLabel().padding(.horizontal, 8).padding(.top, 8).padding(.bottom, 4)
                ForEach(Array(ask.actions.enumerated()), id: \.element.id) { i, action in
                    AskActionRow(action: action, highlighted: ask.highlighted == i) {
                        ask.highlighted = i
                    } run: {
                        ask.highlighted = i
                        Task { if await ask.run(i) { router.closeAsk() } }
                    }
                }
            }
            if !ask.preview.isEmpty || ask.resolving {
                HStack(alignment: .firstTextBaseline) {
                    Text(ask.plan?.name.isEmpty == false ? "PREVIEW · \(ask.plan!.name.uppercased())" : "PREVIEW").sectionLabel()
                    Spacer()
                    if ask.resolving {
                        Text("Finding them on Spotify…").font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
                    } else {
                        Text("\(ask.preview.count) songs").font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
                    }
                }
                .padding(.horizontal, 8)
                .padding(.top, 14)
                .padding(.bottom, 4)
                ScrollView {
                    VStack(spacing: 0) {
                        ForEach(ask.preview) { t in
                            HStack(spacing: 12) {
                                ArtworkView(artwork: t.artwork, cornerRadius: 6).frame(width: 32, height: 32)
                                Text(t.title).font(.pulseBody(14, weight: .medium)).foregroundStyle(Pulse.text).lineLimit(1)
                                Text(t.artist).font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary).lineLimit(1)
                                Spacer()
                                if t.duration > 0 {
                                    Text(t.duration.clock).font(.pulseBody(12)).monospacedDigit().foregroundStyle(Pulse.textSecondary)
                                }
                            }
                            .padding(.horizontal, 12)
                            .frame(height: 44)
                        }
                    }
                }
                .frame(maxHeight: 240)
                .scrollIndicators(.never)
            }
        }
        .padding(.horizontal, 12)
        .padding(.bottom, 10)
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
