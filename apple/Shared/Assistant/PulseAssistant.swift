import Foundation
import SwiftUI

/// Where Ask Pulse runs.
enum AssistantChoice: String, CaseIterable, Identifiable {
    case automatic, fairy, ollama
    var id: String { rawValue }
    var title: String {
        switch self {
        case .automatic: "Automatic"
        case .fairy: "Fairy"
        case .ollama: "PC (Ollama)"
        }
    }
}

@MainActor @Observable
final class AssistantSettings {
    var choice: AssistantChoice {
        didSet { UserDefaults.standard.set(choice.rawValue, forKey: "assistantChoice") }
    }
    /// e.g. http://my-pc.tailnet.ts.net:11434 — empty means not set up.
    var ollamaURL: String {
        didSet { UserDefaults.standard.set(ollamaURL, forKey: "ollamaURL") }
    }
    var ollamaModel: String {
        didSet { UserDefaults.standard.set(ollamaModel, forKey: "ollamaModel") }
    }
    private(set) var fairyPaired = FairyPairing.load() != nil

    init() {
        let d = UserDefaults.standard
        choice = AssistantChoice(rawValue: d.string(forKey: "assistantChoice") ?? "") ?? .automatic
        var url = d.string(forKey: "ollamaURL") ?? ""
        var model = d.string(forKey: "ollamaModel") ?? ""
        #if os(macOS)
        // Same settings the terminal app reads.
        if url.isEmpty || model.isEmpty,
           let data = try? Data(contentsOf: SpotifyConfig.configFile),
           let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any] {
            if url.isEmpty { url = obj["ollama_url"] as? String ?? "" }
            if model.isEmpty { model = obj["ollama_model"] as? String ?? "" }
        }
        #endif
        ollamaURL = url
        ollamaModel = model
    }

    func pairFairy(link: String) async throws {
        #if os(iOS)
        let name = "AudioPulse"
        #else
        let name = "AudioPulse (Mac)"
        #endif
        _ = try await FairyPairing.pair(with: link, deviceName: name)
        fairyPaired = true
    }

    func forgetFairy() {
        FairyPairing.forget()
        fairyPaired = false
    }

    private var ollama: OllamaBackend? {
        let trimmed = ollamaURL.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty, let url = URL(string: trimmed.hasPrefix("http") ? trimmed : "http://\(trimmed)") else { return nil }
        return OllamaBackend(baseURL: url, model: ollamaModel.trimmingCharacters(in: .whitespaces))
    }

    private func fairy() async -> AssistantBackend? {
        #if os(macOS)
        return await FairySocketBackend.hello()
        #else
        return FairyPairing.load().map { FairyPhoneBackend(pairing: $0) }
        #endif
    }

    /// Backends to try, in order.
    func backends() async -> [AssistantBackend] {
        switch choice {
        case .fairy: return [await fairy()].compactMap { $0 }
        case .ollama: return [ollama].compactMap { $0 }
        case .automatic: return [await fairy(), ollama].compactMap { $0 }
        }
    }

    /// Why nothing can run, for the UI.
    var setupHint: String {
        #if os(macOS)
        "Open Fairy on this Mac, or add your PC’s Ollama address (ollama_url) in AudioPulse’s config."
        #else
        "Pair with Fairy, or add your PC’s Ollama address, in Assistant settings."
        #endif
    }
}

// MARK: - Plan

struct SongSuggestion: Hashable, Sendable {
    let title: String
    let artist: String
}

/// What the assistant decided, in the TUI's vocabulary plus the payload for it.
struct AssistantPlan: Sendable {
    var action: String
    var query: String
    var on: Bool
    var repeatMode: String
    var volume: Int
    var name: String
    var suggestions: [SongSuggestion]
    var answer: String
    var summary: String
}

