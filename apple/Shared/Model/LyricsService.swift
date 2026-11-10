import Foundation

/// Synced lyrics from LRCLIB (free, no auth), the same source the TUI uses.
enum LyricsService {
    private static let userAgent = "AudioPulse (https://github.com/razeenwasif/audiopulse)"

    static func fetch(for track: Track) async -> [LyricLine]? {
        guard track.duration > 0 else { return nil }
        var comps = URLComponents(string: "https://lrclib.net/api/get")!
        comps.queryItems = [
            .init(name: "artist_name", value: track.artist.components(separatedBy: ", ").first ?? track.artist),
            .init(name: "track_name", value: track.title),
            .init(name: "album_name", value: track.album),
            .init(name: "duration", value: "\(Int(track.duration.rounded()))"),
        ]
        var req = URLRequest(url: comps.url!)
        req.setValue(userAgent, forHTTPHeaderField: "User-Agent")
        req.timeoutInterval = 8
        guard let (data, resp) = try? await URLSession.shared.data(for: req),
              (resp as? HTTPURLResponse)?.statusCode == 200 else { return nil }
        struct Body: Decodable { let syncedLyrics: String?; let plainLyrics: String?; let instrumental: Bool? }
        guard let body = try? JSONDecoder().decode(Body.self, from: data) else { return nil }
        if body.instrumental == true { return [LyricLine(time: 0, text: "♪ Instrumental")] }
        if let synced = body.syncedLyrics, !synced.isEmpty { return parseLRC(synced) }
        if let plain = body.plainLyrics, !plain.isEmpty {
            return plain.split(separator: "\n").enumerated().map { LyricLine(time: Double($0.offset) * 4, text: String($0.element)) }
        }
        return nil
    }

    /// Parses "[mm:ss.xx] line" LRC text.
    static func parseLRC(_ text: String) -> [LyricLine] {
        var lines: [LyricLine] = []
        for raw in text.split(separator: "\n") {
            guard raw.hasPrefix("["), let close = raw.firstIndex(of: "]") else { continue }
            let stamp = raw[raw.index(after: raw.startIndex)..<close]
            let parts = stamp.split(separator: ":")
            guard parts.count == 2, let m = Double(parts[0]), let s = Double(parts[1]) else { continue }
            let words = raw[raw.index(after: close)...].trimmingCharacters(in: .whitespaces)
            lines.append(LyricLine(time: m * 60 + s, text: words.isEmpty ? "♪" : words))
        }
        // Keep ids (times) unique for ForEach.
        var seen = Set<TimeInterval>()
        return lines.sorted { $0.time < $1.time }.filter { seen.insert($0.time).inserted }
    }
}
