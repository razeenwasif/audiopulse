package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
		// Fetch lyrics when the track changes.
		if m.state != nil && m.state.Track != nil && m.state.Track.ID != m.lyricsForID {
			t := m.state.Track
			m.lyricsForID = t.ID
			m.lyricsState = "loading"
			m.lyricsLines = nil
			m.lyricsSynced = false
			m.lyricsInstrumental = false
			cmds = append(cmds, m.loadLyricsCmd(*t))
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
		m.podcastFocus = "episodes"
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

	case deviceMsg:
		if msg.id != "" {
			m.deviceID = msg.id
			m.err = nil
			m.status = "Reconnected to the playback device."
		} else {
			m.status = "Playback device unavailable — is librespot running?"
		}
		return m, m.pollCmd(true)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleSpotifyKey(msg)
	}
	return m, nil
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

	case "/":
		m.focus = panelSearch
		return m, m.search.Focus()

	case "tab":
		return m, m.focusPanel(m.nextPanel(m.focus))

	case "shift+tab":
		return m, m.focusPanel(m.prevPanel(m.focus))

	case "up", "k":
		m.moveCursor(-1)
		return m, nil
	case "down", "j":
		m.moveCursor(1)
		return m, nil
	case "g", "home":
		m.setCursor(0)
		return m, nil
	case "G", "end":
		m.setCursor(1 << 30)
		return m, nil

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
				return m, m.loadEpisodesCmd(m.shows[m.showCursor])
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
		m.status = "Smart shuffle isn't available via the Spotify Web API (client-only feature)."
		return m, nil
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

// isDeviceError reports whether err looks like a Spotify "no active device" or
// "device not found" failure, which warrants re-resolving the librespot device.
func isDeviceError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "device") || strings.Contains(s, "no active")
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
