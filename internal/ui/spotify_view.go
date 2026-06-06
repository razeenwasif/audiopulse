package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	spotifyTitleHeight  = 1
	spotifyPlayerHeight = 4 // 2 content lines + rounded border
	spotifyHelpHeight   = 1
)

func (m Spotify) middleHeight() int {
	return m.height - spotifyTitleHeight - spotifyPlayerHeight - spotifyHelpHeight
}

// View renders the Spotify UI: title, three panels, player bar, help.
func (m Spotify) View() string {
	if m.width < 88 || m.height < 20 {
		return m.st.errText.Render("AudioPulse needs a terminal at least 88×20 for the Spotify layout.\nResize and try again. (ctrl+c to quit)")
	}

	// Decide which panels fit. The right panel is dropped on narrower screens.
	showRight := m.width >= 112
	centerWidth := m.width - spotifySidebarWidth
	if showRight {
		centerWidth -= spotifyRightWidth
	}

	panels := []string{m.renderLibrary(), m.renderCenter(centerWidth)}
	if showRight {
		panels = append(panels, m.renderRight())
	}
	middle := lipgloss.JoinHorizontal(lipgloss.Top, panels...)

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderSpotifyTitle(),
		middle,
		m.renderPlayerBar(),
		m.renderSpotifyHelp(),
	)
}

func (m Spotify) renderSpotifyTitle() string {
	left := m.st.title.Render("♫ AudioPulse")
	right := lipgloss.NewStyle().Foreground(colorMuted).Render(m.user + " · Spotify")
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	return " " + left + strings.Repeat(" ", gap) + right + " "
}

func (m Spotify) renderLibrary() string {
	sw, sh, tw, _ := panelDims(m.st.sidebar, spotifySidebarWidth, m.middleHeight())

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(colorFaint).Bold(true).Render("LIBRARY"))
	b.WriteString("\n\n")

	for i, it := range m.lib {
		icon := "▸"
		switch it.kind {
		case libLiked:
			icon = "♥"
		case libRecent:
			icon = "◷"
		}
		label := truncate(it.name, tw-2)
		line := icon + " " + label
		if i == m.libCursor && m.focus == panelLibrary {
			line = m.st.rowSel.Render("▌" + icon + " " + label)
		} else if i == m.libCursor {
			line = m.st.rowTitle.Render(" " + icon + " " + label)
		} else {
			line = m.st.rowArtist.Render(" " + icon + " " + label)
		}
		b.WriteString(truncate(line, tw))
		b.WriteString("\n")
	}
	return m.st.sidebar.Width(sw).Height(sh).Render(b.String())
}

func (m Spotify) renderCenter(outerWidth int) string {
	sw, sh, tw, th := panelDims(m.st.main, outerWidth, m.middleHeight())

	header := m.source.title
	if header == "" {
		header = "Tracks"
	}
	headerLine := m.st.title.Render(truncate(header, tw))
	sub := m.st.rowMuted.Render(fmt.Sprintf("%d tracks", len(m.tracks)))
	if m.loading {
		sub = m.st.rowArtist.Render("Loading…")
	}
	if m.err != nil {
		sub = m.st.errText.Render(truncate("⚠ "+m.err.Error(), tw))
	}

	listH := th - 3
	if listH < 1 {
		listH = 1
	}
	list := m.renderTrackList(tw, listH)

	body := lipgloss.JoinVertical(lipgloss.Left, headerLine, sub, "", list)
	return m.st.main.Width(sw).Height(sh).Render(body)
}

func (m Spotify) renderTrackList(w, h int) string {
	if len(m.tracks) == 0 {
		return lipgloss.NewStyle().Height(h).Foreground(colorFaint).
			Render("No tracks. Pick something on the left and press Enter.")
	}

	start := 0
	if m.trackCursor >= h {
		start = m.trackCursor - h + 1
	}
	end := start + h
	if end > len(m.tracks) {
		end = len(m.tracks)
		if end-h > 0 {
			start = end - h
		} else {
			start = 0
		}
	}

	durW := 6
	textW := w - durW - 3
	if textW < 6 {
		textW = 6
	}

	var nowID string
	if m.state != nil && m.state.Track != nil {
		nowID = string(m.state.Track.ID)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		t := m.tracks[i]
		text := fmt.Sprintf("%s — %s", t.Title, t.Artist)
		dur := fmtDur(t.Duration)

		var marker, body, durCol string
		switch {
		case i == m.trackCursor && m.focus == panelTracks:
			marker = m.st.rowSel.Render("▶ ")
			body = m.st.rowSel.Render(padRight(truncate(text, textW), textW))
			durCol = m.st.rowSel.Render(dur)
		case string(t.ID) != "" && string(t.ID) == nowID:
			marker = m.st.barFill.Render("♪ ")
			body = m.st.barFill.Render(padRight(truncate(text, textW), textW))
			durCol = m.st.rowMuted.Render(dur)
		default:
			marker = "  "
			body = m.st.rowTitle.Render(padRight(truncate(text, textW), textW))
			durCol = m.st.rowMuted.Render(dur)
		}
		b.WriteString(marker + body + " " + durCol)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return lipgloss.NewStyle().Height(h).MaxHeight(h).Render(b.String())
}

func (m Spotify) renderRight() string {
	sw, sh, tw, _ := panelDims(m.st.main, spotifyRightWidth, m.middleHeight())

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(colorFaint).Bold(true).Render("NOW PLAYING"))
	b.WriteString("\n\n")

	// Cover-art placeholder (real thumbnail is a Phase 2 item).
	art := lipgloss.NewStyle().
		Foreground(colorFaint).
		Border(lipgloss.RoundedBorder()).BorderForeground(colorFaint).
		Width(tw - 2).Align(lipgloss.Center).
		Render("\n  ♫  \n")
	b.WriteString(art)
	b.WriteString("\n\n")

	if m.state != nil && m.state.Track != nil {
		t := m.state.Track
		b.WriteString(m.st.nowTitle.Render(truncate(t.Title, tw)))
		b.WriteString("\n")
		b.WriteString(m.st.nowArtist.Render(truncate(t.Artist, tw)))
		b.WriteString("\n")
		b.WriteString(m.st.rowMuted.Render(truncate(t.Album, tw)))
	} else {
		b.WriteString(m.st.rowMuted.Render("Nothing playing"))
	}
	b.WriteString("\n\n")
	b.WriteString(m.st.rowMuted.Render("── Up Next ──"))
	b.WriteString("\n")
	if len(m.queue) == 0 {
		b.WriteString(m.st.rowMuted.Render("queue empty"))
	} else {
		for i, t := range m.queue {
			if i >= 5 {
				break
			}
			b.WriteString(m.st.rowArtist.Render(truncate("• "+t.Title+" — "+t.Artist, tw)))
			b.WriteString("\n")
		}
	}
	return m.st.main.Width(sw).Height(sh).Render(b.String())
}

