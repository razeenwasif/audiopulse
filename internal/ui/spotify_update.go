package ui

import (
	"context"
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
		return m, tea.Batch(m.pollCmd(), m.tickCmd())

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
			m.loading = true
			return m, m.loadTracksCmd(m.lib[m.libCursor])
		}
		return m, nil

	case tracksMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.status = "Could not load tracks."
			return m, nil
		}
		m.err = nil
		m.tracks = msg.tracks
		m.source = msg.source
		m.trackCursor = 0
		return m, nil

	case playerMsg:
		if msg.state != nil {
			m.state = msg.state
		}
		m.queue = msg.queue
		// Load album art when the track's cover changes (only if the right
		// panel is visible).
		if m.width >= 112 && m.state != nil && m.state.Track != nil {
			url := m.state.Track.ImageURL
			if url != "" && url != m.artURL {
				m.artURL = url
				return m, m.loadArtCmd(url)
			}
		}
		return m, nil

	case artMsg:
		if msg.err == nil && msg.url == m.artURL {
			m.art = msg.art
		}
		return m, nil

	case actionMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "Playback action failed."
		}
		return m, m.pollCmd()

	case tea.KeyMsg:
		return m.handleSpotifyKey(msg)
	}
	return m, nil
}

func (m Spotify) handleSpotifyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When the search box has focus, keystrokes edit the query.
	if m.focus == panelSearch {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			q := m.search.Value()
			if q == "" {
				return m, nil
			}
			m.loading = true
			m.focus = panelTracks
			m.search.Blur()
			return m, m.searchCmd(q)
		case "esc":
			m.focus = panelTracks
			m.search.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "/":
		m.focus = panelSearch
		return m, m.search.Focus()

	case "tab":
		if m.focus == panelLibrary {
			m.focus = panelTracks
		} else {
			m.focus = panelLibrary
		}
		return m, nil

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
		if m.focus == panelLibrary {
			if len(m.lib) == 0 {
				return m, nil
			}
			m.loading = true
			m.focus = panelTracks
			return m, m.loadTracksCmd(m.lib[m.libCursor])
		}
		return m, m.playSelectedCmd()

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
		shuffle := m.state != nil && m.state.Shuffle
		return m, m.action(func(ctx context.Context) error { return m.client.SetShuffle(ctx, !shuffle) })
	case "r":
		cur := "off"
		if m.state != nil && m.state.Repeat != "" {
			cur = m.state.Repeat
		}
		return m, m.action(func(ctx context.Context) error {
			_, err := m.client.CycleRepeat(ctx, cur)
			return err
		})
	}
	return m, nil
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
	if m.focus == panelLibrary {
		m.libCursor = clamp(m.libCursor+delta, 0, len(m.lib)-1)
	} else {
		m.trackCursor = clamp(m.trackCursor+delta, 0, len(m.tracks)-1)
	}
}

func (m *Spotify) setCursor(pos int) {
	if m.focus == panelLibrary {
		m.libCursor = clamp(pos, 0, len(m.lib)-1)
	} else {
		m.trackCursor = clamp(pos, 0, len(m.tracks)-1)
	}
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
