import SwiftUI
import VisionKit

/// Where Ask Pulse runs on this iPhone: Fairy on your Mac (paired over
/// Tailscale) and/or Ollama on your PC.
struct PhoneAssistantSettingsView: View {
    @Environment(AskModel.self) private var ask
    @Environment(\.dismiss) private var dismiss
    @State private var link = ""
    @State private var pairing = false
    @State private var pairError: String?
    @State private var scanning = false

    var body: some View {
        let settings = ask.settings
        ScrollView {
            VStack(alignment: .leading, spacing: 22) {
                HStack {
                    Text("Assistant").font(PulseFont.display(34)).tracking(-1).foregroundStyle(Pulse.text)
                    Spacer()
                    IconButton(systemName: "xmark", label: "Done", glyph: 15, kind: .chip) { dismiss() }
                }

                VStack(alignment: .leading, spacing: 10) {
                    Text("USE").sectionLabel()
                    PulseSegmented(options: AssistantChoice.allCases.map { ($0, $0.title) },
                                   selection: Binding(get: { settings.choice }, set: { settings.choice = $0 }))
                    Text("Automatic tries Fairy first, then your PC.")
                        .font(.pulseBody(13)).foregroundStyle(Pulse.textTertiary)
                }

                card {
                    HStack(spacing: 12) {
                        Image(systemName: "sparkles").font(.system(size: 18, weight: .semibold)).foregroundStyle(Pulse.accent)
                        VStack(alignment: .leading, spacing: 2) {
                            Text("Fairy on your Mac").font(.pulseBody(16, weight: .semibold)).foregroundStyle(Pulse.text)
                            Text(settings.fairyPaired ? "Paired · runs on your Claude, Gemini or ChatGPT plan" : "Not paired")
                                .font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)
                        }
                        Spacer()
                        if settings.fairyPaired { UnreadDot() }
                    }
                    if settings.fairyPaired {
                        Button { settings.forgetFairy() } label: {
                            Text("Forget this pairing").font(.pulseBody(14, weight: .medium)).padding(.horizontal, 14).frame(height: 40)
                        }
                        .pulse(.outline)
                    } else {
                        Text("On your Mac, open Fairy → Settings → Phone, turn on the iPhone app, then Pair an iPhone. Both devices need Tailscale.")
                            .font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)
                            .fixedSize(horizontal: false, vertical: true)
                        HStack(spacing: 10) {
                            Button { scanning = true } label: {
                                Label("Scan the code", systemImage: "qrcode.viewfinder").font(.pulseBody(15, weight: .semibold))
                                    .frame(maxWidth: .infinity, minHeight: 46)
                            }
                            .pulse(.primary)
                            .disabled(!DataScannerViewController.isSupported)
                        }
                        field("or paste the fairy://pair link", text: $link)
                        if !link.isEmpty {
                            Button { pair(link) } label: {
                                Text(pairing ? "Pairing…" : "Pair").font(.pulseBody(15, weight: .semibold)).frame(maxWidth: .infinity, minHeight: 46)
                            }
                            .pulse(.tinted)
                            .disabled(pairing)
                        }
                        if let pairError {
                            Text(pairError).font(.pulseBody(13)).foregroundStyle(Color(hex: 0xFF8A7A))
                                .fixedSize(horizontal: false, vertical: true)
                        }
                    }
                }

                card {
                    HStack(spacing: 12) {
                        Image(systemName: "desktopcomputer").font(.system(size: 18, weight: .semibold)).foregroundStyle(Pulse.accent)
                        VStack(alignment: .leading, spacing: 2) {
                            Text("Ollama on your PC").font(.pulseBody(16, weight: .semibold)).foregroundStyle(Pulse.text)
                            Text("Uses your PC’s GPU when it’s on").font(.pulseBody(13)).foregroundStyle(Pulse.textSecondary)
                        }
                    }
                    field("http://your-pc.your-tailnet.ts.net:11434", text: Binding(get: { settings.ollamaURL }, set: { settings.ollamaURL = $0 }))
                    field("Model (blank picks Gemma)", text: Binding(get: { settings.ollamaModel }, set: { settings.ollamaModel = $0 }))
                    Text("Set OLLAMA_HOST=0.0.0.0 on the PC so Ollama listens on Tailscale.")
                        .font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
                }
            }
            .padding(22)
        }
        .scrollIndicators(.never)
        .background(Pulse.background.ignoresSafeArea())
        .fullScreenCover(isPresented: $scanning) {
            PairingScanner { code in
                scanning = false
                pair(code)
            } cancel: { scanning = false }
            .ignoresSafeArea()
        }
    }

    private func pair(_ code: String) {
        pairing = true
        pairError = nil
        Task {
            do {
                try await ask.settings.pairFairy(link: code)
                link = ""
            } catch {
                pairError = error.localizedDescription
            }
            pairing = false
        }
    }

    private func card<C: View>(@ViewBuilder _ content: () -> C) -> some View {
        VStack(alignment: .leading, spacing: 14) { content() }
            .padding(16)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(RoundedRectangle(cornerRadius: 18, style: .continuous).fill(Pulse.panel))
            .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).strokeBorder(Pulse.hairline))
    }

    private func field(_ prompt: String, text: Binding<String>) -> some View {
        TextField("", text: text, prompt: Text(prompt).foregroundStyle(Pulse.textTertiary))
            .textFieldStyle(.plain)
            .font(.system(size: 14, design: .monospaced))
            .foregroundStyle(Pulse.text)
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            .keyboardType(.URL)
            .padding(.horizontal, 14).frame(height: 46)
            .background(RoundedRectangle(cornerRadius: 12, style: .continuous).fill(Pulse.fill))
            .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).strokeBorder(Pulse.hairlineStrong))
    }
}

