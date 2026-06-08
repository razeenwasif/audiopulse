package ui

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zspotify "github.com/zmb3/spotify/v2"

	"audiopulse/internal/lyrics"
	"audiopulse/internal/spotify"
)

// spotifyPanel identifies the focused panel.
type spotifyPanel int

const (
	panelLibrary spotifyPanel = iota
	panelTracks
	panelSearch
)

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

	state *spotify.PlayerState
	queue []spotify.Track

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

	search textinput.Model

	// Spotlight-style search overlay.
	spotlightResults []spotify.Track
	spotlightCursor  int
	searchGen        int // debounce/staleness generation
	searching        bool

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

	w, h := artDims(cellAspect)
	return Spotify{
		st:       newStyles(),
		client:   client,
		deviceID: deviceID,
		user:     user,
		search:   ti,
		artW:     w,
		artH:     h,
		repeat:   "off",
		focus:    panelLibrary,
		status:   "Loading your library…",
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
	return tea.Batch(m.loadLibraryCmd(), m.tickCmd(), textinput.Blink)
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
	state *spotify.PlayerState
	queue []spotify.Track
}
type actionMsg struct{ err error }
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
type lyricsMsg struct {
	trackID zspotify.ID
	res     lyrics.Result
	err     error
}

// --- commands ---------------------------------------------------------------

func (m Spotify) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return spotifyTickMsg(t) })
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

func (m Spotify) pollCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		state, _ := client.State(ctx)
		queue, _ := client.Queue(ctx)
		return playerMsg{state: state, queue: queue}
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

// action wraps a player control call into a command.
func (m Spotify) action(fn func(ctx context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return actionMsg{err: fn(ctx)}
	}
}

// Quit performs no teardown here; librespot is owned by main.
func (m Spotify) Quit() {}
