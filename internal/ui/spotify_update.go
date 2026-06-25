package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"audiopulse/internal/agent"
	"audiopulse/internal/config"
	"audiopulse/internal/downloader"
	"audiopulse/internal/spotify"
	"audiopulse/internal/voice"
)

const seekStep = 5 * time.Second
const volumeStepPct = 10

func (m Spotify) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case spotifyTickMsg:
		// Fetch the queue only when it likely changed, plus a slow keep-alive;
		// tick faster while playing, slower while paused/idle.
		m.ticksSinceQueue++
		withQueue := m.queueDirty || m.ticksSinceQueue >= queueEveryTicks
		if withQueue {
			m.queueDirty = false
			m.ticksSinceQueue = 0
		}
		return m, tea.Batch(m.pollCmd(withQueue), m.tickCmd(m.pollInterval()))

	case libraryMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "Could not load library."
			return m, nil
		}
		m.err = nil
		m.lib = msg.items
		if msg.meID != "" {
			m.meID = msg.meID
		}
		m.status = ""
		if len(m.lib) > 0 {
			return m, m.beginTrackLoad(m.lib[m.libCursor])
		}
		return m, nil

	case tracksPageMsg:
		if msg.gen != m.loadGen {
			return m, nil // a newer source switch supersedes this page
		}
		if msg.err != nil {
			m.loading = false
			m.err = msg.err
			m.status = "Could not load tracks."
			return m, nil
		}
		m.err = nil
		m.source = msg.source
		if msg.page.Offset == 0 {
			m.tracks = msg.page.Tracks
		} else {
			m.tracks = append(m.tracks, msg.page.Tracks...)
		}
		m.tracksTotal = msg.page.Total
		// Everything in the Liked Songs source is, by definition, liked.
		if msg.source.title == "Liked Songs" {
			for _, t := range msg.page.Tracks {
				if t.ID != "" {
					m.liked[t.ID] = true
				}
			}
		}
		if msg.page.HasMore() && len(m.tracks) < maxTrackItems {
			return m, m.loadTrackPageCmd(msg.item, msg.page.NextOffset(), msg.gen)
		}
		m.loading = false
		m.tracksTotal = len(m.tracks)
		return m, nil

	case playerMsg:
		if msg.state != nil {
			m.state = msg.state
		}
		if msg.hadQueue {
			m.queue = msg.queue
		}
		// A track change means the up-next queue moved on — refetch it next tick.
		if m.state != nil && m.state.Track != nil && m.state.Track.ID != m.lastTrackID {
			m.lastTrackID = m.state.Track.ID
			m.queueDirty = true
		}
		var cmds []tea.Cmd
		// Load album art when the track's cover changes (only if the right
		// panel is visible).
		if m.width >= 112 && m.state != nil && m.state.Track != nil {
			url := m.state.Track.ImageURL
			if url != "" && url != m.artURL {
				m.artURL = url
				cmds = append(cmds, m.loadArtCmd(url))
			}
		}
		// (Re)start the visualizer animation loop once playback is active.
		if m.vizActive() && !m.vizTicking {
			m.vizTicking = true
			cmds = append(cmds, m.vizTickCmd())
		}
		// Fetch lyrics and the ♥ saved-state when the track changes.
		if m.state != nil && m.state.Track != nil && m.state.Track.ID != m.lyricsForID {
			t := m.state.Track
			m.lyricsForID = t.ID
			m.lyricsState = "loading"
			m.lyricsLines = nil
			m.lyricsSynced = false
			m.lyricsInstrumental = false
			cmds = append(cmds, m.loadLyricsCmd(*t), m.checkSavedCmd(t.ID))
		}
		return m, tea.Batch(cmds...)

	case lyricsMsg:
		if msg.trackID != m.lyricsForID {
			return m, nil // track changed again; ignore stale lyrics
		}
		if msg.err != nil {
			m.lyricsState = "err"
			return m, nil
		}
		m.lyricsSynced = msg.res.Synced
		m.lyricsInstrumental = msg.res.Instrumental
		m.lyricsLines = msg.res.Lines
		switch {
		case msg.res.Instrumental:
			m.lyricsState = "instrumental"
		case len(msg.res.Lines) == 0:
			m.lyricsState = "none"
		default:
			m.lyricsState = "ready"
		}
		return m, nil

	case showsMsg:
		m.showsLoading = false
		if msg.err != nil {
			m.podErr = msg.err
			return m, nil
		}
		m.podErr = nil
		m.shows = msg.shows
		m.showsLoaded = true
		m.showCursor = clamp(m.showCursor, 0, max0(len(m.shows)-1))
		// Preview the first show's episodes so the detail box isn't empty.
		if len(m.shows) > 0 && m.currentShow.ID == "" {
			m.episodesLoading = true
			return m, m.loadEpisodesCmd(m.shows[0], false)
		}
		return m, nil

	case episodesMsg:
		m.episodesLoading = false
		if msg.err != nil {
			m.podErr = msg.err
			return m, nil
		}
		m.podErr = nil
		m.currentShow = msg.show
		m.episodes = msg.episodes
		m.episodeCursor = 0
		if msg.focus {
			m.podcastFocus = "episodes" // opened with Enter/click
		}
		return m, nil

	case episodeDebounceMsg:
		// Only the latest settled show cursor previews its episodes.
		if msg.gen == m.epLoadGen && m.focus == panelPodcasts {
			m.episodesLoading = true
			return m, m.loadEpisodesCmd(msg.show, false)
		}
		return m, nil

	case vizTickMsg:
		m.vizFrame++
		if m.vizActive() {
			return m, m.vizTickCmd()
		}
		m.vizTicking = false // pause/hidden: let the loop end, restart on resume
		return m, nil

	case artMsg:
		if msg.err == nil && msg.url == m.artURL {
			m.art = msg.art
		}
		return m, nil

	case searchDebounceMsg:
		// Only the most recent keystroke's debounce triggers a search.
		if msg.gen == m.searchGen && strings.TrimSpace(m.search.Value()) != "" {
			m.searching = true
			return m, m.spotlightSearchCmd(strings.TrimSpace(m.search.Value()), msg.gen)
		}
		return m, nil

	case spotlightResultsMsg:
		if msg.gen == m.searchGen { // ignore stale results
			m.searching = false
			if msg.err == nil {
				m.spotlightResults = msg.tracks
				m.spotlightCursor = 0
			}
		}
		return m, nil

	case actionMsg:
		if msg.err != nil {
			m.err = msg.err
			// A "device not found / no active device" error usually means
			// librespot restarted or the Connect device dropped — re-resolve it.
			if isDeviceError(msg.err) {
				m.status = "Playback device lost — reconnecting…"
				return m, m.recoverDeviceCmd()
			}
			m.status = "Playback action failed."
		}
		return m, m.pollCmd(true)

	case likeMsg:
		if msg.err != nil {
			m.liked[msg.id] = !msg.liked // revert the optimistic toggle
			m.err = msg.err
			m.status = "Couldn't update Liked Songs."
			return m, nil
		}
		m.liked[msg.id] = msg.liked
		return m, nil

	case savedCheckMsg:
		m.liked[msg.id] = msg.saved
		return m, nil

	case addedToPlaylistMsg:
		m.addBusy = false
		m.addOpen = false
		if msg.err != nil {
			m.err = msg.err
			m.status = "Couldn't add to “" + msg.playlist + "”: " + truncateErr(msg.err)
			return m, nil
		}
		if msg.liked && msg.trackID != "" {
			m.liked[msg.trackID] = true // keep the ♥ indicator in sync
		}
		m.status = "Added “" + msg.track + "” to “" + msg.playlist + "”."
		return m, nil

	case deviceMsg:
		if msg.id != "" {
			m.deviceID = msg.id
			m.err = nil
			m.status = "Reconnected to the playback device."
		} else {
			m.status = "Playback device unavailable — is librespot running?"
		}
		return m, m.pollCmd(true)

	case exportGatheredMsg:
		if m.exportState != "gathering" {
			return m, nil // cancelled while scanning
		}
		if msg.err != nil {
			m.exportState = "done"
			m.exportProg = downloader.Progress{Finished: true, Err: msg.err}
			return m, nil
		}
		m.exportURIs = msg.uris
		m.exportDir = msg.dir
		if len(m.exportURIs) == 0 {
			m.exportState = "done" // nothing to export
			return m, nil
		}
		m.exportState = "confirm"
		return m, nil

	case exportProgressMsg:
		m.exportProg = msg.p
		if msg.p.Finished {
			if m.exportCancel != nil {
				m.exportCancel()
				m.exportCancel = nil
			}
			m.exportState = "done"
			return m, nil
		}
		return m, waitExportCmd(m.exportCh)

	case agentResultMsg:
		m.agentBusy = false
		if msg.err != nil {
			m.agentErr = msg.err // keep the prompt open showing the error
			return m, nil
		}
		// Understood — close the prompt and carry out the action.
		m.focus = panelTracks
		m.ask.Blur()
		return m.runAgentCommand(msg.cmd)

	case voiceMsg:
		if msg.engine != nil {
			m.voiceEngine = msg.engine
		}
		// Mic is live now — show the cue and keep waiting for the transcript, so
		// the user only speaks once capture has actually started.
		if msg.ready {
			m.voiceReady = true
			m.status = "🎙 Listening… speak now"
			return m, waitVoiceCmd(m.voiceCh)
		}
		m.voiceListening = false
		m.voiceReady = false
		if msg.err != nil {
			m.err = msg.err
			m.status = "Voice: " + truncateErr(msg.err)
			return m, nil
		}
		text := strings.TrimSpace(msg.text)
		if text == "" {
			m.status = "🎙 Didn't catch that — press v and try again."
			return m, nil
		}
		// Feed the transcript into the same assistant pipeline as a typed request.
		m.status = "🎙 Heard: “" + text + "” — thinking…"
		return m, m.interpretCmd(text)

	case idxLoadedMsg:
		if msg.index != nil {
			m.libIndex = msg.index
		}
		return m, nil

	case idxMsg:
		// Terminal event: index built (or failed).
		if msg.index != nil || msg.err != nil {
			m.idxState = ""
			if msg.err != nil {
				m.pendingCmd = nil
				m.err = msg.err
				m.status = "Indexing failed: " + truncateErr(msg.err)
				return m, nil
			}
			m.libIndex = msg.index
			if path, err := config.LibraryIndexPath(); err == nil {
				_ = m.libIndex.Save(path)
			}
			m.status = fmt.Sprintf("Library indexed — %d tracks.", len(m.libIndex.Records))
			if m.pendingCmd != nil {
				cmd := *m.pendingCmd
				m.pendingCmd = nil
				return m.runAgentCommand(cmd)
			}
			return m, nil
		}
		// Progress.
		m.idxProg = [2]int{msg.done, msg.total}
		return m, waitIdxCmd(m.idxCh)

	case chatAnswerMsg:
		if msg.gen != m.chatGen {
			return m, nil // a newer question supersedes this answer
		}
		m.chatBusy = false
		text := strings.TrimSpace(msg.text)
		if msg.err != nil {
			text = "(couldn't answer: " + truncateErr(msg.err) + ")"
		} else if text == "" {
			text = "I'm not sure."
		}
		m.chatTurns = append(m.chatTurns, chatTurn{who: "ai", text: text})
		m.chatScroll = 1 << 30 // pin to bottom
		return m, nil

	case recommendMsg:
		m.recommending = false
		if msg.err != nil {
			m.err = msg.err
			m.status = "Recommendations failed: " + truncateErr(msg.err)
			return m, nil
		}
		if len(msg.tracks) == 0 {
			m.status = "Couldn't find anything to recommend — try a different prompt."
			return m, nil
		}
		m.tracks = msg.tracks
		title := "Recommended"
		if q := strings.TrimSpace(msg.query); q != "" {
			title = "Recommended: " + q
		}
		m.source = trackSource{title: title}
		m.trackCursor = 0
		m.centerTab = "music"
		m.focus = panelTracks
		m.status = fmt.Sprintf("%d recommendations — playing.", len(msg.tracks))
		return m, m.playSelectedCmd()

	case smartShuffleMsg:
		m.recommending = false
		if msg.err != nil {
			m.err = msg.err
			m.status = "Smart shuffle failed: " + truncateErr(msg.err)
			return m, nil
		}
		if len(msg.tracks) == 0 {
			m.status = "Smart shuffle came up empty — try a fuller or more focused playlist."
			return m, nil
		}
		m.tracks = msg.tracks
		m.source = trackSource{title: "Smart Shuffle: " + msg.source}
		m.trackCursor = 0
		m.centerTab = "music"
		m.focus = panelTracks
		m.status = fmt.Sprintf("Smart shuffle — %d fresh songs like “%s”.", len(msg.tracks), msg.source)
		return m, m.playSelectedCmd()

	case playlistCreatedMsg:
		m.recommending = false
		if msg.err != nil {
			m.err = msg.err
			m.status = "Couldn't create the playlist: " + truncateErr(msg.err)
			return m, nil
		}
		// Show the new playlist in the center, play it, and add it to the sidebar
		// (newest first) so it's browsable without a full library reload.
		m.tracks = msg.tracks
		m.source = trackSource{title: msg.name, contextURI: msg.playlistURI}
		m.trackCursor = 0
		m.centerTab = "music"
		m.focus = panelTracks
		ins := 0
		for ins < len(m.lib) && m.lib[ins].kind != libPlaylist {
			ins++
		}
		item := libItem{kind: libPlaylist, name: msg.name, plID: msg.playlistID, plURI: msg.playlistURI, count: len(msg.tracks), editable: true}
		nl := make([]libItem, 0, len(m.lib)+1)
		nl = append(nl, m.lib[:ins]...)
		nl = append(nl, item)
		nl = append(nl, m.lib[ins:]...)
		m.lib = nl
		m.status = fmt.Sprintf("Created “%s” (%d songs) — saved to your library, now playing.", msg.name, len(msg.tracks))
		return m, m.playSelectedCmd()

	case agentPlayMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "Search failed."
			return m, nil
		}
		if len(msg.tracks) == 0 {
			m.status = "No results for “" + msg.query + "”."
			return m, nil
		}
		m.tracks = msg.tracks
		m.source = trackSource{title: "Ask: " + msg.query}
		m.trackCursor = 0
		m.centerTab = "music"
		m.focus = panelTracks
		return m, m.playSelectedCmd()

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleSpotifyKey(msg)
	}
	return m, nil
}