/// Camera QR scanner for Fairy's pairing code.
struct PairingScanner: UIViewControllerRepresentable {
    let found: (String) -> Void
    let cancel: () -> Void

    func makeUIViewController(context: Context) -> UINavigationController {
        let host = ScannerHost(delegate: context.coordinator, cancel: cancel)
        let nav = UINavigationController(rootViewController: host)
        nav.overrideUserInterfaceStyle = .dark
        return nav
    }

    func updateUIViewController(_ controller: UINavigationController, context: Context) {}

    /// Holds the scanner and starts it only once it's on screen: VisionKit's
    /// scanner shows the camera but recognises nothing if started earlier.
    final class ScannerHost: UIViewController {
        private let scanner = DataScannerViewController(recognizedDataTypes: [.barcode(symbologies: [.qr])],
                                                        qualityLevel: .balanced, isHighlightingEnabled: true)
        private let note = UILabel()

        init(delegate: DataScannerViewControllerDelegate, cancel: @escaping () -> Void) {
            super.init(nibName: nil, bundle: nil)
            scanner.delegate = delegate
            navigationItem.title = "Scan Fairy’s pairing code"
            navigationItem.leftBarButtonItem = UIBarButtonItem(systemItem: .cancel, primaryAction: UIAction { _ in cancel() })
        }
        required init?(coder: NSCoder) { fatalError() }

        override func viewDidLoad() {
            super.viewDidLoad()
            view.backgroundColor = .black
            addChild(scanner)
            scanner.view.frame = view.bounds
            scanner.view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
            view.addSubview(scanner.view)
            scanner.didMove(toParent: self)
            note.textColor = .white
            note.numberOfLines = 0
            note.textAlignment = .center
            note.font = .systemFont(ofSize: 15, weight: .medium)
            note.translatesAutoresizingMaskIntoConstraints = false
            view.addSubview(note)
            NSLayoutConstraint.activate([
                note.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 24),
                note.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -24),
                note.bottomAnchor.constraint(equalTo: view.safeAreaLayoutGuide.bottomAnchor, constant: -24),
            ])
        }

        override func viewDidAppear(_ animated: Bool) {
            super.viewDidAppear(animated)
            guard DataScannerViewController.isAvailable else {
                note.text = "The camera isn’t available. Allow AudioPulse to use it in Settings, or paste the pairing link instead."
                return
            }
            do {
                try scanner.startScanning()
                note.text = "Point at the code in Fairy → Settings → Phone."
            } catch {
                note.text = "The scanner couldn’t start: \(error.localizedDescription)"
            }
        }

        override func viewWillDisappear(_ animated: Bool) {
            super.viewWillDisappear(animated)
            scanner.stopScanning()
        }
    }

    func makeCoordinator() -> Coordinator { Coordinator(found: found) }

    final class Coordinator: NSObject, DataScannerViewControllerDelegate {
        let found: (String) -> Void
        private var done = false
        init(found: @escaping (String) -> Void) { self.found = found }

        func dataScanner(_ scanner: DataScannerViewController, didAdd items: [RecognizedItem], allItems: [RecognizedItem]) {
            for item in items {
                if case .barcode(let code) = item, let value = code.payloadStringValue, value.hasPrefix("fairy://pair"), !done {
                    done = true
                    scanner.stopScanning()
                    found(value)
                }
            }
        }
    }
}
