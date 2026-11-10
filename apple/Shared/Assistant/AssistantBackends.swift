import CryptoKit
import Foundation

/// Somewhere AudioPulse's assistant can run: it takes instructions and a request
/// and returns the model's text (JSON for the plan).
protocol AssistantBackend: Sendable {
    /// Shown to the user, e.g. "Fairy (Claude)" or "Ollama on your PC".
    var label: String { get }
    func complete(instructions: String, prompt: String) async throws -> String
}

enum AssistantError: LocalizedError {
    case unavailable(String)
    case failed(String)
    case unreadable

    var errorDescription: String? {
        switch self {
        case .unavailable(let why): why
        case .failed(let why): why
        case .unreadable: "The assistant’s answer couldn’t be read. Try again."
        }
    }
}

// MARK: - Ollama (e.g. the PC with the GPU, over Tailscale)

/// Ollama's chat API in JSON mode. Works with a local Ollama or one on another
/// machine in your tailnet (http://pc-name.your-tailnet.ts.net:11434).
struct OllamaBackend: AssistantBackend {
    let baseURL: URL
    let model: String

    var label: String { "Ollama (\(baseURL.host() ?? "PC"))" }

    func complete(instructions: String, prompt: String) async throws -> String {
        let model = try await resolvedModel()
        var req = URLRequest(url: baseURL.appendingPathComponent("api/chat"))
        req.httpMethod = "POST"
        req.timeoutInterval = 120
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONSerialization.data(withJSONObject: [
            "model": model, "stream": false, "format": "json",
            "messages": [["role": "system", "content": instructions], ["role": "user", "content": prompt]],
            "options": ["temperature": 0.5],
        ])
        let data: Data, resp: URLResponse
        do { (data, resp) = try await URLSession.shared.data(for: req) } catch {
            throw AssistantError.unavailable("Ollama at \(baseURL.host() ?? "your PC") isn’t reachable. Is the PC on and Tailscale connected?")
        }
        guard (resp as? HTTPURLResponse)?.statusCode == 200 else {
            throw AssistantError.failed("Ollama returned \((resp as? HTTPURLResponse)?.statusCode ?? 0).")
        }
        struct Body: Decodable { struct M: Decodable { let content: String }; let message: M }
        return try JSONDecoder().decode(Body.self, from: data).message.content
    }

    /// The configured model, else a Gemma-like chat model, as the TUI picks it.
    private func resolvedModel() async throws -> String {
        if !model.isEmpty { return model }
        var req = URLRequest(url: baseURL.appendingPathComponent("api/tags"))
        req.timeoutInterval = 6
        guard let (data, _) = try? await URLSession.shared.data(for: req) else {
            throw AssistantError.unavailable("Ollama at \(baseURL.host() ?? "your PC") isn’t reachable. Is the PC on and Tailscale connected?")
        }
        struct Tags: Decodable { struct M: Decodable { let name: String }; let models: [M] }
        let names = (try? JSONDecoder().decode(Tags.self, from: data))?.models.map(\.name) ?? []
        let chat = names.filter { !$0.lowercased().contains("embed") }
        guard let pick = chat.first(where: { $0.lowercased().contains("gemma") }) ?? chat.first else {
            throw AssistantError.unavailable("Ollama has no chat model installed.")
        }
        return pick
    }
}

// MARK: - Fairy on this Mac (private socket)

#if os(macOS)
/// Fairy's AudioPulse link: JSON lines over `audiopulse.sock` in her data folder.
struct FairySocketBackend: AssistantBackend {
    static var socketPath: String {
        if let dir = ProcessInfo.processInfo.environment["LYRA_DATA_DIR"], !dir.isEmpty {
            return (dir as NSString).appendingPathComponent("audiopulse.sock")
        }
        return FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Application Support/lyra-companion/lyra/audiopulse.sock").path
    }

    static var isAvailable: Bool { FileManager.default.fileExists(atPath: socketPath) }

    let providerName: String
    var label: String { providerName.isEmpty ? "Fairy" : "Fairy (\(providerName))" }

    /// Asks Fairy which provider she'll use. Nil if she isn't listening.
    static func hello() async -> FairySocketBackend? {
        guard isAvailable, let reply = try? await exchange(["type": "hello"], timeout: 3),
              reply["type"] as? String == "hello" else { return nil }
        return FairySocketBackend(providerName: reply["provider"] as? String ?? "")
    }