// runAgentCommand carries out an interpreted assistant command, mirroring the
// equivalent keyboard action (and reusing m.action so playback errors trigger
// device recovery).
func (m Spotify) runAgentCommand(cmd agent.Command) (tea.Model, tea.Cmd) {
	switch cmd.Action {
	case agent.ActionPlay:
		m.status = "Looking for “" + cmd.Query + "”…"
		return m, m.agentPlayCmd(cmd.Query)
	case agent.ActionPause:
		m.status = "Pausing."
		return m, m.action(func(ctx context.Context) error { return m.client.Pause(ctx) })
	case agent.ActionResume:
		deviceID := m.deviceID
		m.status = "Resuming."
		return m, m.action(func(ctx context.Context) error { return m.client.Resume(ctx, deviceID) })
	case agent.ActionNext:
		m.status = "Skipping ahead."
		return m, m.action(func(ctx context.Context) error { return m.client.Next(ctx) })
	case agent.ActionPrevious:
		m.status = "Going back."
		return m, m.action(func(ctx context.Context) error { return m.client.Previous(ctx) })
	case agent.ActionShuffle:
		m.shuffle = cmd.On
		want := cmd.On
		if want {
			m.status = "Shuffle on."
		} else {
			m.status = "Shuffle off."
		}
		return m, m.action(func(ctx context.Context) error { return m.client.SetShuffle(ctx, want) })
	case agent.ActionRepeat:
		mode := "off"
		switch cmd.Repeat {
		case "all":
			mode = "context"
		case "one":
			mode = "track"
		}
		m.repeat = mode
		m.status = "Repeat " + cmd.Repeat + "."
		return m, m.action(func(ctx context.Context) error { return m.client.SetRepeat(ctx, mode) })
	case agent.ActionVolume:
		vol := cmd.Volume
		m.status = fmt.Sprintf("Volume %d%%.", vol)
		return m, m.action(func(ctx context.Context) error { return m.client.SetVolume(ctx, vol) })
	case agent.ActionRecommend:
		if m.libIndex == nil {
			return m.startIndex(&cmd) // build the index first, then recommend
		}
		m.recommending = true
		m.status = "Finding recommendations…"
		return m, m.recommendCmd(cmd.Query)
	case agent.ActionShuffleAI:
		return m.startSmartShuffle()
	case agent.ActionCreatePL:
		if m.recommending {
			return m, nil
		}
		m.recommending = true
		m.status = "Curating “" + cmd.Query + "” and saving it as a playlist…"
		return m, m.createPlaylistCmd(cmd.Query)
	case agent.ActionReindex:
		return m.startIndex(nil)
	case agent.ActionAsk:
		if m.libIndex == nil {
			return m.startIndex(&cmd) // build the index first, then answer
		}
		// Open a fresh chat grounded in the library and answer the question.
		m.chatOpen = true
		m.chatTurns = nil
		m.chatScroll = 0
		focus := m.chatInput.Focus()
		m2, ansCmd := m.sendChat(cmd.Query)
		return m2, tea.Batch(focus, ansCmd)
	default:
		m.status = "Sorry, I didn't catch that — try “play <song>”, “recommend …”, “shuffle on”, or “skip”."
		return m, nil
	}
}

