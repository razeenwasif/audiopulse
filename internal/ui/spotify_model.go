package ui

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zspotify "github.com/zmb3/spotify/v2"

	"audiopulse/internal/agent"
	"audiopulse/internal/config"
	"audiopulse/internal/downloader"
	"audiopulse/internal/library"
	"audiopulse/internal/lyrics"
	"audiopulse/internal/spotify"
	"audiopulse/internal/voice"
)

// spotifyPanel identifies the focused panel.
type spotifyPanel int

const (
	panelLibrary spotifyPanel = iota
	panelTracks
	panelPodcasts
	panelLyrics
	panelSearch
	panelAgent
)

// centerSplitMin is the minimum center outer width to show music and podcasts
// side by side; below it the center is a single pane with a Music/Podcasts
// toggle (the chips).
const centerSplitMin = 64

const spotifySidebarWidth = 32
const spotifyRightWidth = 34

// maxPlayURIs bounds the explicit URI list sent to Spotify's play endpoint for
// context-less sources (Liked Songs / Recent / Search). The endpoint rejects
// very large arrays, so playback starts a window of this many tracks from the
// selection.
const maxPlayURIs = 500

// maxTrackItems caps how many tracks the background page-streamer will collect
// for one source, a safety bound against pathologically large playlists.
const maxTrackItems = 10000

// libKind distinguishes special library entries from user playlists.
type libKind int

const (
	libLiked libKind = iota
	libRecent
	libPlaylist
)

// libItem is a row in the left library panel.
type libItem struct {
	kind  libKind
	name  string
	plID  zspotify.ID
	plURI zspotify.URI
	count int
}

// trackSource describes what the center list currently shows.
type trackSource struct {
	title      string
	contextURI zspotify.URI // non-empty for playlists/albums
}

// Spotify is the Bubble Tea model for the full-song, Spotify-backed UI.
type Spotify struct {
	st       styles
	client   *spotify.Client
	deviceID string
	user     string

	width, height int

	lib       []libItem
	libCursor int

	tracks      []spotify.Track
	trackCursor int
	source      trackSource
	loading     bool
	tracksTotal int // expected total for the loading source (for progress)
	loadGen     int // bumped per source switch; stale pages are dropped

	// Podcasts (center-right pane / "Podcasts" tab).
	centerTab       string // "music" | "podcasts" — active pane in single-column mode
	shows           []spotify.Show
	showCursor      int
	showsLoaded     bool
	showsLoading    bool
	podcastFocus    string // "shows" | "episodes"
	currentShow     spotify.Show
	episodes        []spotify.Episode
	episodeCursor   int
	episodesLoading bool
	epLoadGen       int // debounce generation for auto-preview of episodes
	podErr          error

	state *spotify.PlayerState
	queue []spotify.Track

	// Queue polling: the up-next queue is fetched only when it likely changed
	// (track change / explicit action) plus a slow keep-alive, not every tick.
	queueDirty      bool
	ticksSinceQueue int
	lastTrackID     zspotify.ID // detects track changes to refresh the queue

	// liked caches which track ids are in Liked Songs (for the ♥ indicator and
	// toggle logic); not exhaustive — filled from the playing track, the Liked
	// Songs list, and like/unlike actions.
	liked map[zspotify.ID]bool

	// Shuffle/repeat are tracked locally and start off. The Web API doesn't
	// reliably report these for a librespot device, so seeding from it flipped the
	// glyphs the wrong way — the user's keypresses are the single source of truth.
	shuffle bool
	repeat  string // "off" | "context" | "track"

	art        string // rendered album art for the current track
	artURL     string // image URL the art was rendered from
	artW, artH int    // art size in cells (height derived from cell aspect)

	// Visualizer (CAVA-style). AudioPulse can't see librespot's PCM, so the
	// spectrum is a procedural animation driven by playback: vizFrame advances on
	// a fast tick while a track plays, and decays to a flat baseline when paused.
	vizFrame   int
	vizTicking bool // a vizTick loop is currently scheduled (avoids duplicates)

	// Lyrics for the current track (from lrclib.net; see internal/lyrics).
	lyricsLines        []lyrics.Line
	lyricsSynced       bool
	lyricsInstrumental bool
	lyricsState        string      // "" | loading | ready | none | instrumental | err
	lyricsForID        zspotify.ID // the track the lyrics are for/loading

	// Floating keybinding cheatsheet (toggled with ?).
	showHelp bool

	// Library export (spotDL). exportState: "" | gathering | confirm | running | done.
	exportState  string
	exportURIs   []string
	exportDir    string
	exportProg   downloader.Progress
	exportCancel context.CancelFunc
	exportCh     <-chan downloader.Progress

	// Floating full-lyrics modal (opened with Enter on the focused lyrics panel).
	lyricsModal  bool
	lyricsScroll int  // top display-line index when manually scrolled
	lyricsFollow bool // auto-follow the current synced line until the user scrolls

	search textinput.Model

	// Spotlight-style search overlay.
	spotlightResults []spotify.Track
	spotlightCursor  int
	searchGen        int // debounce/staleness generation
	searching        bool

	// AI assistant (local Ollama/Gemma): a floating prompt (opened with ':')
	// turns a natural-language request into a playback command.
	agent     *agent.Client
	ask       textinput.Model
	agentBusy bool  // an Interpret call is in flight
	agentErr  error // last assistant error, shown in the overlay

	// Multi-turn library chat (opened by an "ask" request). Grounded in libIndex.
	chatOpen   bool
	chatTurns  []chatTurn
	chatInput  textinput.Model
	chatBusy   bool // an Answer call is in flight
	chatScroll int  // top display-line of the transcript (1<<30 = pinned bottom)
	chatGen    int  // staleness for in-flight answers

	// Library RAG index (recommendations + library chat). Loaded from disk on
	// startup if present; built on demand (with a progress overlay) when the
	// assistant needs it. pendingCmd is an agent command waiting for the build.
	libIndex     *library.Index
	idxState     string // "" | "building"
	idxProg      [2]int // done, total during a build
	idxCh        <-chan idxEvent
	pendingCmd   *agent.Command // recommend/ask to run once the index is built
	recommending bool

	// Voice control (offline Vosk speech-to-text; `v` to talk). The engine loads
	// its model lazily on first use and is reused; a spoken phrase is transcribed
	// and fed into the same assistant pipeline as a typed request.
	voiceEngine    *voice.Engine
	voiceListening bool              // a capture is in progress (starting or live)
	voiceReady     bool              // capture is live — ok to speak
	voiceCh        <-chan voiceEvent // events from the active capture
	voiceModel     string            // config path; empty → package default
	voiceSource    string            // PulseAudio source; empty → "default"

	focus  spotifyPanel
	status string
	err    error
}

