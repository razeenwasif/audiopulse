import SwiftUI

/// Ask Pulse: turns a typed or spoken request into a plan (via Fairy or a PC's
/// Ollama) and carries it out with Spotify.
@MainActor @Observable
final class AskModel {
    enum Phase: Equatable { case idle, listening, thinking, results, failed }

    struct Action: Identifiable {
        enum Kind { case primary, queue, open }
        let id = UUID()
        let glyph: String
        let title: String
        let detail: String
        let shortcut: String
        let kind: Kind
    }

    var query = ""
    var phase: Phase = .idle
    var plan: AssistantPlan?
    /// Suggestions matched to Spotify tracks (or placeholders without Spotify).
    var preview: [Track] = []
    var resolving = false
    var errorMessage: String?
    /// Where the last answer came from, e.g. "Fairy (Claude)".
    var backendLabel = ""
    var highlighted = 0
    /// 0…1 microphone level while listening (drives the live wave).
    var level: Double = 0
    /// A short note after an action ran ("Saved “Rainy Indie” · 25 songs").
    var outcome: String?

    let settings = AssistantSettings()
    private let speech = SpeechListener()
    private weak var player: PlayerModel?
    private weak var library: LibraryStore?
    /// Opens a playlist (Mac: route; iPhone: push).
    var openPlaylist: ((String) -> Void)?
    private var planTask: Task<Void, Never>?

    func configure(player: PlayerModel, library: LibraryStore) {
        self.player = player
        self.library = library
    }

    var hasSpotify: Bool { library?.api != nil }

    // MARK: Asking

    func submit() {
        let request = query.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !request.isEmpty, let player, let library else { return }
        planTask?.cancel()
        phase = .thinking
        plan = nil
        preview = []
        errorMessage = nil
        outcome = nil
        highlighted = 0
        planTask = Task {
            do {
                let (plan, label) = try await PulseAssistant.plan(for: request, player: player, library: library, settings: settings)
                guard !Task.isCancelled else { return }
                self.plan = plan
                self.backendLabel = label
                if runsImmediately(plan) {
                    await perform(plan)
                    phase = .results
                    return
                }
                phase = .results
                await resolveSuggestions(plan)
            } catch is CancellationError {
                return
            } catch {
                errorMessage = error.localizedDescription
                phase = .failed
            }
        }
    }

    func reset() {
        planTask?.cancel()
        stopListening(submit: false)
        phase = .idle
        plan = nil
        preview = []
        errorMessage = nil
        outcome = nil
    }

    // MARK: Voice

    func startListening() {
        reset()
        query = ""
        phase = .listening
        Task {
            do {
                try await speech.start(onText: { [weak self] text in self?.query = text },
                                       onLevel: { [weak self] level in self?.level = level })
            } catch {
                errorMessage = error.localizedDescription
                phase = .failed
            }
        }
    }

    func stopListening(submit shouldSubmit: Bool = true) {
        guard phase == .listening else { return }
        speech.stop()
        level = 0
        if shouldSubmit, !query.trimmingCharacters(in: .whitespaces).isEmpty { submit() } else { phase = .idle }
    }

    // MARK: Actions

    /// What the result offers, built from the plan.
    var actions: [Action] {
        guard let plan else { return [] }
        let n = preview.isEmpty ? plan.suggestions.count : preview.filter { $0.uri != nil || !hasSpotify }.count
        switch plan.action {
        case "create_playlist":
            let name = plan.name.isEmpty ? "New playlist" : plan.name
            return [Action(glyph: "plus", title: "Create playlist “\(name)”", detail: "\(n) songs, saved to your Spotify", shortcut: "↵", kind: .primary),
                    Action(glyph: "play.fill", title: "Just play them", detail: "Nothing is saved", shortcut: "⌘↵", kind: .queue)]
        case "recommend":
            return [Action(glyph: "play.fill", title: "Play these \(n) songs", detail: plan.summary.isEmpty ? "A temporary queue" : plan.summary, shortcut: "↵", kind: .primary),
                    Action(glyph: "text.line.first.and.arrowtriangle.forward", title: "Add them after this song", detail: "Keeps what’s playing", shortcut: "⌘↵", kind: .queue)]
        case "smart_shuffle":
            return [Action(glyph: "shuffle", title: "Queue \(n) songs like this playlist", detail: plan.summary.isEmpty ? "Songs that fit but aren’t in it" : plan.summary, shortcut: "↵", kind: .queue),
                    Action(glyph: "plus", title: "Save them as a playlist", detail: "“\(player?.context?.name ?? "This playlist") · Smart shuffle”", shortcut: "⌘↵", kind: .primary)]
        case "play":
            return [Action(glyph: "play.fill", title: "Play “\(plan.query)”", detail: "Best match on Spotify", shortcut: "↵", kind: .primary)]
        default:
            return []
        }
    }

    func moveHighlight(_ delta: Int) {
        let count = actions.count
        guard count > 0 else { return }
        highlighted = (highlighted + delta + count) % count
    }

