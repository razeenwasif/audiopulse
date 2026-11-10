import Foundation

/// Plays through the librespot "AudioPulse" device, controlled with the Web API.
@MainActor
final class SpotifyPlayback: PlaybackBackend {
    let api: SpotifyAPI
    var deviceID: String
    /// Reports errors (e.g. Premium required) to the UI.
    var onError: ((String) -> Void)?
    /// Asks the session to re-resolve the device after librespot restarts.
    var onDeviceLost: (() async -> String?)?

    private var volumeTask: Task<Void, Never>?

    init(api: SpotifyAPI, deviceID: String) {
        self.api = api
        self.deviceID = deviceID
    }

    var isRemote: Bool { true }

    func play(_ track: Track, in playlist: Playlist?, queue: [Track]) {
        run {
            if let ctx = playlist?.uri, let uri = track.uri {
                try await self.api.play(deviceID: self.deviceID, contextURI: ctx, uris: nil, offsetURI: uri, position: nil)
            } else {
                // Liked Songs / ad-hoc lists play as an explicit URI list.
                let uris = (queue.isEmpty ? [track] : queue).compactMap(\.uri)
                let start = uris.firstIndex(of: track.uri ?? "") ?? 0
                let window = Array(uris[max(0, start - 50)..<min(uris.count, start + 450)])
                let pos = window.firstIndex(of: track.uri ?? "") ?? 0
                try await self.api.play(deviceID: self.deviceID, contextURI: nil, uris: window, offsetURI: nil, position: pos)
            }
        }
    }

    func pause() { run { try await self.api.pause() } }
    func resume() { run { try await self.api.resume(deviceID: self.deviceID) } }
    func seek(to time: TimeInterval) { run { try await self.api.seek(ms: Int(time * 1000)) } }
    func next() { run { try await self.api.next() } }
    func previous() { run { try await self.api.previous() } }
    func setShuffle(_ on: Bool) { run { try await self.api.shuffle(on) } }

    func setRepeat(_ mode: RepeatMode) {
        let state = switch mode { case .off: "off"; case .all: "context"; case .one: "track" }
        run { try await self.api.repeatMode(state) }
    }

    func setVolume(_ volume: Double) {
        // Debounce slider drags.
        volumeTask?.cancel()
        volumeTask = Task {
            try? await Task.sleep(for: .milliseconds(250))
            guard !Task.isCancelled else { return }
            try? await self.api.volume(Int((volume * 100).rounded()))
        }
    }

    func setLiked(_ liked: Bool, track: Track) {
        guard track.uri != nil else { return }
        run { try await self.api.setSaved(track.id, liked) }
    }

    /// Runs a command; if the device vanished (librespot restarted), re-resolves
    /// it and retries once.
    private func run(_ op: @escaping @MainActor () async throws -> Void) {
        Task {
            do {
                try await op()
            } catch SpotifyAPIError.http(404, _) {
                if let id = await self.onDeviceLost?() {
                    self.deviceID = id
                    do { try await op() } catch { self.onError?(error.localizedDescription) }
                }
            } catch {
                self.onError?(error.localizedDescription)
            }
        }
    }
}
