import Foundation
import SpotifyiOS
import SwiftUI

/// iPhone Spotify connection: Client ID → sign-in → library, then App Remote
/// to play through the Spotify app.
@MainActor @Observable
final class PhoneSpotifySession {
    enum Status: Equatable {
        case needsClientID, signedOut, signingIn, loadingLibrary, ready
        case failed(String)
    }

    enum Link: Equatable { case disconnected, connecting, connected }

    private(set) var status: Status = .signedOut
    private(set) var link: Link = .disconnected
    var dismissed = UserDefaults.standard.bool(forKey: "useSampleLibrary") {
        didSet { UserDefaults.standard.set(dismissed, forKey: "useSampleLibrary") }
    }

    var isConnected: Bool { status == .ready }
    var showConnectScreen: Bool { !dismissed && status != .ready }
    var isBusy: Bool { status == .signingIn || status == .loadingLibrary }

    private let player: PlayerModel
    private let library: LibraryStore
    private var api: SpotifyAPI?
    private var remote: AppRemoteController?
    private var playback: PhoneSpotifyPlayback?
    private var remoteToken: String?
    private var imageCache: [String: URL] = [:]
    private var lastTrackID: String?

    init(player: PlayerModel, library: LibraryStore) {
        self.player = player
        self.library = library
    }

    // MARK: Lifecycle

    func start() {
        guard SpotifyConfig.clientID != nil else { status = .needsClientID; return }
        if let token = SpotifyAuth.loadToken(), let id = SpotifyConfig.clientID {
            dismissed = false
            connect(clientID: id, token: token)
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
            status = .failed("Couldn’t save the Client ID: \(error.localizedDescription)")
        }
    }

    func signIn() {
        guard let clientID = SpotifyConfig.clientID else { status = .needsClientID; return }
        dismissed = false
        status = .signingIn
        Task {
            do {
                let token = try await SpotifyAuth.signIn(clientID: clientID)
                connect(clientID: clientID, token: token)
            } catch SpotifyAuthError.cancelled {
                status = .signedOut
            } catch {
                status = .failed(error.localizedDescription)
            }
        }
    }

    func retry() {
        if SpotifyConfig.clientID == nil { status = .needsClientID; return }
        start()
    }

    func useSampleLibrary() { dismissed = true }

    func signOut() {
        remote?.disconnect()
        SpotifyAuth.clearToken()
        api = nil
        remote = nil
        playback = nil
        link = .disconnected
        library.useSample()
        player.detachToSample()
        status = SpotifyConfig.clientID == nil ? .needsClientID : .signedOut
    }

    /// App came to the foreground: reconnect to the Spotify app if it's running.
    func becameActive() {
        guard status == .ready, let remote, !remote.isConnected else { return }
        Task {
            guard let token = await connectionToken() else { return }
            link = .connecting
            remote.connect(token: token)
        }
    }

    /// App went to the background: App Remote should disconnect.
    func resignedActive() {
        remote?.disconnect()
    }

    /// audiopulse://spotify-callback from the Spotify app.
    func handle(_ url: URL) {
        _ = remote?.handle(url)
    }

    // MARK: Connect

    private func connect(clientID: String, token: SpotifyToken) {
        Task {
            do {
                let api = SpotifyAPI(clientID: clientID, token: token)
                self.api = api
                _ = try await api.validToken()
                status = .loadingLibrary
                try await SpotifyLibraryLoader.load(api, into: library)

                let remote = AppRemoteController(clientID: clientID)
                let playback = PhoneSpotifyPlayback(api: api, remote: remote)
                playback.onError = { [weak self] msg in self?.player.notice = msg }
                wire(remote, playback: playback)
                self.remote = remote
                self.playback = playback
                player.attach(playback)
                status = .ready

                // Quietly attach to Spotify if it's already running.
                if let t = await connectionToken() {
                    link = .connecting
                    remote.connect(token: t)
                }
            } catch {
                status = .failed(error.localizedDescription)
            }
        }
    }

    /// App Remote token: the one Spotify handed back, else the Web API token
    /// (same Client ID, with the app-remote-control scope).
    private func connectionToken() async -> String? {
        if let remoteToken { return remoteToken }
        return try? await api?.validToken()
    }

    private func wire(_ remote: AppRemoteController, playback: PhoneSpotifyPlayback) {
        remote.onToken = { [weak self] token in
            self?.remoteToken = token
            self?.link = .connecting
        }
        remote.onConnected = { [weak self, weak playback] in
            self?.link = .connected
            playback?.connectionEstablished()
        }
        remote.onDisconnected = { [weak self] message in
            guard let self else { return }
            self.link = .disconnected
            // Expected when Spotify isn't running yet; only surface real problems.
            if let message, self.player.isPlaying == false, message.localizedCaseInsensitiveContains("install") {
                self.player.notice = message
            }
        }
        remote.onState = { [weak self] state in self?.apply(state) }
    }

    // MARK: State

    private func apply(_ s: SPTAppRemotePlayerState) {
        let uri = s.track.uri
        let id = uri.split(separator: ":").last.map(String.init) ?? uri
        var track = Track(id: id, title: s.track.name, artist: s.track.artist.name, album: s.track.album.name,
                          genre: "", duration: Double(s.track.duration) / 1000,
                          artwork: .placeholder(for: s.track.album.name, imageURL: imageCache[id]), uri: uri)
        if let current = library.liked.tracks.first(where: { $0.id == id }) ?? library.picks.first(where: { $0.id == id }) {
            track.artwork = current.artwork
        }
        let repeatMode: RepeatMode = switch s.playbackOptions.repeatMode {
        case .track: .one
        case .context: .all
        default: .off
        }
        player.apply(RemotePlayerState(track: track, isPlaying: !s.isPaused,
                                       position: Double(s.playbackPosition) / 1000, volume: nil,
                                       shuffle: s.playbackOptions.isShuffling, repeatMode: repeatMode,
                                       contextURI: s.contextURI.absoluteString), library: library)

        guard id != lastTrackID, let api else { return }
        lastTrackID = id
        Task {
            if track.artwork.imageURL == nil, let t = try? await api.track(id),
               let url = t.album?.images?.first.flatMap({ URL(string: $0.url) }) {
                imageCache[id] = url
                player.refreshArtwork(trackID: id, imageURL: url)
            }
            if let saved = try? await api.isSaved(id) { player.setLikedFromRemote(saved) }
        }
    }
}