    /// Runs the highlighted (or given) action. Returns true when the panel can close.
    @discardableResult
    func run(_ index: Int? = nil) async -> Bool {
        guard let plan, let player, let library else { return false }
        let actions = self.actions
        guard !actions.isEmpty else { return false }
        let action = actions[min(index ?? highlighted, actions.count - 1)]
        let tracks = preview.filter { $0.uri != nil }
        guard library.api != nil else {
            outcome = "Connect Spotify to play these."
            return false
        }
        do {
            switch (plan.action, action.kind) {
            case ("play", _):
                await perform(plan)
            case ("create_playlist", .primary), ("smart_shuffle", .primary):
                let name = plan.action == "create_playlist"
                    ? (plan.name.isEmpty ? "New playlist" : plan.name)
                    : "\(player.context?.name ?? "Playlist") · Smart shuffle"
                try await savePlaylist(name: name, request: query, tracks: tracks)
            case (_, .queue) where plan.action == "smart_shuffle" || plan.action == "recommend":
                try await queue(tracks)
                outcome = "Added \(tracks.count) songs after this one"
            default:
                guard let first = tracks.first else { outcome = "None of those songs were found on Spotify."; return false }
                player.play(first, in: adHoc(plan.summary.isEmpty ? "Ask Pulse" : plan.summary, tracks))
                outcome = "Playing \(tracks.count) songs"
            }
            return true
        } catch {
            outcome = error.localizedDescription
            return false
        }
    }

    private func runsImmediately(_ plan: AssistantPlan) -> Bool {
        ["pause", "resume", "next", "previous", "shuffle", "repeat", "volume", "organize_library"].contains(plan.action)
    }

    /// Simple controls and "play X" run straight away.
    private func perform(_ plan: AssistantPlan) async {
        guard let player else { return }
        switch plan.action {
        case "pause": if player.isPlaying { player.togglePlay() }; outcome = "Paused"
        case "resume": if !player.isPlaying { player.togglePlay() }; outcome = "Playing"
        case "next": player.next(); outcome = "Skipped"
        case "previous": player.previous(); outcome = "Back one"
        case "shuffle": player.shuffle = plan.on; outcome = plan.on ? "Shuffle on" : "Shuffle off"
        case "repeat":
            player.repeatMode = plan.repeatMode == "one" ? .one : (plan.repeatMode == "all" ? .all : .off)
            outcome = "Repeat \(plan.repeatMode)"
        case "volume": player.volume = Double(plan.volume) / 100; outcome = "Volume \(plan.volume)"
        case "organize_library":
            outcome = "Sorting Liked Songs into genre playlists runs in the terminal app for now: press : there and ask the same thing."
        case "play":
            guard let api = library?.api else { outcome = "Connect Spotify to play this."; return }
            let parts = plan.query.components(separatedBy: " — ")
            if let hit = try? await api.searchTrack(title: parts[0], artist: parts.count > 1 ? parts[1] : "") {
                let track = SpotifyLibraryLoader.track(hit)
                player.play(track, in: adHoc(track.title, [track]))
                outcome = "Playing \(track.title) — \(track.artist)"
            } else {
                outcome = "Couldn’t find “\(plan.query)” on Spotify."
            }
        default: break
        }
    }

    /// Matches each suggestion to a Spotify track, a few at a time.
    private func resolveSuggestions(_ plan: AssistantPlan) async {
        let suggestions = plan.suggestions
        guard !suggestions.isEmpty else { return }
        guard let api = library?.api else {
            preview = suggestions.map(Self.placeholder)
            return
        }
        resolving = true
        defer { resolving = false }
        var found = [Track?](repeating: nil, count: suggestions.count)
        await withTaskGroup(of: (Int, Track?).self) { group in
            var next = 0
            func add() {
                guard next < suggestions.count else { return }
                let i = next, s = suggestions[i]
                next += 1
                group.addTask { @MainActor in
                    let hit = try? await api.searchTrack(title: s.title, artist: s.artist)
                    return (i, hit.map(SpotifyLibraryLoader.track))
                }
            }
            for _ in 0..<6 { add() }
            for await (i, track) in group {
                found[i] = track
                preview = found.compactMap { $0 }
                add()
            }
        }
        var seen = Set<String>()
        preview = found.compactMap { $0 }.filter { seen.insert($0.id).inserted }
    }

    private func queue(_ tracks: [Track]) async throws {
        guard let api = library?.api else { return }
        for t in tracks { if let uri = t.uri { try await api.addToQueue(uri: uri) } }
    }

    private func savePlaylist(name: String, request: String, tracks: [Track]) async throws {
        guard let api = library?.api, let userID = library?.userID, let library, let player else { return }
        guard !tracks.isEmpty else { outcome = "None of those songs were found on Spotify."; return }
        let made = try await api.createPlaylist(userID: userID, name: name,
                                                description: "Made by Pulse from “\(request)”")
        try await api.addTracks(playlistID: made.id, uris: tracks.compactMap(\.uri))
        var playlist = SpotifyLibraryLoader.playlist(made)
        playlist.tracks = tracks
        playlist.tracksLoaded = true
        playlist.reportedCount = tracks.count
        playlist.prompt = request
        playlist.updated = true
        if let cover = tracks.first?.artwork.imageURL { playlist.artwork.imageURL = cover }
        library.insertPlaylist(playlist)
        player.play(tracks[0], in: playlist)
        outcome = "Saved “\(name)” · \(tracks.count) songs"
        openPlaylist?(playlist.id)
    }

    private func adHoc(_ name: String, _ tracks: [Track]) -> Playlist {
        Playlist(id: "ask-\(UUID().uuidString.prefix(6))", name: name, initials: "AP", badgeColor: 0x1E8F62,
                 badgeOnDark: true, artwork: tracks.first?.artwork ?? .placeholder(for: name), tracks: tracks)
    }

    private static func placeholder(_ s: SongSuggestion) -> Track {
        Track(id: "s-\(s.title)-\(s.artist)", title: s.title, artist: s.artist, album: "", genre: "", duration: 0,
              artwork: .placeholder(for: s.artist + s.title))
    }
}
