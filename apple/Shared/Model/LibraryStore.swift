import SwiftUI

/// A card in the "Built from your library" row.
struct HomeCard: Identifiable, Hashable {
    let id: String
    let name: String
    let detail: String
    let artwork: Artwork
    let playlistID: String
}

/// What the screens show: the sample library, or the user's real Spotify
/// library once connected (macOS).
@MainActor @Observable
final class LibraryStore {
    enum Source { case sample, spotify }

    static let likedID = "liked"

    private(set) var source: Source = .sample
    var userName = "Mika Hart"
    var userDetail = "Sample library"
    var playlists: [Playlist] = MockLibrary.playlists
    var liked: Playlist = LibraryStore.sampleLiked
    var likedCount = MockLibrary.likedCount
    var picks: [Track] = MockLibrary.picks
    /// Recently played songs (deduplicated, up to 20) for "Jump back in".
    var recent: [Track] = MockLibrary.tracks
    var picksTitle = "Because you played Low Tide all week"
    var homeCards: [HomeCard] = MockLibrary.builtFromLibrary.map {
        HomeCard(id: $0.name, name: $0.name, detail: $0.why, artwork: $0.art, playlistID: MockLibrary.rainyIndie.id)
    }
    var genreCount = MockLibrary.genres.count
    var isLoading = false

    /// Loads a playlist's tracks on demand (set by the Spotify session).
    var trackLoader: ((Playlist) async throws -> [Track])?

    var userInitials: String {
        let parts = userName.split(separator: " ").prefix(2)
        let letters = parts.compactMap(\.first).map(String.init).joined()
        return letters.isEmpty ? "♪" : letters.uppercased()
    }

    func playlist(_ id: String) -> Playlist? {
        id == Self.likedID ? liked : playlists.first { $0.id == id }
    }

    /// Fetches a playlist's tracks the first time it is opened.
    func ensureTracks(for id: String) async {
        guard let p = playlist(id), !p.tracksLoaded, let trackLoader else { return }
        do {
            let tracks = try await trackLoader(p)
            update(id) { $0.tracks = tracks; $0.tracksLoaded = true }
        } catch {
            update(id) { $0.tracksLoaded = true }
        }
    }

    private func update(_ id: String, _ change: (inout Playlist) -> Void) {
        if id == Self.likedID { change(&liked); return }
        if let i = playlists.firstIndex(where: { $0.id == id }) { change(&playlists[i]) }
    }

    func useSpotify(userName: String, detail: String, playlists: [Playlist], liked: Playlist, likedCount: Int, recent: [Track]) {
        source = .spotify
        self.userName = userName
        self.userDetail = detail
        self.playlists = playlists
        self.liked = liked
        self.likedCount = likedCount
        var seen = Set<String>()
        self.recent = recent.filter { seen.insert($0.id).inserted }.prefix(20).map { $0 }
        self.picks = Array(self.recent.prefix(5))
        self.picksTitle = "Jump back in"
        self.homeCards = playlists.prefix(3).map {
            HomeCard(id: $0.id, name: $0.name, detail: "\($0.reportedCount ?? $0.tracks.count) songs", artwork: $0.artwork, playlistID: $0.id)
        }
        self.genreCount = 0
    }

    func useSample() {
        source = .sample
        userName = "Mika Hart"
        userDetail = "Sample library"
        playlists = MockLibrary.playlists
        liked = Self.sampleLiked
        likedCount = MockLibrary.likedCount
        picks = MockLibrary.picks
        recent = MockLibrary.tracks
        picksTitle = "Because you played Low Tide all week"
        homeCards = MockLibrary.builtFromLibrary.map {
            HomeCard(id: $0.name, name: $0.name, detail: $0.why, artwork: $0.art, playlistID: MockLibrary.rainyIndie.id)
        }
        genreCount = MockLibrary.genres.count
        trackLoader = nil
    }

    static let sampleLiked = Playlist(
        id: likedID, name: "Liked Songs", initials: "LS", badgeColor: 0x1E8F62, badgeOnDark: true,
        artwork: Artwork(colors: [0x1E8F62, 0xB8F5CF], angle: 135), tracks: MockLibrary.tracks)

    /// Badge colors for playlist initials, cycling through the greens.
    static func badge(for key: String) -> (UInt32, Bool) {
        let options: [(UInt32, Bool)] = [(0x0F766E, true), (0xB8F5CF, false), (0x5EEAD4, false),
                                         (0x4D6B2F, true), (0xC6D9A0, false), (0x1F7A55, true)]
        let i = abs(key.unicodeScalars.reduce(0) { ($0 &* 31) &+ Int($1.value) }) % options.count
        return options[i]
    }

    static func initials(_ name: String) -> String {
        let words = name.split { !$0.isLetter && !$0.isNumber }.prefix(2)
        let s = words.compactMap(\.first).map(String.init).joined().uppercased()
        return s.isEmpty ? "♪" : s
    }
}