    func complete(instructions: String, prompt: String) async throws -> String {
        let id = UUID().uuidString
        let reply = try await Self.exchange(["type": "ask", "id": id, "instructions": instructions, "prompt": prompt], timeout: 100)
        if reply["type"] as? String == "done", let text = reply["text"] as? String { return text }
        throw AssistantError.failed(reply["message"] as? String ?? "Fairy couldn’t answer.")
    }

    /// Sends one JSON line and reads one JSON line back, on a background thread.
    private static func exchange(_ message: [String: Any], timeout: Int) async throws -> [String: Any] {
        let payload = try JSONSerialization.data(withJSONObject: message) + Data("\n".utf8)
        let path = socketPath
        return try await Task.detached(priority: .userInitiated) {
            let fd = socket(AF_UNIX, SOCK_STREAM, 0)
            guard fd >= 0 else { throw AssistantError.unavailable("Couldn’t open a socket.") }
            defer { close(fd) }
            var tv = timeval(tv_sec: timeout, tv_usec: 0)
            setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &tv, socklen_t(MemoryLayout<timeval>.size))
            var addr = sockaddr_un()
            addr.sun_family = sa_family_t(AF_UNIX)
            let bytes = Array(path.utf8CString)
            guard bytes.count <= MemoryLayout.size(ofValue: addr.sun_path) else { throw AssistantError.unavailable("Fairy’s socket path is too long.") }
            withUnsafeMutableBytes(of: &addr.sun_path) { raw in
                bytes.withUnsafeBytes { raw.copyMemory(from: $0) }
            }
            let connected = withUnsafePointer(to: &addr) {
                $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
            }
            guard connected == 0 else { throw AssistantError.unavailable("Fairy isn’t running on this Mac.") }
            _ = payload.withUnsafeBytes { write(fd, $0.baseAddress, payload.count) }
            var buffer = Data()
            var chunk = [UInt8](repeating: 0, count: 65_536)
            while !buffer.contains(UInt8(ascii: "\n")) {
                let n = read(fd, &chunk, chunk.count)
                if n <= 0 { throw AssistantError.failed(n == 0 ? "Fairy closed the connection." : "Fairy took too long to answer.") }
                buffer.append(contentsOf: chunk[0..<n])
            }
            let line = buffer.prefix { $0 != UInt8(ascii: "\n") }
            guard let obj = try JSONSerialization.jsonObject(with: Data(line)) as? [String: Any] else { throw AssistantError.unreadable }
            return obj
        }.value
    }
}
#endif

// MARK: - Fairy from an iPhone (over Tailscale, pinned certificate)

/// What pairing with Fairy gives the phone: where she is, the certificate to
/// trust, and the read key. Stored in the Keychain.
struct FairyPairing: Codable, Sendable {
    let host: String
    let port: Int
    let pin: String
    let readKey: String

    static let keychainAccount = "fairy-pairing"

    static func load() -> FairyPairing? {
        guard let data = Keychain.read(account: keychainAccount) else { return nil }
        return try? JSONDecoder().decode(FairyPairing.self, from: data)
    }

    func save() { if let data = try? JSONEncoder().encode(self) { Keychain.write(data, account: Self.keychainAccount) } }
    static func forget() { Keychain.delete(account: keychainAccount) }

    /// Pairs using a `fairy://pair?host=…&port=…&code=…&pin=…` link from Fairy's Mac.
    static func pair(with link: String, deviceName: String) async throws -> FairyPairing {
        guard let comps = URLComponents(string: link.trimmingCharacters(in: .whitespacesAndNewlines)),
              comps.scheme == "fairy", comps.host == "pair",
              let host = comps.queryItems?.first(where: { $0.name == "host" })?.value,
              let port = comps.queryItems?.first(where: { $0.name == "port" })?.value.flatMap(Int.init),
              let code = comps.queryItems?.first(where: { $0.name == "code" })?.value,
              let pin = comps.queryItems?.first(where: { $0.name == "pin" })?.value
        else { throw AssistantError.failed("That isn’t a Fairy pairing code. On your Mac, open Fairy → Settings → Phone → Pair an iPhone.") }
        var req = URLRequest(url: URL(string: "https://\(host):\(port)/v1/pair")!)
        req.httpMethod = "POST"
        req.timeoutInterval = 15
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONSerialization.data(withJSONObject: ["code": code, "name": deviceName])
        let (data, resp) = try await PinnedSession.session(pin: pin).data(for: req)
        let status = (resp as? HTTPURLResponse)?.statusCode ?? 0
        struct Body: Decodable { let readKey: String?; let error: String? }
        let body = try? JSONDecoder().decode(Body.self, from: data)
        guard status == 200, let key = body?.readKey else {
            throw AssistantError.failed(body?.error ?? "Fairy refused the pairing (\(status)).")
        }
        let pairing = FairyPairing(host: host, port: port, pin: pin, readKey: key)
        pairing.save()
        return pairing
    }
}

