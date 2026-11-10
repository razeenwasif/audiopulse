import Foundation
import SpotifyiOS

/// iPhone playback: the Spotify app plays the audio. App Remote handles the
/// connection and quick controls; starting a playlist at a given song uses the
/// Web API against this iPhone's Spotify device.
@MainActor
final class PhoneSpotifyPlayback: PlaybackBackend {
    let api: SpotifyAPI
    let remote: AppRemoteController
    var onError: ((String) -> Void)?

    /// A play request made while Spotify wasn't connected; replayed on connect.
    private var pending: (track: Track, playlist: Playlist?, queue: [Track])?

    init(api: SpotifyAPI, remote: AppRemoteController) {
        self.api = api
        self.remote = remote
    }

    var isRemote: Bool { true }

    func play(_ track: Track, in playlist: Playlist?, queue: [Track]) {
        guard remote.isConnected else {
            pending = (track, playlist, queue)
            remote.wake(playing: track.uri ?? playlist?.uri ?? "")
            return
        }
        Task { await startOnPhone(track, in: playlist, queue: queue) }
    }

    /// Called once App Remote connects: finish any play request that woke Spotify.
    func connectionEstablished() {
        guard let p = pending else { return }
        pending = nil
        // Only needed to restore the playlist context; a lone track is already playing.
        if p.playlist?.uri != nil || p.queue.count > 1 {
            Task { await startOnPhone(p.track, in: p.playlist, queue: p.queue) }
        }
    }

    private func startOnPhone(_ track: Track, in playlist: Playlist?, queue: [Track]) async {
        do {
            guard let deviceID = await phoneDeviceID() else {
                // No Web API device yet: fall back to App Remote for the single track.
                if let uri = track.uri { remote.player?.play(uri, callback: nil) }
                return
            }
            if let ctx = playlist?.uri, let uri = track.uri {
                try await api.play(deviceID: deviceID, contextURI: ctx, uris: nil, offsetURI: uri, position: nil)
            } else {
                let uris = (queue.isEmpty ? [track] : queue).compactMap(\.uri)
                let start = uris.firstIndex(of: track.uri ?? "") ?? 0
                let window = Array(uris[max(0, start - 50)..<min(uris.count, start + 450)])
                let pos = window.firstIndex(of: track.uri ?? "") ?? 0
                try await api.play(deviceID: deviceID, contextURI: nil, uris: window, offsetURI: nil, position: pos)
            }
        } catch {
            onError?(error.localizedDescription)
        }
    }

    /// This iPhone's Spotify device, as the Web API sees it.
    private func phoneDeviceID() async -> String? {
        for _ in 0..<5 {
            if let devices = try? await api.devices() {
                let phones = devices.filter { ($0.type ?? "").caseInsensitiveCompare("Smartphone") == .orderedSame }
                if let d = phones.first(where: \.is_active) ?? phones.first, let id = d.id { return id }
            }
            try? await Task.sleep(for: .milliseconds(600))
        }
        return nil
    }

    // MARK: Controls (App Remote when connected, Web API otherwise)

    func pause() {
        if let p = remote.player, remote.isConnected { p.pause(nil) } else { web { try await self.api.pause() } }
    }

    func resume() {
        if let p = remote.player, remote.isConnected { p.resume(nil) } else { remote.wake(playing: "") }
    }

    func next() {
        if let p = remote.player, remote.isConnected { p.skip(toNext: nil) } else { web { try await self.api.next() } }
    }

    func previous() {
        if let p = remote.player, remote.isConnected { p.skip(toPrevious: nil) } else { web { try await self.api.previous() } }
    }

    func seek(to time: TimeInterval) {
        let ms = Int(time * 1000)
        if let p = remote.player, remote.isConnected { p.seek(toPosition: ms, callback: nil) } else { web { try await self.api.seek(ms: ms) } }
    }

    func setShuffle(_ on: Bool) {
        if let p = remote.player, remote.isConnected { p.setShuffle(on, callback: nil) } else { web { try await self.api.shuffle(on) } }
    }

    func setRepeat(_ mode: RepeatMode) {
        if let p = remote.player, remote.isConnected {
            let m: SPTAppRemotePlaybackOptionsRepeatMode = switch mode { case .off: .off; case .one: .track; case .all: .context }
            p.setRepeatMode(m, callback: nil)
        } else {
            let state = switch mode { case .off: "off"; case .all: "context"; case .one: "track" }
            web { try await self.api.repeatMode(state) }
        }
    }

    /// The phone's hardware volume controls Spotify; nothing to send.
    func setVolume(_ volume: Double) {}

    func setLiked(_ liked: Bool, track: Track) {
        guard track.uri != nil else { return }
        web { try await self.api.setSaved(track.id, liked) }
    }

    private func web(_ op: @escaping @MainActor () async throws -> Void) {
        Task {
            do { try await op() } catch { self.onError?(error.localizedDescription) }
        }
    }
}
