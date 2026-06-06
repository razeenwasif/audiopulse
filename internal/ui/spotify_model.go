package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zspotify "github.com/zmb3/spotify/v2"

	"audiopulse/internal/spotify"
)

// spotifyPanel identifies the focused panel.
type spotifyPanel int

const (
	panelLibrary spotifyPanel = iota
	panelTracks
)

const spotifySidebarWidth = 28
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

	focus  spotifyPanel
	status string
	err    error
}

// NewSpotify builds the Spotify UI model.
func NewSpotify(client *spotify.Client, deviceID, user string) Spotify {
	return Spotify{
		st:       newStyles(),
		client:   client,
		deviceID: deviceID,
		user:     user,
		focus:    panelLibrary,
		status:   "Loading your library…",
	}
}

func (m Spotify) Init() tea.Cmd {
	return tea.Batch(m.loadLibraryCmd(), m.tickCmd())
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