// startSmartShuffle builds a smart shuffle from the track list the user is
// viewing: similar songs that are NOT already in it. It seeds from the current
// list (a playlist, Liked Songs, search results…) so no extra context is needed,
// and reuses the recommend spinner. Needs Ollama for the suggestions.
func (m Spotify) startSmartShuffle() (tea.Model, tea.Cmd) {
	if m.recommending {
		return m, nil // a recommend/shuffle is already in flight
	}
	if len(m.tracks) == 0 {
		m.status = "Open a playlist first, then press S to smart-shuffle in similar songs."
		return m, nil
	}
	name := strings.TrimSpace(m.source.title)
	name = strings.TrimPrefix(name, "Smart Shuffle: ") // re-shuffling a shuffle keeps the base name
	name = strings.TrimPrefix(name, "Recommended: ")
	if name == "" {
		name = "this playlist"
	}
	m.recommending = true
	m.status = "Smart shuffling “" + name + "” — finding similar songs…"
	return m, m.smartShuffleCmd(m.tracks, name)
}

// openAddToPlaylist opens the playlist picker for the selected (or playing)
// track. It needs a target track and at least one editable playlist.
func (m Spotify) openAddToPlaylist() (tea.Model, tea.Cmd) {
	id, label, ok := m.saveTarget()
	if !ok {
		m.status = "Select or play a track first, then press P to save it."
		return m, nil
	}
	// Liked Songs is always offered first (its own endpoint), then the playlists
	// the user can edit.
	lists := append([]libItem{{kind: libLiked, name: "♥ Liked Songs"}}, m.editablePlaylists()...)
	m.addOpen = true
	m.addBusy = false
	m.addCursor = 0
	m.addTrackID = id
	m.addTrackLbl = label
	m.addLists = lists
	return m, nil
}