// NewSpotify builds the Spotify UI model. cellAspect is the terminal cell
// height/width ratio, used to keep album art square.
func NewSpotify(client *spotify.Client, deviceID, user string, cellAspect float64) Spotify {
	ti := textinput.New()
	ti.Placeholder = "Search Spotify…"
	ti.Prompt = "🔎 "
	ti.CharLimit = 80
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorAccent)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorAccent)

	cfg := config.Load()
	ask := textinput.New()
	ask.Placeholder = "play X · recommend like X · ask about your library · skip"
	ask.Prompt = "✦ "
	ask.CharLimit = 120
	ask.PromptStyle = lipgloss.NewStyle().Foreground(colorAccent)
	ask.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	ask.Cursor.Style = lipgloss.NewStyle().Foreground(colorAccent)

	chatInput := textinput.New()
	chatInput.Placeholder = "ask a follow-up…"
	chatInput.Prompt = "› "
	chatInput.CharLimit = 200
	chatInput.PromptStyle = lipgloss.NewStyle().Foreground(colorAccent)
	chatInput.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	chatInput.Cursor.Style = lipgloss.NewStyle().Foreground(colorAccent)

	w, h := artDims(cellAspect)
	return Spotify{
		st:           newStyles(),
		client:       client,
		deviceID:     deviceID,
		user:         user,
		search:       ti,
		ask:          ask,
		chatInput:    chatInput,
		agent:        agent.New(cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaEmbedModel),
		voiceModel:   cfg.VoiceModel,
		voiceSource:  cfg.VoiceSource,
		artW:         w,
		artH:         h,
		repeat:       "off",
		focus:        panelLibrary,
		centerTab:    "music",
		podcastFocus: "shows",
		showsLoading: true, // Init fetches saved shows up front
		queueDirty:   true, // fetch the up-next queue on the first tick
		liked:        make(map[zspotify.ID]bool),
		status:       "Loading your library…",
	}
}

// artDims picks album-art cell dimensions so a square cover displays square:
// width·cellW == height·cellH, i.e. height = width / (cellH/cellW).
func artDims(cellAspect float64) (w, h int) {
	if cellAspect <= 0 {
		cellAspect = 2.0
	}
	w = artCellW
	h = int(math.Round(float64(w) / cellAspect))
	if h < 6 {
		h = 6
	}
	if h > 16 {
		h = 16
	}
	return w, h
}

