import Foundation
import SwiftUI

/// Orchestrates the Spotify connection on macOS: Client ID → sign-in → library
/// → librespot device → remote playback with state polling.
@MainActor @Observable
final class SpotifySession {
    enum Status: Equatable {
        case needsClientID
        case signedOut
        case signingIn
        case loadingLibrary
        case authorizingDevice(browser: Bool)
        case startingDevice
        case ready
        case failed(String)
    }

    private(set) var status: Status = .signedOut
    /// The user chose the sample library for now; hides the connect screen.
    var dismissed = UserDefaults.standard.bool(forKey: "useSampleLibrary") {
        didSet { UserDefaults.standard.set(dismissed, forKey: "useSampleLibrary") }
    }

    var isConnected: Bool { status == .ready }
    var showConnectScreen: Bool { !dismissed && status != .ready }
    var isBusy: Bool {
        switch status {
        case .signingIn, .loadingLibrary, .authorizingDevice, .startingDevice: true
        default: false
        }
    }

    private let player: PlayerModel
    private let library: LibraryStore
    private let device = LibrespotDevice()
    private var api: SpotifyAPI?
    private var playback: SpotifyPlayback?
    private var poller: Task<Void, Never>?
    private var connectTask: Task<Void, Never>?

    init(player: PlayerModel, library: LibraryStore) {
        self.player = player
        self.library = library
    }

    // MARK: Lifecycle

    /// Called at launch: reconnects silently if a token is cached.
    func start() {
        guard let clientID = SpotifyConfig.clientID else { status = .needsClientID; return }
        if let token = SpotifyAuth.loadToken() {
            dismissed = false
            connect(clientID: clientID, token: token)
        } else {
            status = .signedOut
        }
    }

    func saveClientID(_ id: String) {
        let trimmed = id.trimmingCharacters(in: .whitespacesAndNewlines)
        guard trimmed.count >= 16, trimmed.allSatisfy({ $0.isLetter || $0.isNumber }) else {
            status = .failed("That doesn’t look like a Spotify Client ID (32 letters and numbers).")
            return
        }
        do {
            try SpotifyConfig.saveClientID(trimmed)
            status = .signedOut
        } catch {
            status = .failed("Couldn’t save config.json: \(error.localizedDescription)")
        }
    }

    func signIn() {
        guard let clientID = SpotifyConfig.clientID else { status = .needsClientID; return }
        dismissed = false
        status = .signingIn
        connectTask = Task {
            do {
                let token = try await SpotifyAuth.signIn(clientID: clientID)
                connect(clientID: clientID, token: token)
            } catch {
                status = .failed(error.localizedDescription)
            }
        }
    }

    func retry() {
        if SpotifyConfig.clientID == nil { status = .needsClientID; return }
        if let token = SpotifyAuth.loadToken(), let id = SpotifyConfig.clientID {
            connect(clientID: id, token: token)
        } else {
            status = .signedOut
        }
    }

    func useSampleLibrary() {
        dismissed = true
        if status != .ready { connectTask?.cancel() }
    }

    func signOut() {
        poller?.cancel()
        device.stop()
        SpotifyAuth.clearToken()
        api = nil
        playback = nil
        library.useSample()
        player.detachToSample()
        status = SpotifyConfig.clientID == nil ? .needsClientID : .signedOut
    }

    func shutdown() {
        poller?.cancel()
        device.stop()
    }

    // MARK: Connect

    private func connect(clientID: String, token: SpotifyToken) {
        connectTask = Task {
            do {
                let api = SpotifyAPI(clientID: clientID, token: token)
                self.api = api
                _ = try await api.validToken()

                status = .loadingLibrary
                try await loadLibrary(api)

                try await authorizeDeviceIfNeeded()

                status = .startingDevice
                device.onInvalidCredentials = { [weak self] in self?.reauthorizeDevice() }
                try device.start()
                guard let deviceID = await waitForDevice(api, timeout: 25) else {
                    throw LibrespotError.exited(LibrespotDevice.logTail())
                }
                try? await api.transfer(to: deviceID, play: false)

                let playback = SpotifyPlayback(api: api, deviceID: deviceID)
                playback.onError = { [weak self] msg in self?.player.notice = msg }
                playback.onDeviceLost = { [weak self] in
                    guard let self, let api = self.api else { return nil }
                    return await self.waitForDevice(api, timeout: 10)
                }
                self.playback = playback
                player.attach(playback)
                status = .ready
                startPolling(api)
            } catch is CancellationError {
                return
            } catch {
                status = .failed(error.localizedDescription)
            }
        }
    }

    private func authorizeDeviceIfNeeded() async throws {
        guard !device.hasCredentials else { return }
        status = .authorizingDevice(browser: true)
        try await device.ensureCredentials()
    }

    /// The device's cached login was rejected: drop it, approve again, restart.
    private func reauthorizeDevice() {
        guard let api else { return }
        poller?.cancel()
        device.stop()
        device.discardCredentials()
        connectTask = Task {
            do {
                try await authorizeDeviceIfNeeded()
                status = .startingDevice
                try device.start()
                guard let deviceID = await waitForDevice(api, timeout: 25) else {
                    throw LibrespotError.exited(LibrespotDevice.logTail())
                }
                try? await api.transfer(to: deviceID, play: false)
                playback?.deviceID = deviceID
                if let playback { player.attach(playback) }
                status = .ready
                startPolling(api)
            } catch is CancellationError {
                return
            } catch {
                status = .failed(error.localizedDescription)
            }
        }
    }

    private func waitForDevice(_ api: SpotifyAPI, timeout: TimeInterval) async -> String? {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if let d = try? await api.devices().first(where: { $0.name == SpotifyConfig.deviceName }), let id = d.id {
                return id
            }
            try? await Task.sleep(for: .seconds(1))
        }
        return nil
    }

    private func loadLibrary(_ api: SpotifyAPI) async throws {
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
        library.trackLoader = { [weak self] p in
            guard let api = self?.api else { return [] }
            return try await api.playlistTracks(p.id).map(Self.track)
        }
    }

    private func startPolling(_ api: SpotifyAPI) {
        poller?.cancel()
        poller = Task { [weak self] in
            var lastTrackID: String?
            while !Task.isCancelled {
                guard let self else { return }
                if let state = try? await api.state() {
                    let remote = Self.remoteState(state)
                    if remote.track != nil || state.device != nil {
                        self.player.apply(remote, library: self.library)
                    }
                    if let id = remote.track?.id, id != lastTrackID {
                        lastTrackID = id
                        if let saved = try? await api.isSaved(id) { self.player.setLikedFromRemote(saved) }
                    }
                }
                try? await Task.sleep(for: .seconds(self.player.isPlaying ? 1.5 : 4))
            }
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
