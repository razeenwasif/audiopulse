package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	titleHeight = 1
	nowHeight   = 5 // 3 content lines + rounded border
	helpHeight  = 1
)

func (m Model) middleHeight() int { return m.height - titleHeight - nowHeight - helpHeight }
func (m Model) mainOuterWidth() int {
	return m.width - sidebarWidth
}

// mainContentWidth is the usable text width inside the main panel.
func (m Model) mainContentWidth() int {
	w := m.mainOuterWidth() - m.st.main.GetHorizontalFrameSize()
	if w < 1 {
		w = 1
	}
	return w
}

// panelDims translates a desired OUTER size into the values to feed a bordered
// lipgloss style. In lipgloss, Width()/Height() set the padding+content box
// (border is added outside), so we subtract only the border for the style and
// the full frame (border+padding) for the inner text area.
func panelDims(base lipgloss.Style, outerW, outerH int) (styleW, styleH, textW, textH int) {
	styleW = outerW - base.GetHorizontalBorderSize()
	styleH = outerH - base.GetVerticalBorderSize()
	textW = outerW - base.GetHorizontalFrameSize()
	textH = outerH - base.GetVerticalFrameSize()
	for _, p := range []*int{&styleW, &styleH, &textW, &textH} {
		if *p < 1 {
			*p = 1
		}
	}
	return
}

// View renders the whole UI.
func (m Model) View() string {
	if m.width < 64 || m.height < 18 {
		return m.st.errText.Render("AudioPulse needs a terminal at least 64×18.\nResize and try again. (ctrl+c to quit)")
	}
	middle := lipgloss.JoinHorizontal(lipgloss.Top, m.renderSidebar(), m.renderMain())
	frame := lipgloss.JoinVertical(lipgloss.Left,
		m.renderTitle(),
		middle,
		m.renderNowPlaying(),
		m.renderHelp(),
	)
	return fillBG(frame, m.width, m.height)
}

func (m Model) renderTitle() string {
	left := m.st.title.Render("♫ AudioPulse")
	right := lipgloss.NewStyle().Foreground(colorMuted).Render("powered by Deezer")
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	return " " + left + strings.Repeat(" ", gap) + right + " "
}

func (m Model) renderSidebar() string {
	sw, sh, tw, _ := panelDims(m.st.sidebar, sidebarWidth, m.middleHeight())

	header := func(s string) string {
		return lipgloss.NewStyle().Foreground(colorFaint).Bold(true).Render(s)
	}
	nav := func(label string, f focus) string {
		if m.focus == f {
			return m.st.barFill.Render("▌") + " " + m.st.sidebarSel.Render(label)
		}
		return "  " + m.st.rowArtist.Render(label)
	}

	nowTitle := "—"
	if m.playing >= 0 && m.playing < len(m.results) {
		nowTitle = truncate(m.results[m.playing].Title, tw)
	}
	autoplay := "off"
	if m.autoplay {
		autoplay = "on"
	}

	lines := []string{
		header("LIBRARY"),
		"",
		nav("Search", focusSearch),
		nav("Results", focusResults),
		"",
		header("NOW PLAYING"),
		m.st.rowTitle.Render(nowTitle),
		"",
		m.st.rowMuted.Render("autoplay: ") + m.st.rowArtist.Render(autoplay),
		m.st.rowMuted.Render("results:  ") + m.st.rowArtist.Render(fmt.Sprintf("%d", len(m.results))),
	}
	return m.st.sidebar.Width(sw).Height(sh).Render(strings.Join(lines, "\n"))
}

func (m Model) renderMain() string {
	sw, sh, tw, th := panelDims(m.st.main, m.mainOuterWidth(), m.middleHeight())

	// Search input line + an underline that highlights when focused.
	m.input.Width = tw - lipgloss.Width(m.input.Prompt) - 1
	inputLine := truncate(m.input.View(), tw)
	underlineColor := colorFaint
	if m.focus == focusSearch {
		underlineColor = colorAccent
	}
	underline := lipgloss.NewStyle().Foreground(underlineColor).Render(strings.Repeat("─", tw))

	header := m.statusLine(tw)

	listH := th - 3 // input, underline, header
	if listH < 1 {
		listH = 1
	}
	list := m.renderList(tw, listH)

	body := lipgloss.JoinVertical(lipgloss.Left, inputLine, underline, header, list)
	return m.st.main.Width(sw).Height(sh).Render(body)
}

