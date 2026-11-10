import SwiftUI

/// Placeholder album art: a gradient with an optional soft highlight.
/// Real cover images replace this once the Spotify backend is wired in.
struct Artwork: Hashable, Sendable {
    var colors: [UInt32]
    var highlight: UInt32? = nil
    var angle: Double = 160
    /// Real cover art (Spotify). The gradient shows while it loads or if it fails.
    var imageURL: URL? = nil

    /// A stable green gradient picked from an identifier, used behind real covers.
    static func placeholder(for key: String, imageURL: URL? = nil) -> Artwork {
        let palettes: [[UInt32]] = [
            [0x064E3B, 0x10B981], [0x042F2E, 0x14B8A6], [0x14532D, 0x86EFAC],
            [0x365314, 0xA3E635], [0x1C2B1F, 0x86A873], [0x134E4A, 0x5EEAD4],
        ]
        let i = abs(key.unicodeScalars.reduce(0) { ($0 &* 31) &+ Int($1.value) }) % palettes.count
        return Artwork(colors: palettes[i], angle: 135, imageURL: imageURL)
    }
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
    /// spotify:track:… when the track comes from Spotify.
    var uri: String? = nil

    static let nothing = Track(id: "nothing", title: "Nothing playing", artist: "Pick something to play",
                               album: "", genre: "", duration: 0,
                               artwork: Artwork(colors: [0x0D0E0D, 0x1E8F62], angle: 135))
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
    /// spotify:playlist:… (nil for Liked Songs, which plays as a track list).
    var uri: String? = nil
    /// Track count reported by Spotify before the tracks are loaded.
    var reportedCount: Int? = nil
    var tracksLoaded = true

    var trackCount: Int { tracksLoaded ? tracks.count : (reportedCount ?? tracks.count) }

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