enum PulseAssistant {
    static let instructions = """
    You turn a music request into ONE JSON object for the AudioPulse music player, and nothing else.

    Schema (include every field):
    {"action": one of "play","pause","resume","next","previous","shuffle","repeat","volume","recommend","smart_shuffle","create_playlist","organize_library","ask","unknown",
     "query": string — "play": the exact song or artist; "recommend": the vibe or seed; "create_playlist": the theme; "ask": the question; else "",
     "on": boolean — "shuffle" only: true to turn on,
     "repeat": "off" | "all" | "one" — "repeat" only,
     "volume": integer 0-100 — "volume" only,
     "name": string — "create_playlist" only: a short catchy playlist name (3–5 words),
     "suggestions": [{"title": string, "artist": string}] — real, findable songs: 12 for "recommend", 12 for "smart_shuffle", 25 for "create_playlist", else [],
     "answer": string — "ask": a friendly 1–4 sentence answer from the library details; "unknown": one sentence on what you can do; else "",
     "summary": string — one short line saying what AudioPulse will do, e.g. "Queue 12 mellow indie songs after this one"}

    Rules:
    - "play X" / "put on X" -> "play", query X (a SPECIFIC song or artist).
    - "something like X" / "recommend …" / "music for Y" -> "recommend" (a temporary queue, nothing saved).
    - "create/make/build/save a playlist of X" -> "create_playlist" (saves a real playlist).
    - "smart shuffle" / "shuffle in similar songs" / "add songs like this playlist" -> "smart_shuffle": songs that fit the current playlist but are NOT in it.
    - "organize/sort my liked songs by genre" -> "organize_library".
    - A question about their library or taste -> "ask". Never claim a song is in their library unless it's in the details.
    - "skip"/"next" -> "next"; "go back"/"previous" -> "previous"; "pause"/"stop" -> "pause"; "resume"/"continue" -> "resume".
    - "shuffle on/off" -> "shuffle" with on; "repeat"/"loop this" -> repeat "one"; "repeat all" -> "all"; "stop repeating" -> "off".
    - "set volume to N" -> "volume", volume N.
    - Suggestions: real songs by real artists that fit the request; vary artists (at most two per artist); respect era and genre constraints; use the library only as a gentle tie-breaker.
    - If the request is unclear -> "unknown".
    Output only the JSON object.
    """

    /// The request plus what's playing and a sample of the library.
    @MainActor
    static func prompt(for request: String, player: PlayerModel, library: LibraryStore) -> String {
        var lines = ["Request: \(request)", ""]
        if player.current.id != Track.nothing.id {
            lines.append("Now playing: \(player.current.title) — \(player.current.artist)" + (player.context.map { " (from \"\($0.name)\")" } ?? ""))
        }
        if let ctx = player.context, !ctx.tracks.isEmpty {
            lines.append("Current playlist (\(ctx.name)):")
            lines += ctx.tracks.prefix(40).map { "- \($0.title) — \($0.artist)" }
        }
        let liked = library.liked.tracks.prefix(50)
        if !liked.isEmpty {
            lines += ["", "Liked songs (sample of \(library.likedCount)):"]
            lines += liked.map { "- \($0.title) — \($0.artist)" }
        }
        if !library.recent.isEmpty {
            lines += ["", "Recently played:"]
            lines += library.recent.prefix(15).map { "- \($0.title) — \($0.artist)" }
        }
        if !library.playlists.isEmpty {
            lines += ["", "Their playlists: " + library.playlists.prefix(40).map(\.name).joined(separator: "; ")]
        }
        return String(lines.joined(separator: "\n").prefix(11_500))
    }

    /// Runs the request on the first backend that's reachable.
    @MainActor
    static func plan(for request: String, player: PlayerModel, library: LibraryStore, settings: AssistantSettings) async throws -> (AssistantPlan, String) {
        let backends = await settings.backends()
        guard !backends.isEmpty else { throw AssistantError.unavailable(settings.setupHint) }
        let text = prompt(for: request, player: player, library: library)
        var lastError: Error = AssistantError.unavailable(settings.setupHint)
        for backend in backends {
            do {
                let raw = try await backend.complete(instructions: instructions, prompt: text)
                return (try parse(raw), backend.label)
            } catch let error as AssistantError {
                lastError = error
                if case .unavailable = error { continue }
                throw error
            }
        }
        throw lastError
    }

    /// Reads the plan leniently: strips code fences and finds the first JSON object.
    static func parse(_ raw: String) throws -> AssistantPlan {
        guard let start = raw.firstIndex(of: "{"), let end = raw.lastIndex(of: "}"), start < end,
              let obj = try? JSONSerialization.jsonObject(with: Data(raw[start...end].utf8)) as? [String: Any]
        else { throw AssistantError.unreadable }
        let str = { (k: String) in (obj[k] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines) }
        let suggestions = (obj["suggestions"] as? [[String: Any]] ?? []).compactMap { s -> SongSuggestion? in
            let title = (s["title"] as? String ?? s["name"] as? String ?? "").trimmingCharacters(in: .whitespaces)
            let artist = (s["artist"] as? String ?? "").trimmingCharacters(in: .whitespaces)
            return title.isEmpty ? nil : SongSuggestion(title: title, artist: artist)
        }
        let action = str("action").lowercased()
        let known: Set = ["play", "pause", "resume", "next", "previous", "shuffle", "repeat", "volume", "recommend",
                          "smart_shuffle", "create_playlist", "organize_library", "ask", "unknown"]
        return AssistantPlan(
            action: known.contains(action) ? action : "unknown",
            query: str("query"), on: obj["on"] as? Bool ?? false,
            repeatMode: str("repeat").isEmpty ? "off" : str("repeat"),
            volume: max(0, min(100, (obj["volume"] as? NSNumber)?.intValue ?? 0)),
            name: str("name"), suggestions: suggestions, answer: str("answer"), summary: str("summary"))
    }
}
