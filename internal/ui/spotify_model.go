package ui

import (
	"context"
	"math"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zspotify "github.com/zmb3/spotify/v2"

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
type tracksMsg struct {
	source trackSource
	tracks []spotify.Track
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

// --- commands ---------------------------------------------------------------

func (m Spotify) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return spotifyTickMsg(t) })
}

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

func (m Spotify) loadTracksCmd(item libItem) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var (
			tracks []spotify.Track
			err    error
			src    trackSource
		)
		switch item.kind {
		case libLiked:
			tracks, err = client.LikedSongs(ctx)
			src = trackSource{title: "Liked Songs"}
		case libRecent:
			tracks, err = client.RecentlyPlayed(ctx)
			src = trackSource{title: "Recently Played"}
		default:
			tracks, err = client.PlaylistTracks(ctx, item.plID)
			src = trackSource{title: item.name, contextURI: item.plURI}
		}
		return tracksMsg{source: src, tracks: tracks, err: err}
	}
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
			uris := make([]zspotify.URI, len(tracks))
			for i, t := range tracks {
				uris[i] = t.URI
			}
			err = client.PlayTracksAt(ctx, deviceID, uris, pos)
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
