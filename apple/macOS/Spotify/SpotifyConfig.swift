import Foundation

/// Settings shared with the Go TUI. Both read the same config directory
/// (Go's `os.UserConfigDir()` on macOS is ~/Library/Application Support), so a
/// Client ID set for one works for the other, and librespot's cached device
/// credentials are shared too.
enum SpotifyConfig {
    static let redirectURI = "http://127.0.0.1:8888/callback"
    static let callbackPort: UInt16 = 8888
    static let deviceName = "AudioPulse"

    /// Same scopes as `internal/config/config.go`.
    static let scopes = [
        "user-read-private", "user-read-email",
        "playlist-read-private", "playlist-read-collaborative",
        "playlist-modify-public", "playlist-modify-private",
        "user-library-read", "user-library-modify",
        "user-read-playback-state", "user-modify-playback-state",
        "user-read-currently-playing", "user-read-recently-played",
        "user-top-read", "streaming",
    ]

    static var directory: URL {
        let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        let dir = base.appendingPathComponent("audiopulse", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true,
                                                 attributes: [.posixPermissions: 0o700])
        return dir
    }

    static var configFile: URL { directory.appendingPathComponent("config.json") }

    /// The Mac app's own token cache (0600, like the TUI's token.json). Kept
    /// separate because Spotify rotates refresh tokens on every refresh, so two
    /// apps sharing one would sign each other out.
    static var tokenFile: URL { directory.appendingPathComponent("mac-token.json") }

    static var librespotCache: URL {
        let dir = directory.appendingPathComponent("librespot", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true,
                                                 attributes: [.posixPermissions: 0o700])
        return dir
    }

    static var librespotLog: URL {
        FileManager.default.temporaryDirectory.appendingPathComponent("audiopulse-mac-librespot.log")
    }

    /// The Client ID from `SPOTIFY_CLIENT_ID` or config.json.
    static var clientID: String? {
        if let env = ProcessInfo.processInfo.environment["SPOTIFY_CLIENT_ID"], !env.isEmpty { return env }
        guard let data = try? Data(contentsOf: configFile),
              let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let id = obj["client_id"] as? String, !id.isEmpty
        else { return nil }
        return id
    }

    /// Writes the Client ID into config.json, preserving any other keys the TUI uses.
    static func saveClientID(_ id: String) throws {
        var obj: [String: Any] = [:]
        if let data = try? Data(contentsOf: configFile),
           let existing = try? JSONSerialization.jsonObject(with: data) as? [String: Any] {
            obj = existing
        }
        obj["client_id"] = id.trimmingCharacters(in: .whitespacesAndNewlines)
        let data = try JSONSerialization.data(withJSONObject: obj, options: [.prettyPrinted, .sortedKeys])
        try data.write(to: configFile, options: .atomic)
        try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: configFile.path)
    }

    /// librespot binary: ~/.cargo/bin first, then PATH-style common locations.
    static var librespotBinary: URL? {
        let home = FileManager.default.homeDirectoryForCurrentUser
        let candidates = [
            home.appendingPathComponent(".cargo/bin/librespot"),
            URL(fileURLWithPath: "/opt/homebrew/bin/librespot"),
            URL(fileURLWithPath: "/usr/local/bin/librespot"),
        ]
        return candidates.first { FileManager.default.isExecutableFile(atPath: $0.path) }
    }
}
