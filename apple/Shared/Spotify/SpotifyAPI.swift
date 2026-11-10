import Foundation

// MARK: - Wire models (only the fields the app uses)

struct SPImage: Decodable, Sendable { let url: String }
struct SPArtistRef: Decodable, Sendable { let id: String?; let name: String }
struct SPAlbumRef: Decodable, Sendable { let name: String; let images: [SPImage]? }

struct SPTrack: Decodable, Sendable {
    let id: String?
    let uri: String
    let name: String
    let duration_ms: Int
    let artists: [SPArtistRef]
    let album: SPAlbumRef?
    let is_local: Bool?
    let type: String?
}

struct SPPlaylist: Decodable, Sendable {
    let id: String
    let uri: String
    let name: String
    let images: [SPImage]?
    let tracks: Count?
    let items: Count?
    struct Count: Decodable, Sendable { let total: Int }
    var total: Int { tracks?.total ?? items?.total ?? 0 }
}

struct SPPage<T: Decodable & Sendable>: Decodable, Sendable {
    let items: [T]
    let total: Int?
    let next: String?

    private enum CodingKeys: String, CodingKey { case items, total, next }

    /// Spotify sometimes returns `null` entries in lists; skip them.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        items = (try c.decodeIfPresent([Lenient<T>].self, forKey: .items) ?? []).compactMap(\.value)
        total = try c.decodeIfPresent(Int.self, forKey: .total)
        next = try c.decodeIfPresent(String.self, forKey: .next)
    }
}

/// Decodes a value, or nil if it is null or malformed.
struct Lenient<T: Decodable>: Decodable {
    let value: T?
    init(from decoder: Decoder) throws { value = try? T(from: decoder) }
}

struct SPPlaylistItem: Decodable, Sendable { let track: SPTrack?; let item: SPTrack? }
struct SPSavedTrack: Decodable, Sendable { let track: SPTrack }
struct SPPlayHistory: Decodable, Sendable { let track: SPTrack }

struct SPDevice: Decodable, Sendable {
    let id: String?
    let name: String
    let type: String?
    let is_active: Bool
    let volume_percent: Int?
}

struct SPPlayerState: Decodable, Sendable {
    let device: SPDevice?
    let is_playing: Bool
    let progress_ms: Int?
    let shuffle_state: Bool?
    let repeat_state: String?
    let item: SPTrack?
    let context: Context?
    struct Context: Decodable, Sendable { let uri: String? }
}

struct SPUser: Decodable, Sendable {
    let id: String
    let display_name: String?
    let product: String?
}

enum SpotifyAPIError: LocalizedError {
    case notSignedIn, http(Int, String), premiumRequired

    var errorDescription: String? {
        switch self {
        case .notSignedIn: "Not signed in to Spotify."
        case .premiumRequired: "Playback needs a Spotify Premium account."
        case .http(let code, let msg): "Spotify returned \(code): \(msg)"
        }
    }
}

// MARK: - Client

/// Thin Spotify Web API client with automatic token refresh and 429 back-off.
@MainActor
final class SpotifyAPI {
    private let clientID: String
    private var token: SpotifyToken
    private let base = URL(string: "https://api.spotify.com/v1/")!

    init(clientID: String, token: SpotifyToken) {
        self.clientID = clientID
        self.token = token
    }

    var accessToken: String { token.accessToken }

    func validToken() async throws -> String {
        if token.isExpired { token = try await SpotifyAuth.refresh(token, clientID: clientID) }
        return token.accessToken
    }

    @discardableResult
    func send(_ method: String, _ path: String, query: [String: String] = [:], body: [String: Any]? = nil) async throws -> Data {
        var attempt = 0
        while true {
            attempt += 1
            var comps = URLComponents(url: base.appendingPathComponent(path), resolvingAgainstBaseURL: false)!
            if !query.isEmpty { comps.queryItems = query.map { URLQueryItem(name: $0.key, value: $0.value) } }
            var req = URLRequest(url: comps.url!)
            req.httpMethod = method
            req.setValue("Bearer \(try await validToken())", forHTTPHeaderField: "Authorization")
            if let body {
                req.setValue("application/json", forHTTPHeaderField: "Content-Type")
                req.httpBody = try JSONSerialization.data(withJSONObject: body)
            } else if method != "GET" {
                req.setValue("0", forHTTPHeaderField: "Content-Length")
            }
            let (data, resp) = try await URLSession.shared.data(for: req)
            let status = (resp as? HTTPURLResponse)?.statusCode ?? 0
            switch status {
            case 200...299:
                return data
            case 401 where attempt == 1:
                token = try await SpotifyAuth.refresh(token, clientID: clientID)
            case 429 where attempt <= 3:
                let wait = Double((resp as? HTTPURLResponse)?.value(forHTTPHeaderField: "Retry-After") ?? "1") ?? 1
                try await Task.sleep(for: .seconds(min(wait, 10)))
            default:
                let message = Self.errorMessage(data)
                if status == 403 && message.localizedCaseInsensitiveContains("premium") { throw SpotifyAPIError.premiumRequired }
                throw SpotifyAPIError.http(status, message)
            }
        }
    }

    func get<T: Decodable>(_ path: String, query: [String: String] = [:], as: T.Type = T.self) async throws -> T {
        let data = try await send("GET", path, query: query)
        return try JSONDecoder().decode(T.self, from: data)
    }

    private static func errorMessage(_ data: Data) -> String {
        struct E: Decodable { struct Inner: Decodable { let message: String? }; let error: Inner? }
        return (try? JSONDecoder().decode(E.self, from: data))?.error?.message
            ?? String(data: data, encoding: .utf8)?.prefix(200).description ?? ""
    }

