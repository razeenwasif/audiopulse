import AppKit
import CryptoKit
import Foundation
import Network

struct SpotifyToken: Codable, Sendable {
    var accessToken: String
    var refreshToken: String
    var expiresAt: Date
    var scope: String

    var isExpired: Bool { expiresAt.timeIntervalSinceNow < 60 }
}

enum SpotifyAuthError: LocalizedError {
    case cancelled, portInUse, denied(String), badResponse(String), timedOut

    var errorDescription: String? {
        switch self {
        case .cancelled: "Sign-in was cancelled."
        case .portInUse: "Port 8888 is busy (is the terminal app signing in?). Close it and try again."
        case .denied(let why): "Spotify didn’t grant access: \(why)."
        case .badResponse(let why): "Spotify sign-in failed: \(why)"
        case .timedOut: "Timed out waiting for the browser sign-in."
        }
    }
}

/// OAuth 2.0 Authorization Code + PKCE for a public client, mirroring
/// `internal/auth`: a short-lived loopback server on 127.0.0.1:8888 catches the
/// redirect, so the same registered Redirect URI works for the TUI and the app.
enum SpotifyAuth {
    // MARK: Token cache

    static func loadToken() -> SpotifyToken? {
        guard let data = try? Data(contentsOf: SpotifyConfig.tokenFile) else { return nil }
        return try? JSONDecoder().decode(SpotifyToken.self, from: data)
    }

    static func saveToken(_ token: SpotifyToken) {
        guard let data = try? JSONEncoder().encode(token) else { return }
        try? data.write(to: SpotifyConfig.tokenFile, options: .atomic)
        try? FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: SpotifyConfig.tokenFile.path)
    }

    static func clearToken() {
        try? FileManager.default.removeItem(at: SpotifyConfig.tokenFile)
    }

    // MARK: Sign in

    /// Opens the browser, waits for the redirect, and exchanges the code.
    static func signIn(clientID: String) async throws -> SpotifyToken {
        let verifier = randomURLSafe(64)
        let challenge = Data(SHA256.hash(data: Data(verifier.utf8))).base64URL
        let state = randomURLSafe(16)

        var comps = URLComponents(string: "https://accounts.spotify.com/authorize")!
        comps.queryItems = [
            .init(name: "client_id", value: clientID),
            .init(name: "response_type", value: "code"),
            .init(name: "redirect_uri", value: SpotifyConfig.redirectURI),
            .init(name: "code_challenge_method", value: "S256"),
            .init(name: "code_challenge", value: challenge),
            .init(name: "state", value: state),
            .init(name: "scope", value: SpotifyConfig.scopes.joined(separator: " ")),
        ]

        let server = LoopbackServer(port: SpotifyConfig.callbackPort)
        try await server.start()
        defer { server.stop() }

        _ = await MainActor.run { NSWorkspace.shared.open(comps.url!) }

        let params = try await server.waitForCallback(timeout: 300)
        if let error = params["error"] { throw SpotifyAuthError.denied(error) }
        guard params["state"] == state else { throw SpotifyAuthError.badResponse("state mismatch") }
        guard let code = params["code"] else { throw SpotifyAuthError.badResponse("no authorization code") }

        return try await tokenRequest([
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": SpotifyConfig.redirectURI,
            "client_id": clientID,
            "code_verifier": verifier,
        ], previousRefresh: nil)
    }

    static func refresh(_ token: SpotifyToken, clientID: String) async throws -> SpotifyToken {
        try await tokenRequest([
            "grant_type": "refresh_token",
            "refresh_token": token.refreshToken,
            "client_id": clientID,
        ], previousRefresh: token.refreshToken)
    }

    private static func tokenRequest(_ form: [String: String], previousRefresh: String?) async throws -> SpotifyToken {
        var req = URLRequest(url: URL(string: "https://accounts.spotify.com/api/token")!)
        req.httpMethod = "POST"
        req.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
        req.httpBody = form.map { "\($0.key)=\($0.value.formEncoded)" }.joined(separator: "&").data(using: .utf8)
        let (data, resp) = try await URLSession.shared.data(for: req)
        let status = (resp as? HTTPURLResponse)?.statusCode ?? 0
        struct Body: Decodable {
            let access_token: String?
            let refresh_token: String?
            let expires_in: Double?
            let scope: String?
            let error: String?
            let error_description: String?
        }
        let body = try JSONDecoder().decode(Body.self, from: data)
        guard status == 200, let access = body.access_token else {
            throw SpotifyAuthError.badResponse(body.error_description ?? body.error ?? "HTTP \(status)")
        }
        let token = SpotifyToken(
            accessToken: access,
            refreshToken: body.refresh_token ?? previousRefresh ?? "",
            expiresAt: Date().addingTimeInterval(body.expires_in ?? 3600),
            scope: body.scope ?? "")
        saveToken(token)
        return token
    }

    private static func randomURLSafe(_ bytes: Int) -> String {
        var data = Data(count: bytes)
        _ = data.withUnsafeMutableBytes { SecRandomCopyBytes(kSecRandomDefault, bytes, $0.baseAddress!) }
        return data.base64URL
    }
}

