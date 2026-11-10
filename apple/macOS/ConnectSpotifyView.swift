import AppKit
import SwiftUI

/// First-run connection flow, in the Pulse style.
struct ConnectSpotifyView: View {
    @Environment(SpotifySession.self) private var session
    @State private var clientID = ""
    @State private var copied = false
    @FocusState private var fieldFocused: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 22) {
            HStack(spacing: 14) {
                Image(systemName: "waveform")
                    .font(.system(size: 22, weight: .bold))
                    .foregroundStyle(Pulse.onAccent)
                    .frame(width: 48, height: 48)
                    .background(RoundedRectangle(cornerRadius: 13, style: .continuous).fill(Pulse.accent))
                    .shadow(color: Pulse.accent.opacity(0.4), radius: 18)
                VStack(alignment: .leading, spacing: 2) {
                    Text("Connect Spotify")
                        .font(PulseFont.display(30)).tracking(-0.9)
                        .foregroundStyle(Pulse.text)
                    Text("Play your own library, in full, through AudioPulse.")
                        .font(.pulseBody(14)).foregroundStyle(Pulse.textSecondary)
                }
            }

            steps

            Group {
                switch session.status {
                case .needsClientID: clientIDStep
                case .signedOut: signInStep
                case .failed(let message): failedStep(message)
                default: progressStep
                }
            }
            .transition(.opacity)

            HStack {
                Text("Spotify Premium is needed for playback.")
                    .font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
                Spacer()
                Button("Use the sample library") { session.useSampleLibrary() }
                    .font(.pulseBody(13, weight: .medium))
                    .foregroundStyle(Pulse.textSecondary)
                    .buttonStyle(PlainPressStyle())
            }
        }
        .padding(30)
        .frame(width: 600)
        .background(RoundedRectangle(cornerRadius: 24, style: .continuous).fill(Color(hex: 0x0E0F0E).opacity(0.98)))
        .overlay(RoundedRectangle(cornerRadius: 24, style: .continuous).strokeBorder(Pulse.hairlineStrong))
        .shadow(color: .black.opacity(0.6), radius: 60, y: 30)
        .shadow(color: Pulse.accent.opacity(0.10), radius: 90)
        .animation(.easeOut(duration: 0.2), value: session.status)
    }

    // MARK: Steps indicator

    private var stepIndex: Int {
        switch session.status {
        case .needsClientID: 0
        case .signedOut, .signingIn: 1
        case .loadingLibrary: 2
        case .authorizingDevice, .startingDevice: 3
        case .ready: 4
        case .failed: SpotifyConfig.clientID == nil ? 0 : 1
        }
    }

    private var steps: some View {
        HStack(spacing: 8) {
            ForEach(Array(["Client ID", "Sign in", "Library", "Device"].enumerated()), id: \.offset) { i, title in
                HStack(spacing: 8) {
                    ZStack {
                        Circle().fill(i < stepIndex ? Pulse.accent : (i == stepIndex ? Pulse.accentPill : Pulse.fill))
                        if i < stepIndex {
                            Image(systemName: "checkmark").font(.system(size: 10, weight: .heavy)).foregroundStyle(Pulse.onAccent)
                        } else {
                            Text("\(i + 1)").font(.system(size: 11, weight: .bold))
                                .foregroundStyle(i == stepIndex ? Pulse.accent : Pulse.textTertiary)
                        }
                    }
                    .frame(width: 22, height: 22)
                    Text(title).font(.pulseBody(12, weight: .medium))
                        .foregroundStyle(i <= stepIndex ? Pulse.text : Pulse.textTertiary)
                        .lineLimit(1).fixedSize()
                }
                if i < 3 { Capsule().fill(i < stepIndex ? Pulse.accent.opacity(0.5) : Pulse.hairlineStrong).frame(height: 2) }
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("Step \(min(stepIndex + 1, 4)) of 4")
    }

    // MARK: States

    private var clientIDStep: some View {
        VStack(alignment: .leading, spacing: 14) {
            instruction(1, "Open the Spotify Developer Dashboard and create an app.") {
                Button { NSWorkspace.shared.open(URL(string: "https://developer.spotify.com/dashboard")!) } label: {
                    Label("Open dashboard", systemImage: "arrow.up.right").font(.pulseBody(13, weight: .medium))
                        .padding(.horizontal, 12).frame(height: 32)
                }
                .pulse(.outline)
            }
            instruction(2, "Add this Redirect URI exactly:") {
                Button {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(SpotifyConfig.redirectURI, forType: .string)
                    copied = true
                } label: {
                    HStack(spacing: 8) {
                        Text(SpotifyConfig.redirectURI).font(.system(size: 12, design: .monospaced))
                            .lineLimit(1).fixedSize()
                        Image(systemName: copied ? "checkmark" : "doc.on.doc").font(.system(size: 11, weight: .semibold))
                            .foregroundStyle(copied ? Pulse.accent : Pulse.textSecondary)
                    }
                    .padding(.horizontal, 12).frame(height: 32)
                }
                .pulse(.chip, shape: .rounded(8))
                .accessibilityLabel("Copy redirect URI")
            }
            instruction(3, "Paste the app’s Client ID (no secret needed):") { EmptyView() }
            HStack(spacing: 10) {
                TextField("", text: $clientID, prompt: Text("Client ID").foregroundStyle(Pulse.textTertiary))
                    .textFieldStyle(.plain)
                    .font(.system(size: 14, design: .monospaced))
                    .foregroundStyle(Pulse.text)
                    .focused($fieldFocused)
                    .onSubmit { session.saveClientID(clientID) }
                    .padding(.horizontal, 14).frame(height: 44)
                    .background(RoundedRectangle(cornerRadius: 12, style: .continuous).fill(Pulse.fill))
                    .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous)
                        .strokeBorder(fieldFocused ? Pulse.accent.opacity(0.5) : Pulse.hairlineStrong))
                    .accessibilityLabel("Spotify Client ID")
                Button { session.saveClientID(clientID) } label: {
                    Text("Save").font(.pulseBody(14, weight: .semibold)).padding(.horizontal, 18).frame(height: 44)
                }
                .pulse(.primary, shape: .rounded(12))
                .disabled(clientID.trimmingCharacters(in: .whitespaces).isEmpty)
            }
            Text("Saved to ~/Library/Application Support/audiopulse/config.json — the terminal app uses it too.")
                .font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
        }
        .onAppear { fieldFocused = true }
    }

    private var signInStep: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("Your browser will open to sign in to Spotify and approve AudioPulse. Then come back here.")
                .font(.pulseBody(14)).foregroundStyle(Pulse.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
            Button { session.signIn() } label: {
                HStack(spacing: 10) {
                    Image(systemName: "person.crop.circle.badge.checkmark").font(.system(size: 16, weight: .semibold))
                    Text("Connect Spotify").font(.pulseBody(15, weight: .semibold))
                }
                .padding(.horizontal, 22).frame(height: 48)
            }
            .pulse(.primary)
            .shadow(color: Pulse.accent.opacity(0.35), radius: 18)
        }
    }

    private var progressStep: some View {
        HStack(spacing: 14) {
            ProgressDots()
            VStack(alignment: .leading, spacing: 3) {
                Text(progressTitle).font(.pulseBody(15, weight: .semibold)).foregroundStyle(Pulse.text)
                Text(progressDetail).font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: 14, style: .continuous).fill(Pulse.accent.opacity(0.06)))
        .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).strokeBorder(Pulse.accent.opacity(0.2)))
    }

    private var progressTitle: String {
        switch session.status {
        case .signingIn: "Finish signing in in your browser"
        case .loadingLibrary: "Loading your library"
        case .authorizingDevice(let browser): browser ? "Approve the AudioPulse device in your browser" : "Authorizing the AudioPulse device"
        case .startingDevice: "Starting the AudioPulse device"
        default: "Connecting"
        }
    }

    private var progressDetail: String {
        switch session.status {
        case .signingIn: "Waiting for Spotify to send you back here."
        case .loadingLibrary: "Playlists, Liked Songs and what you played recently."
        case .authorizingDevice: "Spotify asks once more, for the librespot player itself. One-time, and shared with the terminal app."
        case .startingDevice: "librespot is connecting as a Spotify Connect device."
        default: ""
        }
    }

    private func failedStep(_ message: String) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 10) {
                Image(systemName: "exclamationmark.triangle.fill").foregroundStyle(Color(hex: 0xFF8A7A))
                Text(message).font(.pulseBody(14)).foregroundStyle(Pulse.text)
                    .fixedSize(horizontal: false, vertical: true)
                    .textSelection(.enabled)
            }
            HStack(spacing: 10) {
                Button { session.retry() } label: {
                    Text("Try again").font(.pulseBody(14, weight: .semibold)).padding(.horizontal, 18).frame(height: 40)
                }
                .pulse(.primary)
                if SpotifyConfig.clientID != nil {
                    Button { session.signIn() } label: {
                        Text("Sign in again").font(.pulseBody(14, weight: .medium)).padding(.horizontal, 16).frame(height: 40)
                    }
                    .pulse(.outline)
                }
            }
        }
    }

    private func instruction<Trailing: View>(_ n: Int, _ text: String, @ViewBuilder trailing: () -> Trailing) -> some View {
        HStack(spacing: 12) {
            Text("\(n)").font(.system(size: 12, weight: .bold)).foregroundStyle(Pulse.accent)
                .frame(width: 24, height: 24).background(Circle().fill(Pulse.accentPill))
            Text(text).font(.pulseBody(14)).foregroundStyle(Pulse.text)
                .fixedSize(horizontal: false, vertical: true)
            Spacer(minLength: 8)
            trailing()
        }
    }
}
