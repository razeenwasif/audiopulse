import Foundation

/// Loads the Spotify library into `LibraryStore` and maps API models to the
/// app's models. Shared by the macOS and iOS sessions.
@MainActor
enum SpotifyLibraryLoader {
    static func load(_ api: SpotifyAPI, into library: LibraryStore) async throws {
        library.isLoading = true
        defer { library.isLoading = false }
        async let meCall = api.me()
        async let playlistsCall = api.playlists()
        async let likedCall = api.likedTracks(limit: 200)
        async let recentCall = api.recentlyPlayed()
        let me = try await meCall
        let sp = try await playlistsCall
        let liked = try await likedCall
        let recent = (try? await recentCall) ?? []

        let playlists = sp.map(Self.playlist)
        let likedPlaylist = Playlist(
            id: LibraryStore.likedID, name: "Liked Songs", initials: "LS", badgeColor: 0x1E8F62, badgeOnDark: true,
            artwork: Artwork(colors: [0x1E8F62, 0xB8F5CF], angle: 135),
            tracks: liked.tracks.map(Self.track), reportedCount: liked.total)
        let detail = (me.product == "premium") ? "Spotify Premium" : "Spotify \((me.product ?? "Free").capitalized)"
        library.useSpotify(userName: me.display_name ?? me.id, detail: detail, playlists: playlists,
                           liked: likedPlaylist, likedCount: liked.total, recent: recent.map(Self.track))
        library.userID = me.id
        library.api = api
        library.trackLoader = { [weak api] p in
            guard let api else { return [] }
            return try await api.playlistTracks(p.id).map(Self.track)
        }
    }

    // MARK: Mapping


    static func track(_ t: SPTrack) -> Track {
        let image = t.album?.images?.first.flatMap { URL(string: $0.url) }
        return Track(id: t.id ?? t.uri, title: t.name, artist: t.artists.map(\.name).joined(separator: ", "),
                     album: t.album?.name ?? "", genre: "", duration: Double(t.duration_ms) / 1000,
                     artwork: .placeholder(for: t.album?.name ?? t.name, imageURL: image), uri: t.uri)
    }

    static func playlist(_ p: SPPlaylist) -> Playlist {
        let (color, dark) = LibraryStore.badge(for: p.id)
        let image = p.images?.first.flatMap { URL(string: $0.url) }
        return Playlist(id: p.id, name: p.name, initials: LibraryStore.initials(p.name), badgeColor: color,
                        badgeOnDark: dark, artwork: .placeholder(for: p.id, imageURL: image), tracks: [],
                        uri: p.uri, reportedCount: p.total, tracksLoaded: false)
    }

    static func remoteState(_ s: SPPlayerState) -> RemotePlayerState {
        let repeatMode: RepeatMode = switch s.repeat_state { case "track": .one; case "context": .all; default: .off }
        return RemotePlayerState(
            track: s.item.map(track), isPlaying: s.is_playing,
            position: Double(s.progress_ms ?? 0) / 1000,
            volume: s.device?.volume_percent.map { Double($0) / 100 },
            shuffle: s.shuffle_state ?? false, repeatMode: repeatMode, contextURI: s.context?.uri)
    }
}