// MARK: - Loopback server

/// Minimal one-shot HTTP server on 127.0.0.1 that captures `/callback?…`.
private final class LoopbackServer: @unchecked Sendable {
    private let port: UInt16
    private var listener: NWListener?
    private let queue = DispatchQueue(label: "dev.audiopulse.oauth")
    private var continuation: CheckedContinuation<[String: String], Error>?
    private let lock = NSLock()

    init(port: UInt16) { self.port = port }

    func start() async throws {
        let params = NWParameters.tcp
        params.requiredLocalEndpoint = .hostPort(host: "127.0.0.1", port: NWEndpoint.Port(rawValue: port)!)
        params.allowLocalEndpointReuse = true
        let listener: NWListener
        do { listener = try NWListener(using: params) } catch { throw SpotifyAuthError.portInUse }
        self.listener = listener
        listener.newConnectionHandler = { [weak self] conn in self?.handle(conn) }

        try await withCheckedThrowingContinuation { (cont: CheckedContinuation<Void, Error>) in
            var resumed = false
            listener.stateUpdateHandler = { state in
                guard !resumed else { return }
                switch state {
                case .ready: resumed = true; cont.resume()
                case .failed: resumed = true; cont.resume(throwing: SpotifyAuthError.portInUse)
                default: break
                }
            }
            listener.start(queue: queue)
        }
    }

    func stop() {
        listener?.cancel()
        listener = nil
    }

    func waitForCallback(timeout: TimeInterval) async throws -> [String: String] {
        try await withThrowingTaskGroup(of: [String: String].self) { group in
            group.addTask {
                try await withCheckedThrowingContinuation { cont in
                    self.lock.lock(); self.continuation = cont; self.lock.unlock()
                }
            }
            group.addTask {
                try await Task.sleep(for: .seconds(timeout))
                throw SpotifyAuthError.timedOut
            }
            defer { group.cancelAll() }
            let result = try await group.next()!
            return result
        }
    }

    private func finish(_ result: Result<[String: String], Error>) {
        lock.lock()
        let cont = continuation
        continuation = nil
        lock.unlock()
        cont?.resume(with: result)
    }

    private func handle(_ conn: NWConnection) {
        conn.start(queue: queue)
        conn.receive(minimumIncompleteLength: 1, maximumLength: 16_384) { [weak self] data, _, _, _ in
            guard let self, let data, let text = String(data: data, encoding: .utf8),
                  let line = text.split(separator: "\r\n").first else { conn.cancel(); return }
            let parts = line.split(separator: " ")
            guard parts.count >= 2, let comps = URLComponents(string: String(parts[1])),
                  comps.path == "/callback" else {
                self.respond(conn, status: "404 Not Found", body: "Not found")
                return
            }
            var params: [String: String] = [:]
            for item in comps.queryItems ?? [] { params[item.name] = item.value ?? "" }
            let ok = params["code"] != nil
            self.respond(conn, status: "200 OK", body: Self.page(ok: ok))
            self.finish(.success(params))
        }
    }

    private func respond(_ conn: NWConnection, status: String, body: String) {
        let payload = Data(body.utf8)
        let head = "HTTP/1.1 \(status)\r\nContent-Type: text/html; charset=utf-8\r\nContent-Length: \(payload.count)\r\nConnection: close\r\n\r\n"
        conn.send(content: Data(head.utf8) + payload, completion: .contentProcessed { _ in conn.cancel() })
    }

    private static func page(ok: Bool) -> String {
        let title = ok ? "You’re connected" : "Sign-in didn’t finish"
        let detail = ok ? "AudioPulse has your Spotify library. You can close this tab." : "Head back to AudioPulse and try again."
        return """
        <!doctype html><html><head><meta charset="utf-8"><title>AudioPulse</title>
        <style>body{margin:0;height:100vh;display:grid;place-items:center;background:#050505;color:#EDF2EF;
        font-family:-apple-system,system-ui,sans-serif}main{text-align:center}
        h1{font-size:40px;letter-spacing:-1px;margin:0 0 8px}p{color:#A3ACA7;margin:0}
        .dot{width:14px;height:14px;border-radius:7px;background:#5EE6A0;margin:0 auto 22px;box-shadow:0 0 40px #5EE6A0}</style>
        </head><body><main><div class="dot"></div><h1>\(title)</h1><p>\(detail)</p></main></body></html>
        """
    }
}

// MARK: - Encoding helpers

extension Data {
    var base64URL: String {
        base64EncodedString().replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_").replacingOccurrences(of: "=", with: "")
    }
}

extension String {
    var formEncoded: String {
        var allowed = CharacterSet.alphanumerics
        allowed.insert(charactersIn: "-._~")
        return addingPercentEncoding(withAllowedCharacters: allowed) ?? self
    }
}