func (m Spotify) Init() tea.Cmd {
	// Load saved podcasts up front too, so the podcast pane is populated even in
	// the side-by-side layout where it's visible without being focused.
	return tea.Batch(m.loadLibraryCmd(), m.loadShowsCmd(), loadIndexCmd(), m.tickCmd(pollPlaying), textinput.Blink)
}

// --- messages ---------------------------------------------------------------

type libraryMsg struct {
	items []libItem
	err   error
}

// tracksPageMsg delivers one streamed page of the center track list. Pages for
// a source arrive in order; gen guards against a source switch mid-stream.
type tracksPageMsg struct {
	gen    int
	source trackSource
	item   libItem // carried so the next page can be requested
	page   spotify.TrackPage
	err    error
}
type playerMsg struct {
	state    *spotify.PlayerState
	queue    []spotify.Track
	hadQueue bool // whether queue was fetched this poll (else leave it unchanged)
}
type showsMsg struct {
	shows []spotify.Show
	err   error
}
type episodesMsg struct {
	show     spotify.Show
	episodes []spotify.Episode
	focus    bool // move focus to the episodes box (Enter/click), vs just preview
	err      error
}

// episodeDebounceMsg fires after the show cursor settles, to preview that show's
// episodes without an API call per keystroke.
type episodeDebounceMsg struct {
	gen  int
	show spotify.Show
}
type actionMsg struct{ err error }
type deviceMsg struct{ id string } // recovered librespot device id ("" = not found)
type likeMsg struct {
	id    zspotify.ID
	liked bool // intended new state
	err   error
}
type savedCheckMsg struct {
	id    zspotify.ID
	saved bool
}
type searchDebounceMsg struct{ gen int }
type spotlightResultsMsg struct {
	gen    int
	query  string
	tracks []spotify.Track
	err    error
}
type artMsg struct {
	url string
	art string
	err error
}
type spotifyTickMsg time.Time
type vizTickMsg time.Time
type exportGatheredMsg struct {
	uris []string
	dir  string
	err  error
}
type exportProgressMsg struct{ p downloader.Progress }
type lyricsMsg struct {
	trackID zspotify.ID
	res     lyrics.Result
	err     error
}

// agentResultMsg delivers the assistant's interpretation of an utterance.
type agentResultMsg struct {
	utterance string
	cmd       agent.Command
	err       error
}

// agentPlayMsg carries search results for an assistant "play X" request.
type agentPlayMsg struct {
	query  string
	tracks []spotify.Track
	err    error
}

// voiceEvent is one step of a voice capture: a ready signal (mic live) or a
// final result (transcript / error). The opened engine is carried back so it is
// reused across calls.
type voiceEvent struct {
	ready  bool
	engine *voice.Engine
	text   string
	err    error
}

// voiceMsg delivers a voiceEvent into the Bubble Tea update loop.
type voiceMsg voiceEvent

// idxLoadedMsg carries the library index loaded from disk at startup (if any).
type idxLoadedMsg struct{ index *library.Index }

// idxEvent is one step of a background index build: progress (done/total) or a
// terminal event carrying the finished index or an error.
type idxEvent struct {
	done, total int
	index       *library.Index
	err         error
}

// idxMsg delivers an idxEvent into the update loop.
type idxMsg idxEvent

// recommendMsg carries resolved recommendation tracks (LLM suggestions → search).
type recommendMsg struct {
	query  string
	tracks []spotify.Track
	err    error
}

// chatTurn is one line of the library-chat transcript.
type chatTurn struct {
	who  string // "you" | "ai"
	text string
}

// chatAnswerMsg delivers a grounded answer for the chat panel.
type chatAnswerMsg struct {
	gen  int
	text string
	err  error
}

// --- commands ---------------------------------------------------------------

// Poll cadences: fast while playing (to advance the progress bar), slow while
// paused/idle (nothing moves). The heavier Queue() call is decoupled from this —
// see pollCmd / queueEveryTicks.
const (
	pollPlaying     = time.Second
	pollPaused      = 4 * time.Second
	queueEveryTicks = 20 // keep-alive: refetch the queue at most this often
)

// pollInterval picks the tick cadence for the current playback state.
func (m Spotify) pollInterval() time.Duration {
	if m.isPlaying() {
		return pollPlaying
	}
	return pollPaused
}

func (m Spotify) tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return spotifyTickMsg(t) })
}

// vizFrameRate is the visualizer animation cadence (~8 fps). It only runs while
// a track is playing and the right panel is visible, so the cost is bounded.
const vizFrameRate = 120 * time.Millisecond

func (m Spotify) vizTickCmd() tea.Cmd {
	return tea.Tick(vizFrameRate, func(t time.Time) tea.Msg { return vizTickMsg(t) })
}

// isPlaying reports whether a track is actively playing.
func (m Spotify) isPlaying() bool { return m.state != nil && m.state.Playing }

