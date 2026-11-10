import SwiftUI

/// What the UI needs from a playback engine. `SimulatedBackend` drives the
/// sample library; on macOS `SpotifyPlayback` plays through librespot and the
/// Spotify Web API.
@MainActor
protocol PlaybackBackend: AnyObject {
    /// True when the backend owns the queue and reports state back (Spotify).
    var isRemote: Bool { get }
    func play(_ track: Track, in playlist: Playlist?, queue: [Track])
    func pause()
    func resume()
    func seek(to time: TimeInterval)
    func setVolume(_ volume: Double)
    func next()
    func previous()
    func setShuffle(_ on: Bool)
    func setRepeat(_ mode: RepeatMode)
    func setLiked(_ liked: Bool, track: Track)
}

/// Simulated engine: nothing is played, the model's clock just advances.
@MainActor
final class SimulatedBackend: PlaybackBackend {
    var isRemote: Bool { false }
    func play(_ track: Track, in playlist: Playlist?, queue: [Track]) {}
    func pause() {}
    func resume() {}
    func seek(to time: TimeInterval) {}
    func setVolume(_ volume: Double) {}
    func next() {}
    func previous() {}
    func setShuffle(_ on: Bool) {}
    func setRepeat(_ mode: RepeatMode) {}
    func setLiked(_ liked: Bool, track: Track) {}
}

enum RepeatMode { case off, all, one }

/// A snapshot of remote player state (from the Spotify Web API).
struct RemotePlayerState {
    var track: Track?
    var isPlaying: Bool
    var position: TimeInterval
    var volume: Double?
    var shuffle: Bool
    var repeatMode: RepeatMode
    var contextURI: String?
}

@MainActor @Observable
final class PlayerModel {
    private(set) var current: Track = MockLibrary.lowTide { didSet { if oldValue.id != current.id { loadLyrics() } } }
    private(set) var queue: [Track] = MockLibrary.nightDrive.tracks
    private(set) var context: Playlist? = MockLibrary.nightDrive
    private(set) var lyrics: [LyricLine] = MockLibrary.lyrics(for: MockLibrary.lowTide)
    var isPlaying = true
    var position: TimeInterval = 108
    var volume: Double = 0.65 { didSet { if !applying { backend.setVolume(volume) } } }
    var shuffle = true { didSet { if !applying && oldValue != shuffle { backend.setShuffle(shuffle) } } }
    var repeatMode: RepeatMode = .off { didSet { if !applying && oldValue != repeatMode { backend.setRepeat(repeatMode) } } }
    var isLiked = true { didSet { if !applying && oldValue != isLiked { backend.setLiked(isLiked, track: current) } } }
    /// A short message about a playback problem (e.g. Premium required).
    var notice: String?

    private(set) var backend: PlaybackBackend
    private var ticker: Task<Void, Never>?
    private var applying = false
    private var lyricsTask: Task<Void, Never>?

    init(backend: PlaybackBackend? = nil) {
        self.backend = backend ?? SimulatedBackend()
        startTicker()
    }

    var isRemote: Bool { backend.isRemote }
    var progress: Double { current.duration > 0 ? min(1, position / current.duration) : 0 }

    var currentLyricIndex: Int {
        lyrics.lastIndex { $0.time <= position } ?? 0
    }

    var upNext: [Track] {
        guard let i = queue.firstIndex(where: { $0.id == current.id }) else { return Array(queue.prefix(3)) }
        return Array(queue.dropFirst(i + 1).prefix(3))
    }

    // MARK: Backend switching

    /// Switches to a remote backend (Spotify). The UI clears until state arrives.
    func attach(_ backend: PlaybackBackend) {
        self.backend = backend
        withoutEcho {
            current = .nothing
            queue = []
            context = nil
            isPlaying = false
            position = 0
            isLiked = false
        }
        lyrics = []
    }

    func detachToSample() {
        backend = SimulatedBackend()
        withoutEcho {
            current = MockLibrary.lowTide
            queue = MockLibrary.nightDrive.tracks
            context = MockLibrary.nightDrive
            isPlaying = true
            position = 108
            isLiked = true
        }
    }

    /// Applies state reported by the remote player without echoing it back.
    func apply(_ state: RemotePlayerState, library: LibraryStore) {
        withoutEcho {
            if let track = state.track, track.id != current.id {
                current = track
                if let ctx = state.contextURI, let p = library.playlists.first(where: { $0.uri == ctx }) {
                    context = p
                    if p.tracksLoaded { queue = p.tracks }
                }
            }
            isPlaying = state.isPlaying
            position = state.position
            if let v = state.volume { volume = v }
            shuffle = state.shuffle
            repeatMode = state.repeatMode
        }
    }

    func setLikedFromRemote(_ liked: Bool) { withoutEcho { isLiked = liked } }

    /// Fills in cover art fetched after the track started.
    func refreshArtwork(trackID: String, imageURL: URL) {
        guard current.id == trackID else { return }
        withoutEcho { current.artwork.imageURL = imageURL }
    }

    private func withoutEcho(_ body: () -> Void) {
        applying = true
        body()
        applying = false
    }

    // MARK: Transport

    func togglePlay() {
        isPlaying.toggle()
        isPlaying ? backend.resume() : backend.pause()
    }

    func play(_ track: Track, in playlist: Playlist? = nil) {
        if let playlist {
            context = playlist
            queue = playlist.tracks
        }
        current = track
        position = 0
        isPlaying = true
        backend.play(track, in: playlist, queue: queue)
    }

    func next() {
        if backend.isRemote { backend.next(); position = 0; return }
        guard let i = queue.firstIndex(of: current), !queue.isEmpty else { return }
        play(queue[(i + 1) % queue.count])
    }

    func previous() {
        if backend.isRemote { backend.previous(); return }
        if position > 3 { seek(to: 0); return }
        guard let i = queue.firstIndex(of: current), !queue.isEmpty else { return }
        play(queue[(i - 1 + queue.count) % queue.count])
    }

    func seek(to time: TimeInterval) {
        position = min(max(0, time), current.duration)
        backend.seek(to: position)
    }

    func seek(fraction: Double) { seek(to: fraction * current.duration) }

    func cycleRepeat() {
        repeatMode = switch repeatMode { case .off: .all; case .all: .one; case .one: .off }
    }

    // MARK: Lyrics

    private func loadLyrics() {
        lyricsTask?.cancel()
        let track = current
        if !backend.isRemote {
            lyrics = MockLibrary.lyrics(for: track)
            return
        }
        lyrics = []
        guard track.id != Track.nothing.id else { return }
        lyricsTask = Task {
            let lines = await LyricsService.fetch(for: track)
            guard !Task.isCancelled, self.current.id == track.id else { return }
            self.lyrics = lines ?? [LyricLine(time: 0, text: "No synced lyrics for this song.")]
        }
    }

    // MARK: Clock

    private func startTicker() {
        ticker = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .milliseconds(250))
                guard let self, self.isPlaying, self.current.duration > 0 else { continue }
                self.position = min(self.current.duration, self.position + 0.25)
                if !self.backend.isRemote, self.position >= self.current.duration {
                    if self.repeatMode == .one { self.position = 0 } else { self.next() }
                }
            }
        }
    }
}