// startIndex kicks off a background build of the library RAG index, optionally
// stashing an agent command (e.g. a recommend) to run once it's ready.
func (m Spotify) startIndex(pending *agent.Command) (tea.Model, tea.Cmd) {
	if m.idxState == "building" {
		m.pendingCmd = pending // a build is already running; queue the action
		return m, nil
	}
	m.idxState = "building"
	m.idxProg = [2]int{0, 0}
	m.pendingCmd = pending
	m.status = "Indexing your library (one-time)…"
	m.idxCh = startIndexBuild(m.client, m.agent)
	return m, waitIdxCmd(m.idxCh)
}

func (m Spotify) handleSpotifyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Spotlight search overlay: keystrokes edit the query; results update live.
	if m.focus == panelSearch {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.focus = panelTracks
			m.search.Blur()
			return m, nil
		case "up", "ctrl+p":
			if m.spotlightCursor > 0 {
				m.spotlightCursor--
			}
			return m, nil
		case "down", "ctrl+n":
			if m.spotlightCursor < len(m.spotlightResults)-1 {
				m.spotlightCursor++
			}
			return m, nil
		case "enter":
			if len(m.spotlightResults) == 0 {
				return m, nil
			}
			m.tracks = m.spotlightResults
			m.source = trackSource{title: "Search: " + strings.TrimSpace(m.search.Value())}
			m.trackCursor = clamp(m.spotlightCursor, 0, len(m.tracks)-1)
			m.focus = panelTracks
			m.search.Blur()
			return m, m.playSelectedCmd()
		}
		// Typing: update the field and debounce a live search.
		prev := m.search.Value()
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		if m.search.Value() != prev {
			m.searchGen++
			m.spotlightCursor = 0
			if strings.TrimSpace(m.search.Value()) == "" {
				m.spotlightResults = nil
				return m, cmd
			}
			return m, tea.Batch(cmd, m.searchDebounceCmd(m.searchGen))
		}
		return m, cmd
	}

	// AI assistant prompt overlay: type a request; enter sends it to the model.
	if m.focus == panelAgent {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.focus = panelTracks
			m.ask.Blur()
			m.agentBusy = false
			return m, nil
		case "enter":
			q := strings.TrimSpace(m.ask.Value())
			if q == "" || m.agentBusy {
				return m, nil
			}
			m.agentBusy = true
			m.agentErr = nil
			return m, m.interpretCmd(q)
		}
		if m.agentBusy {
			return m, nil // ignore edits while the model is thinking
		}
		var cmd tea.Cmd
		m.ask, cmd = m.ask.Update(msg)
		return m, cmd
	}

	// Library chat panel: type follow-ups; scroll the transcript; esc closes.
	if m.chatOpen {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.chatOpen = false
			m.chatInput.Blur()
			return m, nil
		case "enter":
			q := strings.TrimSpace(m.chatInput.Value())
			if q == "" || m.chatBusy {
				return m, nil
			}
			m2, cmd := m.sendChat(q)
			return m2, cmd
		case "up", "ctrl+p":
			m.scrollChat(-1)
			return m, nil
		case "down", "ctrl+n":
			m.scrollChat(1)
			return m, nil
		case "pgup":
			m.scrollChat(-max0(m.chatBodyH() - 1))
			return m, nil
		case "pgdown":
			m.scrollChat(max0(m.chatBodyH() - 1))
			return m, nil
		}
		var cmd tea.Cmd
		m.chatInput, cmd = m.chatInput.Update(msg)
		return m, cmd
	}

	// Add-to-playlist picker: navigate the list, enter saves, esc cancels.
	if m.addOpen {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "P":
			m.addOpen = false
			return m, nil
		case "up", "k", "ctrl+p":
			if m.addCursor > 0 {
				m.addCursor--
			}
			return m, nil
		case "down", "j", "ctrl+n":
			if m.addCursor < len(m.addLists)-1 {
				m.addCursor++
			}
			return m, nil
		case "g", "home":
			m.addCursor = 0
			return m, nil
		case "G", "end":
			m.addCursor = max0(len(m.addLists) - 1)
			return m, nil
		case "enter":
			if m.addBusy || m.addCursor < 0 || m.addCursor >= len(m.addLists) {
				return m, nil
			}
			pl := m.addLists[m.addCursor]
			m.addBusy = true
			m.status = "Adding to “" + pl.name + "”…"
			if pl.kind == libLiked {
				return m, m.addToLikedCmd(m.addTrackID, m.addTrackLbl)
			}
			return m, m.addToPlaylistCmd(pl.plID, m.addTrackID, pl.name, m.addTrackLbl)
		}
		return m, nil
	}

	// Library export overlay owns the keyboard while active.
	if m.exportState != "" {
		return m.handleExportKey(msg)
	}

	// Keybinding cheatsheet: any key (or ?) dismisses it.
	if m.showHelp {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m.showHelp = false
		return m, nil
	}

	// Floating full-lyrics modal: keys scroll it or close it.
	if m.lyricsModal {
		switch msg.String() {
		case "esc", "enter", "q":
			m.lyricsModal = false
		case "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.scrollLyricsModal(-1)
		case "down", "j":
			m.scrollLyricsModal(1)
		case "pgup":
			m.scrollLyricsModal(-max0(m.lyricsModalBodyH() - 1))
		case "pgdown", " ":
			m.scrollLyricsModal(max0(m.lyricsModalBodyH() - 1))
		case "g", "home":
			m.scrollLyricsModal(-(1 << 30))
		case "G", "end":
			m.scrollLyricsModal(1 << 30)
		case "f":
			m.lyricsFollow = !m.lyricsFollow // toggle synced auto-follow
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "?":
		m.showHelp = true
		return m, nil

	case "e":
		// Export the whole library (Liked Songs + playlists) to local files.
		if !downloader.Available() {
			m.status = "Exporter not installed — run 'make spotdl'."
			return m, nil
		}
		m.exportState = "gathering"
		m.status = ""
		return m, m.gatherExportCmd()

	case "/":
		m.focus = panelSearch
		return m, m.search.Focus()

	case ":":
		// Open the AI assistant prompt (local Ollama/Gemma).
		m.focus = panelAgent
		m.ask.Reset()
		m.agentBusy = false
		m.agentErr = nil
		return m, m.ask.Focus()

	case "v":
		// Voice control: speak a request (offline Vosk → assistant pipeline).
		if !voice.Available() {
			m.status = "Voice control isn't built in — run `make voice`."
			return m, nil
		}
		if m.voiceListening {
			return m, nil
		}
		m.voiceListening = true
		m.voiceReady = false
		m.status = "🎙 Starting mic…"
		m.voiceCh = startVoice(m.voiceEngine, m.voiceModel, m.voiceSource)
		return m, waitVoiceCmd(m.voiceCh)

	case "tab":
		return m, m.focusPanel(m.nextPanel(m.focus))

	case "shift+tab":
		return m, m.focusPanel(m.prevPanel(m.focus))

	case "up", "k":
		m.moveCursor(-1)
		return m, m.maybeAutoLoadEpisodes()
	case "down", "j":
		m.moveCursor(1)
		return m, m.maybeAutoLoadEpisodes()
	case "g", "home":
		m.setCursor(0)
		return m, m.maybeAutoLoadEpisodes()
	case "G", "end":
		m.setCursor(1 << 30)
		return m, m.maybeAutoLoadEpisodes()

	case "enter":
		switch m.focus {
		case panelLibrary:
			if len(m.lib) == 0 {
				return m, nil
			}
			m.focus = panelTracks
			m.centerTab = "music"
			return m, m.beginTrackLoad(m.lib[m.libCursor])
		case panelLyrics:
			m.openLyricsModal()
			return m, nil
		case panelPodcasts:
			if m.podcastFocus == "shows" {
				if m.showCursor < 0 || m.showCursor >= len(m.shows) {
					return m, nil
				}
				m.episodesLoading = true
				return m, m.loadEpisodesCmd(m.shows[m.showCursor], true)
			}
			return m, m.playEpisodeCmd()
		default:
			return m, m.playSelectedCmd()
		}

	case "esc", "backspace":
		// Back out of a show's episode list to the show list.
		if m.focus == panelPodcasts && m.podcastFocus == "episodes" {
			m.podcastFocus = "shows"
		}
		return m, nil

	case "a":
		if cmd := m.queueSelectedCmd(); cmd != nil {
			m.status = "Added to queue."
			m.queueDirty = true // reflect it in Up Next on the next poll
			return m, cmd
		}
		return m, nil

	case "L":
		// Like/unlike the selected (or playing) track.
		id, ok := m.likeTarget()
		if !ok {
			return m, nil
		}
		was := m.liked[id]
		m.liked[id] = !was // optimistic
		if was {
			m.status = "Removed from Liked Songs."
		} else {
			m.status = "♥ Saved to Liked Songs."
		}
		return m, m.toggleLikeCmd(id, was)

	case "P":
		// Add the selected (or playing) track to a playlist you pick.
		return m.openAddToPlaylist()

	case "F":
		// Unfollow the highlighted show (the podcast list is your followed shows).
		if m.focus == panelPodcasts && m.podcastFocus == "shows" && m.showCursor >= 0 && m.showCursor < len(m.shows) {
			show := m.shows[m.showCursor]
			m.status = "Unfollowed " + show.Name
			m.currentShow = spotify.Show{} // re-preview after the list refreshes
			return m, m.unfollowShowCmd(show)
		}
		return m, nil

	case " ", "p":
		return m, m.togglePlay()

	case "n":
		return m, m.action(func(ctx context.Context) error { return m.client.Next(ctx) })
	case "b":
		return m, m.action(func(ctx context.Context) error { return m.client.Previous(ctx) })

	case "right", "l":
		return m, m.seekBy(seekStep)
	case "left", "h":
		return m, m.seekBy(-seekStep)

	case "+", "=":
		return m, m.changeVolume(volumeStepPct)
	case "-", "_":
		return m, m.changeVolume(-volumeStepPct)

	case "s":
		m.shuffle = !m.shuffle // local intent → glyph turns green immediately
		want := m.shuffle
		return m, m.action(func(ctx context.Context) error { return m.client.SetShuffle(ctx, want) })
	case "S":
		return m.startSmartShuffle()
	case "r":
		return m, m.setRepeat("context")
	case "R":
		return m, m.setRepeat("track")
	}
	return m, nil
}