// vizLevels produces n spectrum bar heights in [0,1]. While playing it layers a
// few sine waves (advanced by vizFrame) under a bass-weighted envelope for an
// organic, CAVA-like spectrum; when paused it returns a flat low baseline. This
// is a synthesized animation — AudioPulse has no access to the decoded audio.
func (m Spotify) vizLevels(n int) []float64 {
	levels := make([]float64, n)
	if !m.isPlaying() {
		for i := range levels {
			levels[i] = 0.04 // flat resting line
		}
		return levels
	}
	denom := float64(n - 1)
	if denom < 1 {
		denom = 1
	}
	t := float64(m.vizFrame) * 0.35
	for i := 0; i < n; i++ {
		x := float64(i)
		v := 0.5 + 0.5*math.Sin(t+x*0.45)
		v *= 0.55 + 0.45*math.Sin(t*0.6+x*0.9+1.3)
		v += 0.15 * math.Sin(t*1.7+x*0.2)
		// Bass-weighted envelope: fuller toward the left, tapering right.
		v *= 0.55 + 0.45*math.Cos(math.Pi*x/denom)
		switch {
		case v < 0.05:
			v = 0.05 // small floor so bars never fully vanish while playing
		case v > 1:
			v = 1
		}
		levels[i] = v
	}
	return levels
}

// vizActive reports whether the visualizer should be animating right now.
func (m Spotify) vizActive() bool { return m.isPlaying() && m.width >= 112 }

func (m Spotify) loadLibraryCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		playlists, err := client.Playlists(ctx)
		if err != nil {
			return libraryMsg{err: err}
		}
		items := []libItem{
			{kind: libLiked, name: "Liked Songs"},
			{kind: libRecent, name: "Recently Played"},
		}
		for _, p := range playlists {
			items = append(items, libItem{
				kind:  libPlaylist,
				name:  p.Name,
				plID:  p.ID,
				plURI: p.URI,
				count: p.Count,
			})
		}
		return libraryMsg{items: items}
	}
}

// beginTrackLoad clears the center list and kicks off streaming the first page
// of a library item. Each source switch bumps loadGen so pages still in flight
// for the previous source are ignored when they arrive.
func (m *Spotify) beginTrackLoad(item libItem) tea.Cmd {
	m.loadGen++
	m.tracks = nil
	m.trackCursor = 0
	m.tracksTotal = 0
	m.loading = true
	return m.loadTrackPageCmd(item, 0, m.loadGen)
}

// loadTrackPageCmd fetches a single page of a source's tracks at the given
// offset. The update loop appends it and requests the next page until the
// source is fully loaded.
func (m Spotify) loadTrackPageCmd(item libItem, offset, gen int) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var (
			page spotify.TrackPage
			err  error
			src  trackSource
		)
		switch item.kind {
		case libLiked:
			src = trackSource{title: "Liked Songs"}
			page, err = client.LikedSongsPage(ctx, offset)
		case libRecent:
			// Recently Played isn't offset-paginated; fetch the single page.
			src = trackSource{title: "Recently Played"}
			var tracks []spotify.Track
			tracks, err = client.RecentlyPlayed(ctx)
			page = spotify.TrackPage{Tracks: tracks, Total: len(tracks)}
		default:
			src = trackSource{title: item.name, contextURI: item.plURI}
			page, err = client.PlaylistTracksPage(ctx, item.plID, offset)
		}
		return tracksPageMsg{gen: gen, source: src, item: item, page: page, err: err}
	}
}

func (m Spotify) loadShowsCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		shows, err := client.SavedShows(ctx)
		return showsMsg{shows: shows, err: err}
	}
}

func (m Spotify) loadEpisodesCmd(show spotify.Show, focus bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		eps, err := client.ShowEpisodes(ctx, show.ID)
		return episodesMsg{show: show, episodes: eps, focus: focus, err: err}
	}
}

// episodeDebounceCmd previews a highlighted show's episodes after a short pause.
func (m Spotify) episodeDebounceCmd(show spotify.Show, gen int) tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
		return episodeDebounceMsg{gen: gen, show: show}
	})
}

// playEpisodeCmd plays the selected episode and continues through the rest of
// the show's loaded episode list (bounded, like context-less music sources).
func (m Spotify) playEpisodeCmd() tea.Cmd {
	if m.episodeCursor < 0 || m.episodeCursor >= len(m.episodes) {
		return nil
	}
	client := m.client
	deviceID := m.deviceID
	pos := m.episodeCursor
	episodes := m.episodes
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		end := pos + maxPlayURIs
		if end > len(episodes) {
			end = len(episodes)
		}
		window := episodes[pos:end]
		uris := make([]zspotify.URI, len(window))
		for i, e := range window {
			uris[i] = e.URI
		}
		return actionMsg{err: client.PlayTracksAt(ctx, deviceID, uris, 0)}
	}
}