/// Fairy's `/v1/assist` on the phone link.
struct FairyPhoneBackend: AssistantBackend {
    let pairing: FairyPairing
    var label: String { "Fairy on your Mac" }

    func complete(instructions: String, prompt: String) async throws -> String {
        var req = URLRequest(url: URL(string: "https://\(pairing.host):\(pairing.port)/v1/assist")!)
        req.httpMethod = "POST"
        req.timeoutInterval = 100
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.setValue("Bearer \(pairing.readKey)", forHTTPHeaderField: "Authorization")
        req.httpBody = try JSONSerialization.data(withJSONObject: ["instructions": instructions, "prompt": prompt])
        let data: Data, resp: URLResponse
        do { (data, resp) = try await PinnedSession.session(pin: pairing.pin).data(for: req) } catch {
            throw AssistantError.unavailable("Fairy isn’t reachable. Check your Mac is awake, Fairy is open, and Tailscale is on.")
        }
        let status = (resp as? HTTPURLResponse)?.statusCode ?? 0
        struct Body: Decodable { let text: String?; let error: String? }
        let body = try? JSONDecoder().decode(Body.self, from: data)
        if status == 200, let text = body?.text { return text }
        if status == 401 { throw AssistantError.unavailable("This iPhone isn’t paired with Fairy any more. Pair it again in Assistant settings.") }
        if status == 404 { throw AssistantError.unavailable("Fairy on your Mac needs updating to work with AudioPulse.") }
        throw AssistantError.failed(body?.error ?? "Fairy couldn’t answer (\(status)).")
    }
}

/// Trusts exactly one certificate, by its SHA-256 fingerprint from the pairing
/// code — the same check the Fairy iPhone app uses.
final class PinnedSession: NSObject, URLSessionTaskDelegate, @unchecked Sendable {
    let fingerprint: String
    private init(fingerprint: String) { self.fingerprint = fingerprint.lowercased() }

    static func session(pin: String) -> URLSession {
        URLSession(configuration: .ephemeral, delegate: PinnedSession(fingerprint: pin), delegateQueue: nil)
    }

    private func decide(_ challenge: URLAuthenticationChallenge) -> (URLSession.AuthChallengeDisposition, URLCredential?) {
        guard challenge.protectionSpace.authenticationMethod == NSURLAuthenticationMethodServerTrust,
              let trust = challenge.protectionSpace.serverTrust,
              let certificate = (SecTrustCopyCertificateChain(trust) as? [SecCertificate])?.first
        else { return (.cancelAuthenticationChallenge, nil) }
        let found = SHA256.hash(data: SecCertificateCopyData(certificate) as Data).map { String(format: "%02x", $0) }.joined()
        guard found == fingerprint else { return (.cancelAuthenticationChallenge, nil) }
        SecTrustSetAnchorCertificates(trust, [certificate] as CFArray)
        SecTrustSetAnchorCertificatesOnly(trust, true)
        return SecTrustEvaluateWithError(trust, nil) ? (.useCredential, URLCredential(trust: trust)) : (.cancelAuthenticationChallenge, nil)
    }

    func urlSession(_ session: URLSession, didReceive challenge: URLAuthenticationChallenge) async -> (URLSession.AuthChallengeDisposition, URLCredential?) { decide(challenge) }
    func urlSession(_ session: URLSession, task: URLSessionTask, didReceive challenge: URLAuthenticationChallenge) async -> (URLSession.AuthChallengeDisposition, URLCredential?) { decide(challenge) }
}

/// Minimal generic-password Keychain storage for this app.
enum Keychain {
    private static let service = "dev.audiopulse.assistant"

    static func read(account: String) -> Data? {
        let query: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service,
                                    kSecAttrAccount as String: account, kSecReturnData as String: true]
        var out: AnyObject?
        return SecItemCopyMatching(query as CFDictionary, &out) == errSecSuccess ? out as? Data : nil
    }

    static func write(_ data: Data, account: String) {
        delete(account: account)
        let item: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service,
                                   kSecAttrAccount as String: account, kSecValueData as String: data,
                                   kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly]
        SecItemAdd(item as CFDictionary, nil)
    }

    static func delete(account: String) {
        let query: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service,
                                    kSecAttrAccount as String: account]
        SecItemDelete(query as CFDictionary)
    }
}
