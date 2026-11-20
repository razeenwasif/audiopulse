import SwiftUI

/// What the UI needs from a playback engine. The current implementation is a
/// local simulation; a Spotify-backed engine (Web API + librespot on macOS)
/// will conform to this later without touching the views.
@MainActor
protocol PlaybackBackend: AnyObject {
    func play(_ track: Track)
    func pause()
    func resume()
    func seek(to time: TimeInterval)
    func setVolume(_ volume: Double)
}

/// Simulated engine: nothing is played, the model's clock just advances.
@MainActor
final class SimulatedBackend: PlaybackBackend {
    func play(_ track: Track) {}
    func pause() {}
    func resume() {}
    func seek(to time: TimeInterval) {}
    func setVolume(_ volume: Double) {}
}

enum RepeatMode { case off, all, one }

@MainActor @Observable
final class PlayerModel {
    private(set) var current: Track = MockLibrary.lowTide
    private(set) var queue: [Track] = MockLibrary.nightDrive.tracks
    private(set) var context: Playlist? = MockLibrary.nightDrive
    var isPlaying = true
    var position: TimeInterval = 108
    var volume: Double = 0.65 { didSet { backend.setVolume(volume) } }
    var shuffle = true
    var repeatMode: RepeatMode = .off
    var isLiked = true

    private let backend: PlaybackBackend
    private var ticker: Task<Void, Never>?

    init(backend: PlaybackBackend? = nil) {
        self.backend = backend ?? SimulatedBackend()
        startTicker()
    }

    var progress: Double { current.duration > 0 ? position / current.duration : 0 }
    var lyrics: [LyricLine] { MockLibrary.lyrics(for: current) }

    var currentLyricIndex: Int {
        lyrics.lastIndex { $0.time <= position } ?? 0
    }

    var upNext: [Track] {
        guard let i = queue.firstIndex(of: current) else { return Array(queue.prefix(3)) }
        return Array(queue.dropFirst(i + 1).prefix(3))
    }

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
        backend.play(track)
    }

    func next() {
        guard let i = queue.firstIndex(of: current), !queue.isEmpty else { return }
        play(queue[(i + 1) % queue.count])
    }

    func previous() {
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

    private func startTicker() {
        ticker = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .milliseconds(250))
                guard let self, self.isPlaying else { continue }
                self.position += 0.25
                if self.position >= self.current.duration {
                    if self.repeatMode == .one { self.position = 0 } else { self.next() }
                }
            }
        }
    }
}
