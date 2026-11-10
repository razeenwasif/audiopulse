import SwiftUI

/// Placeholder album art: a gradient with an optional soft highlight.
/// Real cover images replace this once the Spotify backend is wired in.
struct Artwork: Hashable, Sendable {
    var colors: [UInt32]
    var highlight: UInt32? = nil
    var angle: Double = 160
}

struct Track: Identifiable, Hashable, Sendable {
    let id: String
    var title: String
    var artist: String
    var album: String
    var genre: String
    var duration: TimeInterval
    var artwork: Artwork
    /// Why Pulse picked this track (only for Pulse-made playlists).
    var why: String? = nil
}

struct Playlist: Identifiable, Hashable, Sendable {
    let id: String
    var name: String
    var initials: String
    var badgeColor: UInt32
    var badgeOnDark: Bool
    var artwork: Artwork
    var tracks: [Track]
    var updated = false
    /// The request a Pulse-made playlist was built from.
    var prompt: String? = nil

    var totalDuration: TimeInterval { tracks.reduce(0) { $0 + $1.duration } }
}

struct LyricLine: Identifiable, Hashable, Sendable {
    var id: TimeInterval { time }
    let time: TimeInterval
    let text: String
}

struct Genre: Identifiable, Hashable, Sendable {
    var id: String { name }
    let name: String
    let count: Int
    let color: UInt32
}

extension TimeInterval {
    /// 4:16
    var clock: String {
        let s = Int(self.rounded(.down))
        return "\(s / 60):" + String(format: "%02d", s % 60)
    }

    /// 1 h 38 m
    var longDuration: String {
        let m = Int(self) / 60
        return m >= 60 ? "\(m / 60) h \(m % 60) m" : "\(m) m"
    }
}