func (m Spotify) renderPlayerBar() string {
	sw, sh, tw, _ := panelDims(m.st.nowBar, m.width, spotifyPlayerHeight)

	state := m.st.rowMuted.Render("■")
	title := m.st.rowMuted.Render("Nothing playing — pick a track and press Enter")
	var elapsed, total string
	var frac float64
	if m.state != nil && m.state.Track != nil {
		t := m.state.Track
		if m.state.Playing {
			state = m.st.barFill.Render("▶")
		} else {
			state = m.st.barFill.Render("⏸")
		}
		title = m.st.nowTitle.Render(truncate(t.Title, tw/2)) +
			m.st.nowArtist.Render("  —  "+truncate(t.Artist, tw/3))
		elapsed, total = fmtDur(m.state.Progress), fmtDur(t.Duration)
		if t.Duration > 0 {
			frac = float64(m.state.Progress) / float64(t.Duration)
		}
	} else {
		elapsed, total = "0:00", "0:00"
	}

	// Top line: state + title (left), shuffle/repeat (right).
	modes := m.modeIndicators()
	gap := tw - lipgloss.Width(state) - 2 - lipgloss.Width(title) - lipgloss.Width(modes)
	if gap < 1 {
		gap = 1
	}
	line1 := state + "  " + title + strings.Repeat(" ", gap) + modes

	// Bottom line: times + progress + volume.
	times := fmt.Sprintf("%s / %s", elapsed, total)
	vol := 50
	if m.state != nil {
		vol = m.state.Volume
	}
	volStr := fmt.Sprintf("  🔊 %d%%", vol)
	barW := tw - lipgloss.Width(times) - lipgloss.Width(volStr) - 3
	if barW < 4 {
		barW = 4
	}
	line2 := m.st.rowMuted.Render(times) + " " + meter(m.st, frac, barW) + m.st.rowMuted.Render(volStr)

	body := lipgloss.JoinVertical(lipgloss.Left, line1, line2)
	return m.st.nowBar.Width(sw).Height(sh).Render(body)
}

func (m Spotify) modeIndicators() string {
	shuffle, repeat := "🔀", "🔁"
	on := lipgloss.NewStyle().Foreground(colorAccentHi)
	off := lipgloss.NewStyle().Foreground(colorFaint)
	s, r := off, off
	if m.state != nil {
		if m.state.Shuffle {
			s = on
		}
		if m.state.Repeat == "context" || m.state.Repeat == "track" {
			r = on
		}
	}
	return s.Render(shuffle) + " " + r.Render(repeat)
}

func meter(st styles, frac float64, width int) string {
	if width < 1 {
		width = 1
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * float64(width))
	if filled > width {
		filled = width
	}
	return st.barFill.Render(strings.Repeat("━", filled)) +
		st.barEmpty.Render(strings.Repeat("─", width-filled))
}

func (m Spotify) renderSpotifyHelp() string {
	key := m.st.helpKey.Render
	dim := m.st.help.Render
	parts := []string{
		key("tab") + dim(" panel"),
		key("↑↓") + dim(" move"),
		key("enter") + dim(" open/play"),
		key("space") + dim(" pause"),
		key("n/b") + dim(" next/prev"),
		key("←→") + dim(" seek"),
		key("+/-") + dim(" vol"),
		key("s") + dim(" shuffle"),
		key("r") + dim(" repeat"),
		key("q") + dim(" quit"),
	}
	line := "  " + strings.Join(parts, dim("  •  "))
	return lipgloss.NewStyle().MaxHeight(1).Render(truncate(line, m.width))
}