// loadLyricsCmd fetches lyrics for a track from lrclib.net. It passes the
// primary artist (lrclib's exact match dislikes "A, B" multi-artist strings).
func (m Spotify) loadLyricsCmd(t spotify.Track) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		artist := strings.TrimSpace(strings.SplitN(t.Artist, ",", 2)[0])
		res, err := lyrics.Fetch(ctx, artist, t.Title, t.Album, t.Duration)
		return lyricsMsg{trackID: t.ID, res: res, err: err}
	}
}

// currentLyricLine returns the index of the latest synced line whose timestamp
// has been reached, or -1 if playback is before the first line.
func currentLyricLine(lines []lyrics.Line, progress time.Duration) int {
	cur := -1
	for i := range lines {
		if lines[i].At <= progress {
			cur = i
		} else {
			break
		}
	}
	return cur
}

// lyricsStateText is the placeholder shown for a non-"ready" lyrics state.
func lyricsStateText(state string) string {
	switch state {
	case "loading":
		return "Loading lyrics…"
	case "none":
		return "No lyrics found"
	case "instrumental":
		return "♪  instrumental"
	case "err":
		return "Lyrics unavailable"
	default:
		return "—"
	}
}

// lyricDisp is one rendered (word-wrapped) line of the lyrics modal.
type lyricDisp struct {
	text    string
	current bool // part of the current synced line
}

// wrapText word-wraps s to width cells, hard-splitting any single word longer
// than the width. Returns at least one (possibly empty) line.
func wrapText(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	cur := ""
	for _, w := range strings.Fields(s) {
		for lipgloss.Width(w) > width {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			r := []rune(w)
			lines = append(lines, string(r[:width]))
			w = string(r[width:])
		}
		switch {
		case cur == "":
			cur = w
		case lipgloss.Width(cur)+1+lipgloss.Width(w) <= width:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

// lyricsModalWidth is the outer width of the floating lyrics modal.
func (m Spotify) lyricsModalWidth() int { return clamp(m.width*2/3, 50, 96) }

// lyricsModalBodyH is how many lyric rows the modal can show at once.
func (m Spotify) lyricsModalBodyH() int {
	// box height = body + border(2) + padding(2) + header + sep + blank + hint(4).
	bh := m.height - 10
	if bh < 1 {
		bh = 1
	}
	return bh
}

// lyricsModalLines builds the word-wrapped display lines for the modal at the
// given inner width, plus the index of the current synced line's first row
// (-1 when none / not synced / not ready).
func (m Spotify) lyricsModalLines(innerW int) ([]lyricDisp, int) {
	if m.lyricsState != "ready" {
		return []lyricDisp{{text: lyricsStateText(m.lyricsState)}}, -1
	}
	cur := -1
	if m.lyricsSynced && m.state != nil {
		cur = currentLyricLine(m.lyricsLines, m.state.Progress)
	}
	var disp []lyricDisp
	curDisp := -1
	for i := range m.lyricsLines {
		text := m.lyricsLines[i].Text
		if strings.TrimSpace(text) == "" {
			disp = append(disp, lyricDisp{})
			continue
		}
		for _, wl := range wrapText(text, innerW) {
			if i == cur && curDisp < 0 {
				curDisp = len(disp)
			}
			disp = append(disp, lyricDisp{text: wl, current: i == cur})
		}
	}
	return disp, curDisp
}

// lyricsModalStart is the effective top display-line index for the modal: it
// auto-centers the current synced line while following, else uses the manual
// scroll position. Always clamped to a valid range.
func (m Spotify) lyricsModalStart() int {
	disp, curDisp := m.lyricsModalLines(m.lyricsModalWidth() - 4)
	bodyH := m.lyricsModalBodyH()
	maxStart := max0(len(disp) - bodyH)
	start := m.lyricsScroll
	if m.lyricsFollow && m.lyricsSynced && curDisp >= 0 {
		start = curDisp - bodyH/2
	}
	return clamp(start, 0, maxStart)
}

// scrollLyricsModal moves the modal viewport by delta rows and stops following.
func (m *Spotify) scrollLyricsModal(delta int) {
	from := m.lyricsModalStart()
	m.lyricsFollow = false
	disp, _ := m.lyricsModalLines(m.lyricsModalWidth() - 4)
	maxStart := max0(len(disp) - m.lyricsModalBodyH())
	m.lyricsScroll = clamp(from+delta, 0, maxStart)
}

// --- chat panel layout -------------------------------------------------------

// chatDisp is one rendered (wrapped) line of the chat transcript.
type chatDisp struct {
	text string
	who  string // "you" | "ai" | "" (blank spacer)
}

func (m Spotify) chatModalWidth() int { return clamp(m.width*2/3, 50, 96) }

// chatBodyH is how many transcript rows the panel shows at once.
func (m Spotify) chatBodyH() int {
	// box = border(2)+padding(2)+header+sep+body+sep+input+hint.
	bh := m.height - 12
	if bh < 3 {
		bh = 3
	}
	return bh
}

// chatLines flattens the transcript into wrapped display lines at inner width w.
func (m Spotify) chatLines(w int) []chatDisp {
	if w < 8 {
		w = 8
	}
	var out []chatDisp
	for i, t := range m.chatTurns {
		if i > 0 {
			out = append(out, chatDisp{}) // blank line between turns
		}
		label := "You"
		if t.who == "ai" {
			label = "AI"
		}
		pad := strings.Repeat(" ", len(label)+2)
		for j, wl := range wrapText(t.text, w-len(label)-2) {
			if j == 0 {
				out = append(out, chatDisp{text: label + ": " + wl, who: t.who})
			} else {
				out = append(out, chatDisp{text: pad + wl, who: t.who})
			}
		}
	}
	if m.chatBusy {
		if len(out) > 0 {
			out = append(out, chatDisp{})
		}
		out = append(out, chatDisp{text: "AI: …thinking", who: "ai"})
	}
	return out
}

// chatStart is the clamped top display-line index (1<<30 pins to the bottom).
func (m Spotify) chatStart() int {
	disp := m.chatLines(m.chatModalWidth() - 4)
	maxStart := max0(len(disp) - m.chatBodyH())
	return clamp(m.chatScroll, 0, maxStart)
}

// scrollChat moves the transcript viewport by delta rows.
func (m *Spotify) scrollChat(delta int) {
	from := m.chatStart()
	disp := m.chatLines(m.chatModalWidth() - 4)
	maxStart := max0(len(disp) - m.chatBodyH())
	m.chatScroll = clamp(from+delta, 0, maxStart)
}

// searchDebounceCmd fires after a short pause so we search once the user stops
// typing, not on every keystroke.
func (m Spotify) searchDebounceCmd(gen int) tea.Cmd {
	return tea.Tick(280*time.Millisecond, func(time.Time) tea.Msg {
		return searchDebounceMsg{gen: gen}
	})
}

func (m Spotify) spotlightSearchCmd(query string, gen int) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		tracks, err := client.Search(ctx, query)
		return spotlightResultsMsg{gen: gen, query: query, tracks: tracks, err: err}
	}
}

func (m Spotify) loadArtCmd(url string) tea.Cmd {
	w, h := m.artW, m.artH
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		art, err := fetchAlbumArt(ctx, url, w, h)
		return artMsg{url: url, art: art, err: err}
	}
}