// setRepeat toggles the given repeat mode ("context" = loop all, "track" = loop
// one): pressing it again turns repeat off. Updates the glyph immediately.
func (m *Spotify) setRepeat(mode string) tea.Cmd {
	if m.repeat == mode {
		m.repeat = "off"
	} else {
		m.repeat = mode
	}
	target := m.repeat
	return m.action(func(ctx context.Context) error { return m.client.SetRepeat(ctx, target) })
}

func (m Spotify) togglePlay() tea.Cmd {
	deviceID := m.deviceID
	playing := m.state != nil && m.state.Playing
	return m.action(func(ctx context.Context) error {
		if playing {
			return m.client.Pause(ctx)
		}
		return m.client.Resume(ctx, deviceID)
	})
}

func (m Spotify) seekBy(delta time.Duration) tea.Cmd {
	if m.state == nil || m.state.Track == nil {
		return nil
	}
	target := m.state.Progress + delta
	if target < 0 {
		target = 0
	}
	if target > m.state.Track.Duration {
		target = m.state.Track.Duration
	}
	return m.action(func(ctx context.Context) error { return m.client.Seek(ctx, target) })
}

func (m Spotify) changeVolume(delta int) tea.Cmd {
	cur := 50
	if m.state != nil {
		cur = m.state.Volume
	}
	target := cur + delta
	return m.action(func(ctx context.Context) error { return m.client.SetVolume(ctx, target) })
}