    // MARK: Library

    func me() async throws -> SPUser { try await get("me") }

    func playlists(limit: Int = 200) async throws -> [SPPlaylist] {
        var out: [SPPlaylist] = []
        var offset = 0
        while out.count < limit {
            let page: SPPage<SPPlaylist> = try await get("me/playlists", query: ["limit": "50", "offset": "\(offset)"])
            out += page.items
            offset += page.items.count
            if page.next == nil || page.items.isEmpty { break }
        }
        return out
    }

    func playlistTracks(_ id: String, limit: Int = 500) async throws -> [SPTrack] {
        var out: [SPTrack] = []
        var offset = 0
        while out.count < limit {
            let page: SPPage<SPPlaylistItem> = try await get("playlists/\(id)/tracks",
                                                             query: ["limit": "100", "offset": "\(offset)"])
            out += page.items.compactMap { $0.track ?? $0.item }.filter { $0.type != "episode" && $0.is_local != true }
            offset += page.items.count
            if page.next == nil || page.items.isEmpty { break }
        }
        return out
    }

    func likedTracks(limit: Int = 500) async throws -> (tracks: [SPTrack], total: Int) {
        var out: [SPTrack] = []
        var offset = 0
        var total = 0
        while out.count < limit {
            let page: SPPage<SPSavedTrack> = try await get("me/tracks", query: ["limit": "50", "offset": "\(offset)"])
            total = page.total ?? total
            out += page.items.map(\.track)
            offset += page.items.count
            if page.next == nil || page.items.isEmpty { break }
        }
        return (out, total)
    }

    func recentlyPlayed() async throws -> [SPTrack] {
        let page: SPPage<SPPlayHistory> = try await get("me/player/recently-played", query: ["limit": "50"])
        return page.items.map(\.track)
    }

    func track(_ id: String) async throws -> SPTrack { try await get("tracks/\(id)") }

    /// Best match for a song, or nil.
    func searchTrack(title: String, artist: String) async throws -> SPTrack? {
        struct R: Decodable { let tracks: SPPage<SPTrack> }
        let q = artist.isEmpty ? "track:\(title)" : "track:\(title) artist:\(artist)"
        if let hit = try await get("search", query: ["q": q, "type": "track", "limit": "1"], as: R.self).tracks.items.first { return hit }
        // Looser second try: plain words.
        return try await get("search", query: ["q": "\(title) \(artist)", "type": "track", "limit": "1"], as: R.self).tracks.items.first
    }

    func createPlaylist(userID: String, name: String, description: String) async throws -> SPPlaylist {
        let data = try await send("POST", "users/\(userID)/playlists",
                                  body: ["name": name, "description": description, "public": false])
        return try JSONDecoder().decode(SPPlaylist.self, from: data)
    }

    func addTracks(playlistID: String, uris: [String]) async throws {
        for start in stride(from: 0, to: uris.count, by: 100) {
            try await send("POST", "playlists/\(playlistID)/tracks", body: ["uris": Array(uris[start..<min(uris.count, start + 100)])])
        }
    }

    func addToQueue(uri: String) async throws {
        try await send("POST", "me/player/queue", query: ["uri": uri])
    }

    func isSaved(_ trackID: String) async throws -> Bool {
        let flags: [Bool] = try await get("me/tracks/contains", query: ["ids": trackID])
        return flags.first ?? false
    }

    func setSaved(_ trackID: String, _ saved: Bool) async throws {
        try await send(saved ? "PUT" : "DELETE", "me/tracks", query: ["ids": trackID])
    }

    // MARK: Player

    func devices() async throws -> [SPDevice] {
        struct R: Decodable { let devices: [SPDevice] }
        return try await get("me/player/devices", as: R.self).devices
    }

    func state() async throws -> SPPlayerState? {
        let data = try await send("GET", "me/player", query: ["additional_types": "track"])
        if data.isEmpty { return nil }
        return try JSONDecoder().decode(SPPlayerState.self, from: data)
    }

    func transfer(to deviceID: String, play: Bool) async throws {
        try await send("PUT", "me/player", body: ["device_ids": [deviceID], "play": play])
    }

    func play(deviceID: String, contextURI: String?, uris: [String]?, offsetURI: String?, position: Int?) async throws {
        var body: [String: Any] = [:]
        if let contextURI { body["context_uri"] = contextURI }
        if let uris { body["uris"] = uris }
        if let offsetURI { body["offset"] = ["uri": offsetURI] } else if let position { body["offset"] = ["position": position] }
        try await send("PUT", "me/player/play", query: ["device_id": deviceID], body: body)
    }

    func resume(deviceID: String) async throws { try await send("PUT", "me/player/play", query: ["device_id": deviceID]) }
    func pause() async throws { try await send("PUT", "me/player/pause") }
    func next() async throws { try await send("POST", "me/player/next") }
    func previous() async throws { try await send("POST", "me/player/previous") }
    func seek(ms: Int) async throws { try await send("PUT", "me/player/seek", query: ["position_ms": "\(ms)"]) }
    func volume(_ percent: Int) async throws { try await send("PUT", "me/player/volume", query: ["volume_percent": "\(percent)"]) }
    func shuffle(_ on: Bool) async throws { try await send("PUT", "me/player/shuffle", query: ["state": on ? "true" : "false"]) }
    func repeatMode(_ state: String) async throws { try await send("PUT", "me/player/repeat", query: ["state": state]) }
}
