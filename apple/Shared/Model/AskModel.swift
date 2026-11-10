import SwiftUI

/// The Ask Pulse assistant. Today it returns canned results so the UI can be
/// exercised; it will call the same local model the TUI uses (Ollama) later.
@MainActor @Observable
final class AskModel {
    enum Phase: Equatable { case idle, listening, thinking, results }

    struct Action: Identifiable {
        let id = UUID()
        let glyph: String
        let title: String
        let detail: String
        let shortcut: String
    }

    enum Filter: String, CaseIterable { case all = "All", songs = "Songs", artists = "Artists", playlists = "Playlists", ask = "Ask" }

    var query = "rainy day indie, nothing too loud"
    var phase: Phase = .results
    var filter: Filter = .all
    var highlighted = 0
    /// 0…1 microphone level while listening (drives the live wave).
    var level: Double = 0

    private var listenTask: Task<Void, Never>?

    var actions: [Action] {
        [
            Action(glyph: "plus", title: "Create playlist “Rainy Indie”", detail: "25 songs picked from your Liked Songs and playlists", shortcut: "↵"),
            Action(glyph: "text.line.first.and.arrowtriangle.forward", title: "Queue 10 songs like this after Low Tide", detail: "Keeps the current mood, lowers the energy", shortcut: "⌘↵"),
            Action(glyph: "shuffle", title: "Smart shuffle Night Drive toward this mood", detail: "Reorders the playlist; nothing is added or removed", shortcut: "⌥↵"),
        ]
    }

    var preview: [Track] { Array(MockLibrary.rainyIndie.tracks.prefix(4)) }

    func submit() {
        phase = .thinking
        Task {
            try? await Task.sleep(for: .milliseconds(700))
            phase = .results
            highlighted = 0
        }
    }

    func startListening() {
        phase = .listening
        query = ""
        let words = ["rainy", "day", "indie,", "nothing", "too", "loud"]
        listenTask?.cancel()
        listenTask = Task {
            for (i, word) in words.enumerated() {
                for _ in 0..<4 {
                    try? await Task.sleep(for: .milliseconds(80))
                    if Task.isCancelled { return }
                    level = Double.random(in: 0.35...1)
                }
                query += (i == 0 ? "" : " ") + word
            }
            level = 0.1
        }
    }

    func stopListening() {
        listenTask?.cancel()
        level = 0
        if query.isEmpty { query = "rainy day indie, nothing too loud" }
        submit()
    }

    func moveHighlight(_ delta: Int) {
        highlighted = (highlighted + delta + actions.count) % actions.count
    }
}