// --- cursor helpers ----------------------------------------------------------

func (m *Spotify) moveCursor(delta int) {
	switch m.focus {
	case panelLibrary:
		m.libCursor = clamp(m.libCursor+delta, 0, len(m.lib)-1)
	case panelTracks:
		m.trackCursor = clamp(m.trackCursor+delta, 0, len(m.tracks)-1)
	case panelPodcasts:
		if m.podcastFocus == "episodes" {
			m.episodeCursor = clamp(m.episodeCursor+delta, 0, len(m.episodes)-1)
		} else {
			m.showCursor = clamp(m.showCursor+delta, 0, len(m.shows)-1)
		}
	}
}

func (m *Spotify) setCursor(pos int) {
	switch m.focus {
	case panelLibrary:
		m.libCursor = clamp(pos, 0, len(m.lib)-1)
	case panelTracks:
		m.trackCursor = clamp(pos, 0, len(m.tracks)-1)
	case panelPodcasts:
		if m.podcastFocus == "episodes" {
			m.episodeCursor = clamp(pos, 0, len(m.episodes)-1)
		} else {
			m.showCursor = clamp(pos, 0, len(m.shows)-1)
		}
	}
}

// panelCycle is the Tab focus order. Podcasts is always reachable (the center
// pane is always present); lyrics only when its panel is on screen.
func (m Spotify) panelCycle() []spotifyPanel {
	order := []spotifyPanel{panelLibrary, panelTracks, panelPodcasts}
	if m.lyricsPanelHeight() > 0 {
		order = append(order, panelLyrics)
	}
	return order
}

