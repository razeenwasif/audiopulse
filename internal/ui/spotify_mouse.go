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
		m.scrollUnder(msg.X, msg.Y, -1)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.scrollUnder(msg.X, msg.Y, 1)
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
	// Clicks below the library rows fall into the lyrics panel and are ignored.
	if msg.X < spotifySidebarWidth {
		maxLibY := libFirstRowY + m.libVisible()*libItemRows
		if msg.Y >= libFirstRowY && msg.Y < maxLibY {
			start, _ := trackWindow(m.libCursor, len(m.lib), m.libVisible())
			i := start + (msg.Y-libFirstRowY)/libItemRows
			if i >= 0 && i < len(m.lib) {
				m.focus = panelTracks
				m.libCursor = i
				return m, m.beginTrackLoad(m.lib[i])
			}
		}
		// Click in the lyrics panel below the library → focus it (Enter expands).
		if lyrTop := spotifyTopHeight + m.libPanelHeight(); m.lyricsPanelHeight() > 0 && msg.Y >= lyrTop {
			m.focus = panelLyrics
		}
		return m, nil
	}

	// Center column: music pane (left/whole) and podcast pane (right/toggled).
	outerW, listH := m.centerGeom()
	if msg.X >= spotifySidebarWidth && msg.X < spotifySidebarWidth+outerW {
		// Music/Podcasts toggle chips (single-column mode only).
		if !m.centerSplit() && msg.Y == centerHeaderY {
			if cmd, ok := m.clickCenterChip(msg.X); ok {
				return m, cmd
			}
		}
		if m.inPodcastRegion(msg.X) {
			return m, m.clickPodcast(msg.Y)
		}
		// Music pane.
		m.focus = panelTracks
		m.centerTab = "music"
		start, end := trackWindow(m.trackCursor, len(m.tracks), listH)
		if row := msg.Y - trackFirstRowY; row >= 0 && start+row < end {
			m.trackCursor = start + row
			return m, m.playSelectedCmd()
		}
		return m, nil
	}
	return m, nil
}

// podcastEpisodesTopY is the row where the episodes sub-box begins (its border).
func (m Spotify) podcastEpisodesTopY() int { return spotifyTopHeight + m.podcastShowsHeight() }

// podcast list rows begin two header rows (heading + subtitle) below each
// sub-box's top border.
func (m Spotify) podcastShowsListY() int    { return spotifyTopHeight + 3 }
func (m Spotify) podcastEpisodesListY() int { return m.podcastEpisodesTopY() + 3 }

func (m Spotify) podcastListH(outerHeight int) int {
	_, _, _, th := panelDims(panelBox(false, 0, 1), m.centerOuterWidth(), outerHeight)
	if h := th - 2; h > 0 {
		return h
	}
	return 1
}

// clickPodcast focuses the podcast sub-box under y and opens the clicked show or
// plays the clicked episode.
func (m *Spotify) clickPodcast(y int) tea.Cmd {
	loadCmd := m.focusPanel(panelPodcasts)
	if y < m.podcastEpisodesTopY() {
		m.podcastFocus = "shows"
		if loadCmd != nil {
			return loadCmd // still loading saved shows; just focus
		}
		listH := m.podcastListH(m.podcastShowsHeight())
		start, end := trackWindow(m.showCursor, len(m.shows), listH)
		if row := y - m.podcastShowsListY(); row >= 0 && start+row < end {
			m.showCursor = start + row
			m.episodesLoading = true
			return m.loadEpisodesCmd(m.shows[m.showCursor], true)
		}
		return loadCmd
	}
	m.podcastFocus = "episodes"
	listH := m.podcastListH(m.middleHeight() - m.podcastShowsHeight())
	start, end := trackWindow(m.episodeCursor, len(m.episodes), listH)
	if row := y - m.podcastEpisodesListY(); row >= 0 && start+row < end {
		m.episodeCursor = start + row
		return m.playEpisodeCmd()
	}
	return nil
}

// centerHeaderY is the row of the center pane's heading (chips) line.
const centerHeaderY = trackFirstRowY - 4

// clickCenterChip toggles the Music/Podcasts tab when a chip is clicked.
func (m *Spotify) clickCenterChip(x int) (tea.Cmd, bool) {
	cx := spotifySidebarWidth + 2 // border + left padding
	musicEnd := cx + 7            // " Music "
	podStart := musicEnd + 1
	podEnd := podStart + 10 // " Podcasts "
	switch {
	case x >= cx && x < musicEnd:
		return m.focusPanel(panelTracks), true
	case x >= podStart && x < podEnd:
		return m.focusPanel(panelPodcasts), true
	}
	return nil, false
}

func (m Spotify) progressRowY() int { return m.height - 3 }

// inPodcastRegion reports whether x falls in the center's podcast pane.
func (m Spotify) inPodcastRegion(x int) bool {
	cx0 := spotifySidebarWidth
	cw := m.centerOuterWidth()
	if x < cx0 || x >= cx0+cw {
		return false
	}
	if m.centerSplit() {
		return x >= cx0+cw/2
	}
	return m.centerTab == "podcasts"
}

func (m *Spotify) scrollUnder(x, y, delta int) {
	switch {
	case x < spotifySidebarWidth:
		m.libCursor = clamp(m.libCursor+delta, 0, len(m.lib)-1)
	case m.inPodcastRegion(x):
		// Pick the shows or episodes sub-box by row.
		if y >= m.podcastEpisodesTopY() {
			m.episodeCursor = clamp(m.episodeCursor+delta, 0, len(m.episodes)-1)
		} else {
			m.showCursor = clamp(m.showCursor+delta, 0, len(m.shows)-1)
		}
	default:
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