// pollCmd fetches the now-playing state, and the up-next queue only when asked
// (Queue() is the heavier call and rarely changes between ticks).
func (m Spotify) pollCmd(withQueue bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		state, _ := client.State(ctx)
		msg := playerMsg{state: state}
		if withQueue {
			msg.queue, _ = client.Queue(ctx)
			msg.hadQueue = true
		}
		return msg
	}
}

// recoverDeviceCmd re-resolves the "AudioPulse" device by name (after librespot
// restarts or the device drops) and transfers playback back to it, returning the
// new device id. Used to recover from "device not found" playback errors.
func (m Spotify) recoverDeviceCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		id, err := client.FindDevice(ctx, config.DeviceName)
		if err != nil || id == "" {
			return deviceMsg{} // not back yet; UI keeps the old id
		}
		_ = client.Transfer(ctx, id, true) // make it active again
		return deviceMsg{id: id}
	}
}

// playSelectedCmd starts playback of the selected track within its source.
func (m Spotify) playSelectedCmd() tea.Cmd {
	if m.trackCursor < 0 || m.trackCursor >= len(m.tracks) {
		return nil
	}
	client := m.client
	deviceID := m.deviceID
	src := m.source
	pos := m.trackCursor
	tracks := m.tracks
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var err error
		if src.contextURI != "" {
			err = client.PlayContext(ctx, deviceID, src.contextURI, tracks[pos].URI)
		} else {
			// No context URI (Liked Songs / Recent / Search): send an explicit
			// URI list. Spotify's play endpoint rejects very large arrays, so
			// send a bounded window starting at the selection and continuing
			// through the list; the selected track becomes offset 0.
			end := pos + maxPlayURIs
			if end > len(tracks) {
				end = len(tracks)
			}
			window := tracks[pos:end]
			uris := make([]zspotify.URI, len(window))
			for i, t := range window {
				uris[i] = t.URI
			}
			err = client.PlayTracksAt(ctx, deviceID, uris, 0)
		}
		return actionMsg{err: err}
	}
}