// focusPanel moves focus to p, syncs which center tab is active, and lazily
// loads saved shows the first time the podcasts pane is focused.
func (m *Spotify) focusPanel(p spotifyPanel) tea.Cmd {
	m.focus = p
	switch p {
	case panelTracks:
		m.centerTab = "music"
	case panelPodcasts:
		m.centerTab = "podcasts"
		if !m.showsLoaded && !m.showsLoading {
			m.showsLoading = true
			return m.loadShowsCmd()
		}
	}
	return nil
}

func (m Spotify) nextPanel(cur spotifyPanel) spotifyPanel {
	order := m.panelCycle()
	for i, p := range order {
		if p == cur {
			return order[(i+1)%len(order)]
		}
	}
	return panelLibrary
}

func (m Spotify) prevPanel(cur spotifyPanel) spotifyPanel {
	order := m.panelCycle()
	for i, p := range order {
		if p == cur {
			return order[(i-1+len(order))%len(order)]
		}
	}
	return panelLibrary
}

// openLyricsModal opens the floating lyrics pane, centering on the current
// synced line (follow mode on).
func (m *Spotify) openLyricsModal() {
	m.lyricsModal = true
	m.lyricsFollow = true
	m.lyricsScroll = m.lyricsModalStart()
}

// maybeAutoLoadEpisodes schedules a debounced preview of the highlighted show's
// episodes (without moving focus), so browsing the show list updates the detail
// box. No-op unless the shows sub-pane is focused on a different show.
func (m *Spotify) maybeAutoLoadEpisodes() tea.Cmd {
	if m.focus != panelPodcasts || m.podcastFocus != "shows" {
		return nil
	}
	if m.showCursor < 0 || m.showCursor >= len(m.shows) {
		return nil
	}
	show := m.shows[m.showCursor]
	if show.ID == m.currentShow.ID {
		return nil // already previewing this show
	}
	m.epLoadGen++
	return m.episodeDebounceCmd(show, m.epLoadGen)
}

