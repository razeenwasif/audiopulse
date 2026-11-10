import Foundation

/// Spotify settings.
///
/// macOS shares its config directory with the Go TUI (Go's `os.UserConfigDir()`
/// is ~/Library/Application Support there), so a Client ID set for one works
/// for the other, and librespot's cached device credentials are shared too.
/// iOS keeps its own copy inside the app container; the Client ID can be baked
/// in at build time (`SPOTIFY_CLIENT_ID` → Info.plist `SpotifyClientID`).
enum SpotifyConfig {
    #if os(macOS)
    static let redirectURI = "http://127.0.0.1:8888/callback"
    static let callbackPort: UInt16 = 8888
    static let deviceName = "AudioPulse"
    #else
    static let callbackScheme = "audiopulse"
    static let redirectURI = "audiopulse://spotify-callback"
    #endif

    /// Same scopes as `internal/config/config.go`, plus App Remote on iOS.
    static var scopes: [String] {
        var s = [
            "user-read-private", "user-read-email",
            "playlist-read-private", "playlist-read-collaborative",
            "playlist-modify-public", "playlist-modify-private",
            "user-library-read", "user-library-modify",
            "user-read-playback-state", "user-modify-playback-state",
            "user-read-currently-playing", "user-read-recently-played",
            "user-top-read", "streaming",
        ]
        #if os(iOS)
        s.append("app-remote-control")
        #endif
        return s
    }

    static var directory: URL {
        let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
        let dir = base.appendingPathComponent("audiopulse", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true,
                                                 attributes: [.posixPermissions: 0o700])
        return dir
    }

    static var configFile: URL { directory.appendingPathComponent("config.json") }

    /// The app's own token cache (0600, like the TUI's token.json). Kept
    /// separate from the TUI's because Spotify rotates refresh tokens on every
    /// refresh, so two apps sharing one would sign each other out.
    static var tokenFile: URL {
        #if os(macOS)
        directory.appendingPathComponent("mac-token.json")
        #else
        directory.appendingPathComponent("token.json")
        #endif
    }

    /// The Client ID from the environment, config.json, or (iOS) the build.
    static var clientID: String? {
        if let env = ProcessInfo.processInfo.environment["SPOTIFY_CLIENT_ID"], !env.isEmpty { return env }
        if let data = try? Data(contentsOf: configFile),
           let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
           let id = obj["client_id"] as? String, !id.isEmpty {
            return id
        }
        #if os(iOS)
        if let baked = Bundle.main.object(forInfoDictionaryKey: "SpotifyClientID") as? String,
           !baked.isEmpty, !baked.hasPrefix("$(") {
            return baked
        }
        #endif
        return nil
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

    #if os(macOS)
    static var librespotCache: URL {
        let dir = directory.appendingPathComponent("librespot", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true,
                                                 attributes: [.posixPermissions: 0o700])
        return dir
    }

    static var librespotLog: URL {
        FileManager.default.temporaryDirectory.appendingPathComponent("audiopulse-mac-librespot.log")
    }

    /// librespot binary: ~/.cargo/bin first, then common install locations.
    static var librespotBinary: URL? {
        let home = FileManager.default.homeDirectoryForCurrentUser
        let candidates = [
            home.appendingPathComponent(".cargo/bin/librespot"),
            URL(fileURLWithPath: "/opt/homebrew/bin/librespot"),
            URL(fileURLWithPath: "/usr/local/bin/librespot"),
        ]
        return candidates.first { FileManager.default.isExecutableFile(atPath: $0.path) }
    }
    #endif
}