// queueSelectedCmd adds the selected music track to the play queue. Returns nil
// when the music pane isn't focused or nothing is selected (queueing is
// track-only; the Web API helper doesn't take episode URIs).
func (m Spotify) queueSelectedCmd() tea.Cmd {
	if m.focus != panelTracks || m.trackCursor < 0 || m.trackCursor >= len(m.tracks) {
		return nil
	}
	client := m.client
	id := m.tracks[m.trackCursor].ID
	deviceID := m.deviceID
	return m.action(func(ctx context.Context) error { return client.AddToQueue(ctx, id, deviceID) })
}

// toggleLikeCmd saves or removes a track in Liked Songs (opposite of wasLiked).
func (m Spotify) toggleLikeCmd(id zspotify.ID, wasLiked bool) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		var err error
		if wasLiked {
			err = client.UnlikeTrack(ctx, id)
		} else {
			err = client.LikeTrack(ctx, id)
		}
		return likeMsg{id: id, liked: !wasLiked, err: err}
	}
}

// checkSavedCmd looks up whether a track is in Liked Songs (for the ♥ state).
func (m Spotify) checkSavedCmd(id zspotify.ID) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		saved, err := client.TrackSaved(ctx, id)
		if err != nil {
			return nil
		}
		return savedCheckMsg{id: id, saved: saved}
	}
}

// unfollowShowCmd unfollows a saved podcast, then reloads the show list.
func (m Spotify) unfollowShowCmd(show spotify.Show) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if err := client.UnfollowShow(ctx, show.ID); err != nil {
			return actionMsg{err: err}
		}
		shows, err := client.SavedShows(ctx)
		return showsMsg{shows: shows, err: err}
	}
}

// likeTarget is the track a like action applies to: the selected music track if
// the music pane is focused, else the currently playing track.
func (m Spotify) likeTarget() (zspotify.ID, bool) {
	if m.focus == panelTracks && m.trackCursor >= 0 && m.trackCursor < len(m.tracks) {
		if id := m.tracks[m.trackCursor].ID; id != "" {
			return id, true
		}
	}
	if m.state != nil && m.state.Track != nil && m.state.Track.ID != "" {
		return m.state.Track.ID, true
	}
	return "", false
}

// gatherExportCmd collects every track URI (Liked Songs + all playlists) to be
// exported, plus the resolved output directory.
func (m Spotify) gatherExportCmd() tea.Cmd {
	client := m.client
	var plIDs []zspotify.ID
	for _, it := range m.lib {
		if it.kind == libPlaylist {
			plIDs = append(plIDs, it.plID)
		}
	}
	return func() tea.Msg {
		dir, err := config.Load().MusicPath()
		if err != nil {
			return exportGatheredMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		uris, err := client.ExportURIs(ctx, plIDs)
		return exportGatheredMsg{uris: uris, dir: dir, err: err}
	}
}

// waitExportCmd blocks for the next progress update from the export channel.
func waitExportCmd(ch <-chan downloader.Progress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return exportProgressMsg{p: downloader.Progress{Finished: true}}
		}
		return exportProgressMsg{p: p}
	}
}

// interpretCmd asks the local assistant to turn an utterance into a Command.
func (m Spotify) interpretCmd(utterance string) tea.Cmd {
	ag := m.agent
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		cmd, err := ag.Interpret(ctx, utterance)
		return agentResultMsg{utterance: utterance, cmd: cmd, err: err}
	}
}

// loadIndexCmd loads the persisted library index from disk (fast; absent → nil).
func loadIndexCmd() tea.Cmd {
	return func() tea.Msg {
		path, err := config.LibraryIndexPath()
		if err != nil {
			return idxLoadedMsg{}
		}
		ix, _ := library.Load(path)
		return idxLoadedMsg{index: ix}
	}
}

// startIndexBuild gathers + embeds the whole library on a background goroutine,
// streaming progress then a terminal event with the index (mirrors the export /
// voice channel pattern).
func startIndexBuild(sp library.Spotify, emb library.Embedder) <-chan idxEvent {
	ch := make(chan idxEvent, 64)
	go func() {
		defer close(ch)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		ix, err := library.Build(ctx, sp, emb, func(done, total int) {
			ch <- idxEvent{done: done, total: total}
		})
		ch <- idxEvent{index: ix, err: err}
	}()
	return ch
}