// handleExportKey drives the export overlay's state machine.
func (m Spotify) handleExportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		if m.exportCancel != nil {
			m.exportCancel()
		}
		return m, tea.Quit
	}
	switch m.exportState {
	case "gathering":
		if msg.String() == "esc" {
			m.exportState = "" // abandon (the gather command result is ignored)
		}
	case "confirm":
		switch msg.String() {
		case "enter":
			return m.startExport()
		case "esc", "q":
			m.exportState = ""
		}
	case "running":
		if s := msg.String(); s == "esc" || s == "c" {
			if m.exportCancel != nil {
				m.exportCancel()
			}
			m.status = "Cancelling export…"
		}
	case "done":
		m.exportState = "" // any key closes the summary
	}
	return m, nil
}

// startExport kicks off the background download to the resolved output dir.
func (m Spotify) startExport() (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.exportCancel = cancel
	m.exportCh = downloader.Export(ctx, m.exportURIs, m.exportDir)
	m.exportProg = downloader.Progress{Total: len(m.exportURIs)}
	m.exportState = "running"
	return m, waitExportCmd(m.exportCh)
}

// isDeviceError reports whether err looks like a Spotify "no active device" or
// "device not found" failure, which warrants re-resolving the librespot device.
func isDeviceError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "device") || strings.Contains(s, "no active")
}

// truncateErr renders an error as a short, single-line status string.
func truncateErr(err error) string {
	s := strings.TrimSpace(err.Error())
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return truncate(s, 60)
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
