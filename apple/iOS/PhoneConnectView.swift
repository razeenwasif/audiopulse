import SwiftUI

/// iPhone first-run Spotify connection, in the Pulse style.
struct PhoneConnectView: View {
    @Environment(PhoneSpotifySession.self) private var session
    @State private var clientID = ""
    @FocusState private var focused: Bool

    var body: some View {
        ZStack {
            Pulse.background.ignoresSafeArea()
            GlowBackdrop(primary: UnitPoint(x: 0.5, y: 0.1), secondary: UnitPoint(x: 0, y: 1), strength: 0.9)

            VStack(alignment: .leading, spacing: 22) {
                Spacer(minLength: 0)
                Image(systemName: "waveform")
                    .font(.system(size: 28, weight: .bold))
                    .foregroundStyle(Pulse.onAccent)
                    .frame(width: 64, height: 64)
                    .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Pulse.accent))
                    .shadow(color: Pulse.accent.opacity(0.45), radius: 24)
                VStack(alignment: .leading, spacing: 8) {
                    Text("Connect Spotify")
                        .font(PulseFont.display(40)).tracking(-1.2)
                        .foregroundStyle(Pulse.text)
                    Text("Your library in AudioPulse. Music plays through the Spotify app on this iPhone.")
                        .font(.pulseBody(16)).foregroundStyle(Pulse.textSecondary)
                        .fixedSize(horizontal: false, vertical: true)
                }

                Group {
                    switch session.status {
                    case .needsClientID: clientIDStep
                    case .failed(let message): failed(message)
                    case .signingIn, .loadingLibrary: progress
                    default: signIn
                    }
                }
                .animation(.easeOut(duration: 0.2), value: session.status)

                Spacer(minLength: 0)

                VStack(alignment: .leading, spacing: 10) {
                    Text("Needs Spotify Premium and the Spotify app. In your Spotify developer app, add the Redirect URI audiopulse://spotify-callback and the iOS bundle ID dev.audiopulse.ios.")
                        .font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
                        .fixedSize(horizontal: false, vertical: true)
                    Button("Use the sample library") { session.useSampleLibrary() }
                        .font(.pulseBody(14, weight: .medium))
                        .foregroundStyle(Pulse.textSecondary)
                        .buttonStyle(PlainPressStyle())
                        .frame(minHeight: 44)
                }
            }
            .padding(.horizontal, 26)
            .padding(.vertical, 20)
        }
    }

    private var signIn: some View {
        Button { session.signIn() } label: {
            HStack(spacing: 10) {
                Image(systemName: "person.crop.circle.badge.checkmark").font(.system(size: 17, weight: .semibold))
                Text("Connect Spotify").font(.pulseBody(17, weight: .semibold))
            }
            .frame(maxWidth: .infinity, minHeight: 56)
        }
        .pulse(.primary)
        .shadow(color: Pulse.accent.opacity(0.35), radius: 20)
    }

    private var progress: some View {
        HStack(spacing: 14) {
            NowPlayingBars()
            VStack(alignment: .leading, spacing: 3) {
                Text(session.status == .signingIn ? "Signing in to Spotify" : "Loading your library")
                    .font(.pulseBody(16, weight: .semibold)).foregroundStyle(Pulse.text)
                Text(session.status == .signingIn ? "Finish in the sheet that opened." : "Playlists, Liked Songs and recent plays.")
                    .font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: 16, style: .continuous).fill(Pulse.accent.opacity(0.06)))
        .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous).strokeBorder(Pulse.accent.opacity(0.2)))
    }

    private var clientIDStep: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Paste your Spotify app’s Client ID")
                .font(.pulseBody(15, weight: .semibold)).foregroundStyle(Pulse.text)
            TextField("", text: $clientID, prompt: Text("Client ID").foregroundStyle(Pulse.textTertiary))
                .textFieldStyle(.plain)
                .font(.system(size: 15, design: .monospaced))
                .foregroundStyle(Pulse.text)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .focused($focused)
                .submitLabel(.done)
                .onSubmit { session.saveClientID(clientID) }
                .padding(.horizontal, 16).frame(height: 52)
                .background(RoundedRectangle(cornerRadius: 14, style: .continuous).fill(Pulse.fill))
                .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .strokeBorder(focused ? Pulse.accent.opacity(0.5) : Pulse.hairlineStrong))
                .accessibilityLabel("Spotify Client ID")
            Button { session.saveClientID(clientID) } label: {
                Text("Save").font(.pulseBody(16, weight: .semibold)).frame(maxWidth: .infinity, minHeight: 52)
            }
            .pulse(.primary)
            .disabled(clientID.trimmingCharacters(in: .whitespaces).isEmpty)
        }
    }

    private func failed(_ message: String) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 10) {
                Image(systemName: "exclamationmark.triangle.fill").foregroundStyle(Color(hex: 0xFF8A7A))
                Text(message).font(.pulseBody(15)).foregroundStyle(Pulse.text)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Button { session.retry() } label: {
                Text("Try again").font(.pulseBody(16, weight: .semibold)).frame(maxWidth: .infinity, minHeight: 52)
            }
            .pulse(.primary)
        }
    }
}
