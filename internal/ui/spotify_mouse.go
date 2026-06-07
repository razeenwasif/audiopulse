package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Layout landmarks (0-indexed rows / cells), matching what View renders.
const (
	libFirstRowY   = 5 // first library item's top row
	libItemRows    = 2 // each library entry is two rows tall
	trackFirstRowY = 6 // first visible track row
	playerContentX = 3 // border(1) + left padding(2) of the player bar
)

// handleMouse maps mouse events onto UI actions: wheel scrolls the panel under
// the pointer; clicks open library entries, play tracks, scrub the progress bar,
// or toggle play/pause.
func (m Spotify) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.scrollUnder(msg.X, -1)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.scrollUnder(msg.X, 1)
		return m, nil
	case tea.MouseButtonLeft:
		// handled below
	default:
		return m, nil
	}

	press := msg.Action == tea.MouseActionPress
	drag := msg.Action == tea.MouseActionMotion

	// Progress bar: click or drag to seek.
	if (press || drag) && msg.Y == m.progressRowY() {
		if cmd := m.seekToX(msg.X); cmd != nil {
			return m, cmd
		}
	}
	if !press {
		return m, nil
	}

	// Transport row: the centered third toggles play/pause.
	if msg.Y == m.progressRowY()-1 {
		third := m.width / 3
		if msg.X >= third && msg.X < 2*third {
			return m, m.togglePlay()
		}
	}

	// Library entry → open it (two rows per entry, accounting for scroll).
	if msg.X < spotifySidebarWidth {
		if msg.Y >= libFirstRowY {
			start, _ := trackWindow(m.libCursor, len(m.lib), m.libVisible())
			i := start + (msg.Y-libFirstRowY)/libItemRows
			if i >= 0 && i < len(m.lib) {
				m.focus = panelTracks
				m.libCursor = i
				m.loading = true
				return m, m.loadTracksCmd(m.lib[i])
			}
		}
		return m, nil
	}

	// Center track row → play it.
	outerW, listH := m.centerGeom()
	if msg.X >= spotifySidebarWidth && msg.X < spotifySidebarWidth+outerW {
		start, end := trackWindow(m.trackCursor, len(m.tracks), listH)
		if row := msg.Y - trackFirstRowY; row >= 0 && start+row < end {
			m.focus = panelTracks
			m.trackCursor = start + row
			return m, m.playSelectedCmd()
		}
	}
	return m, nil
}

func (m Spotify) progressRowY() int { return m.height - 3 }

func (m *Spotify) scrollUnder(x, delta int) {
	if x < spotifySidebarWidth {
		m.libCursor = clamp(m.libCursor+delta, 0, len(m.lib)-1)
	} else {
		m.trackCursor = clamp(m.trackCursor+delta, 0, len(m.tracks)-1)
	}
}

// seekToX returns a seek command if x falls within the progress bar.
func (m Spotify) seekToX(x int) tea.Cmd {
	if m.state == nil || m.state.Track == nil || m.state.Track.Duration <= 0 {
		return nil
	}
	_, _, tw, _ := panelDims(panelBox(false, 0, 2), m.width, spotifyPlayerHeight)
	_, _, barX0, barW := m.progressMetrics(tw)
	if x < barX0 || x >= barX0+barW {
		return nil
	}
	frac := float64(x-barX0) / float64(barW)
	target := time.Duration(frac * float64(m.state.Track.Duration))
	return m.action(func(ctx context.Context) error { return m.client.Seek(ctx, target) })
}