func (m Model) statusLine(w int) string {
	if m.searching {
		return m.st.rowArtist.Render("Searching…")
	}
	if m.err != nil {
		return m.st.errText.Render(truncate("⚠ "+m.err.Error(), w))
	}
	if m.status != "" {
		return m.st.rowMuted.Render(truncate(m.status, w))
	}
	return m.st.rowMuted.Render(fmt.Sprintf("%d results", len(m.results)))
}

func (m Model) renderList(w, h int) string {
	if len(m.results) == 0 {
		hint := "Press / to focus search, type a query, then Enter."
		return lipgloss.NewStyle().Height(h).Foreground(colorFaint).Render(hint)
	}

	// Window the list so the cursor stays visible.
	start := 0
	if m.cursor >= h {
		start = m.cursor - h + 1
	}
	end := start + h
	if end > len(m.results) {
		end = len(m.results)
		if end-h > 0 {
			start = end - h
		} else {
			start = 0
		}
	}

	durW := 6
	indicatorW := 2
	textW := w - durW - indicatorW - 1
	if textW < 4 {
		textW = 4
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		t := m.results[i]
		titleArtist := fmt.Sprintf("%s — %s", t.Title, t.ArtistName())
		dur := fmtDur(time.Duration(t.Duration) * time.Second)

		var indicator, text, durCol string
		switch {
		case i == m.cursor && m.focus == focusResults:
			indicator = m.st.rowSel.Render("▶ ")
			text = m.st.rowSel.Render(padRight(truncate(titleArtist, textW), textW))
			durCol = m.st.rowSel.Render(padRight(dur, durW))
		default:
			if i == m.playing {
				indicator = m.st.barFill.Render("♪ ")
			} else {
				indicator = "  "
			}
			text = m.st.rowTitle.Render(padRight(truncate(titleArtist, textW), textW))
			durCol = m.st.rowMuted.Render(padRight(dur, durW))
		}

		b.WriteString(indicator + text + " " + durCol)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return lipgloss.NewStyle().Height(h).MaxHeight(h).Render(b.String())
}

func (m Model) renderNowPlaying() string {
	sw, sh, contentW, _ := panelDims(m.st.nowBar, m.width, nowHeight)

	state := m.st.rowMuted.Render("■")
	var headline string
	if m.player.HasTrack() && m.playing >= 0 && m.playing < len(m.results) {
		t := m.results[m.playing]
		if m.paused {
			state = m.st.barFill.Render("⏸")
		} else {
			state = m.st.barFill.Render("▶")
		}
		headline = state + "  " + m.st.nowTitle.Render(truncate(t.Title, contentW/2)) +
			m.st.nowArtist.Render("  —  "+truncate(t.ArtistName(), contentW/3))
	} else {
		headline = state + "  " + m.st.rowMuted.Render("Nothing playing — select a track and press Enter")
	}

	// Progress bar.
	var frac float64
	if m.total > 0 {
		frac = float64(m.elapsed) / float64(m.total)
	}
	times := fmt.Sprintf("%s / %s", fmtDur(m.elapsed), fmtDur(m.total))
	barW := contentW - lipgloss.Width(times) - 2
	progress := m.renderBar(frac, barW) + "  " + m.st.rowMuted.Render(times)

	// Volume line.
	volLabel := m.st.rowMuted.Render("vol ")
	volBar := m.renderBar(m.volume, 12)
	autoplay := "autoplay off"
	if m.autoplay {
		autoplay = "autoplay on"
	}
	volLine := volLabel + volBar + "   " + m.st.rowMuted.Render(autoplay)

	body := lipgloss.JoinVertical(lipgloss.Left, headline, progress, volLine)
	return m.st.nowBar.Width(sw).Height(sh).Render(body)
}

func (m Model) renderBar(frac float64, width int) string {
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
	return m.st.barFill.Render(strings.Repeat("━", filled)) +
		m.st.barEmpty.Render(strings.Repeat("─", width-filled))
}

func (m Model) renderHelp() string {
	key := m.st.helpKey.Render
	dim := m.st.help.Render
	var parts []string
	if m.focus == focusSearch {
		parts = []string{
			key("enter") + dim(" search"),
			key("esc") + dim(" results"),
			key("ctrl+c") + dim(" quit"),
		}
	} else {
		parts = []string{
			key("↑↓/jk") + dim(" move"),
			key("enter") + dim(" play"),
			key("space") + dim(" pause"),
			key("n/b") + dim(" next/prev"),
			key("+/-") + dim(" vol"),
			key("s") + dim(" stop"),
			key("a") + dim(" autoplay"),
			key("/") + dim(" search"),
			key("q") + dim(" quit"),
		}
	}
	line := "  " + strings.Join(parts, dim("  •  "))
	return lipgloss.NewStyle().MaxHeight(1).Render(truncate(line, m.width))
}