// errIndexClosed is returned if the build channel closes without a result.
var errIndexClosed = errors.New("index build ended unexpectedly")

// waitIdxCmd blocks for the next index-build event.
func waitIdxCmd(ch <-chan idxEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return idxMsg{err: errIndexClosed}
		}
		return idxMsg(ev)
	}
}

// recommendCmd builds a taste context from the index (RAG: the user's tracks
// closest to the seed, or a sample), asks the model for discovery suggestions,
// then resolves each to a playable track via Spotify search.
func (m Spotify) recommendCmd(query string) tea.Cmd {
	ix := m.libIndex
	ag := m.agent
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		var taste []string
		if strings.TrimSpace(query) != "" {
			if vecs, err := ag.Embed(ctx, []string{query}); err == nil && len(vecs) > 0 {
				for _, s := range ix.Search(vecs[0], 15) {
					taste = append(taste, s.Record.Label())
				}
			}
		}
		if len(taste) == 0 {
			for _, r := range ix.Sample(15) {
				taste = append(taste, r.Label())
			}
		}

		sugs, err := ag.Recommend(ctx, query, taste, 12)
		if err != nil {
			return recommendMsg{query: query, err: err}
		}
		var tracks []spotify.Track
		seen := make(map[zspotify.ID]bool)
		for _, s := range sugs {
			q := s.Title
			if s.Artist != "" {
				q += " " + s.Artist
			}
			res, err := client.Search(ctx, q)
			if err != nil || len(res) == 0 {
				continue
			}
			t := res[0]
			if t.ID == "" || seen[t.ID] {
				continue
			}
			seen[t.ID] = true
			tracks = append(tracks, t)
		}
		return recommendMsg{query: query, tracks: tracks}
	}
}

// answerCmd retrieves grounding context from the index and asks the model a
// library question (with prior history), for the chat panel.
func (m Spotify) answerCmd(question string, history []agent.Turn, gen int) tea.Cmd {
	ix := m.libIndex
	ag := m.agent
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		ctxLines := ix.Context(ctx, ag, question, 20)
		text, err := ag.Answer(ctx, question, ctxLines, history)
		return chatAnswerMsg{gen: gen, text: text, err: err}
	}
}

// chatHistory converts the transcript so far into agent turns.
func chatHistory(turns []chatTurn) []agent.Turn {
	out := make([]agent.Turn, 0, len(turns))
	for _, t := range turns {
		role := "user"
		if t.who == "ai" {
			role = "assistant"
		}
		out = append(out, agent.Turn{Role: role, Text: t.text})
	}
	return out
}

// sendChat appends the user's question and kicks off a grounded answer.
func (m Spotify) sendChat(q string) (Spotify, tea.Cmd) {
	history := chatHistory(m.chatTurns)
	m.chatTurns = append(m.chatTurns, chatTurn{who: "you", text: q})
	m.chatInput.Reset()
	m.chatBusy = true
	m.chatGen++
	m.chatScroll = 1 << 30 // pin to bottom
	return m, m.answerCmd(q, history, m.chatGen)
}

// agentPlayCmd searches for the assistant's requested query so the top hit can
// be played.
func (m Spotify) agentPlayCmd(query string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		tracks, err := client.Search(ctx, query)
		return agentPlayMsg{query: query, tracks: tracks, err: err}
	}
}

// startVoice opens the engine (loading the model on first use) and captures one
// spoken phrase on a background goroutine, emitting events on the returned
// channel: a ready event the moment the mic is actually live (so the UI shows
// "Listening…" only then — avoiding ffmpeg's startup gap that would clip the
// first word), then a final event with the transcript or error.
func startVoice(eng *voice.Engine, modelPath, source string) <-chan voiceEvent {
	ch := make(chan voiceEvent, 2)
	go func() {
		defer close(ch)
		if eng == nil {
			e, err := voice.Open(modelPath, source)
			if err != nil {
				ch <- voiceEvent{err: err}
				return
			}
			eng = e
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		text, err := eng.Listen(ctx, func() { ch <- voiceEvent{ready: true, engine: eng} })
		ch <- voiceEvent{engine: eng, text: text, err: err}
	}()
	return ch
}

// waitVoiceCmd blocks for the next voice event from the channel.
func waitVoiceCmd(ch <-chan voiceEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return voiceMsg{} // closed without a result
		}
		return voiceMsg(ev)
	}
}

// action wraps a player control call into a command.
func (m Spotify) action(fn func(ctx context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return actionMsg{err: fn(ctx)}
	}
}

// Quit releases the voice engine's model; librespot is owned by main.
func (m Spotify) Quit() {
	if m.voiceEngine != nil {
		m.voiceEngine.Close()
	}
}
