package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// spotlightBGCode is the (slightly elevated) opaque background of the search
// overlay, so it reads as a panel floating above the UI.
const spotlightBGCode = "\x1b[48;2;32;32;32m"

const (
	spotifyTopHeight    = 1
	spotifyPlayerHeight = 4 // 2 content lines + rounded border
	spotifyHelpHeight   = 1
)

func (m Spotify) middleHeight() int {
	return m.height - spotifyTopHeight - spotifyPlayerHeight - spotifyHelpHeight
}

// View renders the Spotify UI: top bar, three panels, player bar, help.
func (m Spotify) View() string {
	if m.width < 88 || m.height < 20 {
		return m.st.errText.Render("AudioPulse needs a terminal at least 88×20 for the Spotify layout.\nResize and try again. (ctrl+c to quit)")
	}

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

	frame := lipgloss.JoinVertical(lipgloss.Left,
		m.renderTopBar(),
		middle,
		m.renderPlayerBar(),
		m.renderSpotifyHelp(),
	)
	out := fillBG(frame, m.width, m.height)

	// Float the Spotlight search box over the UI.
	if m.focus == panelSearch {
		box := m.renderSpotlight()
		x := (m.width - lipgloss.Width(box)) / 2
		y := m.height / 5
		if bh := lipgloss.Height(box); y+bh > m.height {
			y = max0(m.height - bh)
		}
		out = overlay(out, box, max0(x), max0(y))
	}
	return out
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// renderSpotlight builds the floating macOS-Spotlight-style search box.
func (m Spotify) renderSpotlight() string {
	boxW := clamp(m.width*3/5, 48, 76)
	innerW := boxW - 4 // padding (4); border is outside Width

	m.search.Width = innerW - 4
	input := m.search.View() // the text input already shows the 🔎 prompt
	sep := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", innerW))

	var rows []string
	query := strings.TrimSpace(m.search.Value())
	switch {
	case query == "":
		rows = append(rows, lipgloss.NewStyle().Foreground(colorFaint).Render("Type to search Spotify…"))
	case m.searching && len(m.spotlightResults) == 0:
		rows = append(rows, lipgloss.NewStyle().Foreground(colorFaint).Render("Searching…"))
	case len(m.spotlightResults) == 0:
		rows = append(rows, lipgloss.NewStyle().Foreground(colorFaint).Render("No results"))
	default:
		const maxRows = 8
		for i, t := range m.spotlightResults {
			if i >= maxRows {
				break
			}
			label := truncate(t.Title+"  —  "+t.Artist, innerW-3)
			if i == m.spotlightCursor {
				rows = append(rows, lipgloss.NewStyle().Foreground(colorAccentHi).Bold(true).Render("▶ "+label))
			} else {
				rows = append(rows, m.st.rowTitle.Render("  "+label))
			}
		}
	}
	hint := lipgloss.NewStyle().Foreground(colorFaint).Render("↑↓ navigate    ↵ play    esc close")

	parts := append([]string{input, sep}, rows...)
	parts = append(parts, "", hint)
	body := lipgloss.JoinVertical(lipgloss.Left, parts...)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(colorAccent).
		Padding(1, 2).Width(boxW).
		Render(body)
	return solidify(box, spotlightBGCode)
}

// overlay composites box onto base at cell (x, y), ANSI-aware.
func overlay(base, box string, x, y int) string {
	baseLines := strings.Split(base, "\n")
	for i, bl := range strings.Split(box, "\n") {
		r := y + i
		if r < 0 || r >= len(baseLines) {
			continue
		}
		bw := ansi.StringWidth(bl)
		full := ansi.StringWidth(baseLines[r])
		left := ansi.Cut(baseLines[r], 0, x)
		right := ""
		if x+bw < full {
			right = ansi.Cut(baseLines[r], x+bw, full)
		}
		baseLines[r] = left + bl + right
	}
	return strings.Join(baseLines, "\n")
}

// solidify forces an opaque background on every cell of s so the floating box
// isn't see-through where inner styles reset the background.
func solidify(s, bgCode string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = bgCode + strings.ReplaceAll(ln, "\x1b[0m", "\x1b[0m"+bgCode) + "\x1b[0m"
	}
	return strings.Join(lines, "\n")
}

// --- top bar -----------------------------------------------------------------

func (m Spotify) renderTopBar() string {
	brand := m.st.title.Render("♫ AudioPulse")
	left := " " + lipgloss.NewStyle().Foreground(colorText).Render("⌂") + "  " + brand

	pillW := m.width / 3
	pillW = clamp(pillW, 26, 56)
	pillStyle := lipgloss.NewStyle().Background(colorCard).Width(pillW).Padding(0, 1)
	content := lipgloss.NewStyle().Background(colorCard).Foreground(colorMuted).Render("🔎  Search   (press /)")
	pill := pillStyle.Render(content)

	right := lipgloss.NewStyle().Foreground(colorMuted).Render(m.user+" ▾") + " "
	line := lipgloss.NewStyle().MaxWidth(m.width).Render(threeCol(m.width, left, pill, right))
	return clipLines(line, spotifyTopHeight)
}

// threeCol lays out left/center/right with center horizontally centered.
func threeCol(w int, left, center, right string) string {
	lw, cw, rw := lipgloss.Width(left), lipgloss.Width(center), lipgloss.Width(right)
	centerStart := (w - cw) / 2
	gapL := centerStart - lw
	if gapL < 1 {
		gapL = 1
	}
	gapR := w - lw - gapL - cw - rw
	if gapR < 1 {
		gapR = 1
	}
	return left + strings.Repeat(" ", gapL) + center + strings.Repeat(" ", gapR) + right
}

// --- library -----------------------------------------------------------------

func (m Spotify) renderLibrary() string {
	box := panelBox(m.focus == panelLibrary, 1, 2)
	sw, sh, tw, th := panelDims(box, spotifySidebarWidth, m.middleHeight())

	header := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Your Library")
	lines := []string{header, ""}

	start, end := trackWindow(m.libCursor, len(m.lib), m.libVisible())
	for i := start; i < end; i++ {
		lines = append(lines, m.libRow(m.lib[i], i, tw))
	}
	body := clipLines(strings.Join(lines, "\n"), th)
	return box.Width(sw).Height(sh).Render(body)
}

// libVisible is how many two-line library entries fit (after the header+blank).
func (m Spotify) libVisible() int {
	_, _, _, th := panelDims(panelBox(false, 1, 2), spotifySidebarWidth, m.middleHeight())
	v := (th - 2) / 2
	if v < 1 {
		v = 1
	}
	return v
}

// libRow renders a two-line library entry: a colored thumbnail block beside a
// title and subtitle.
func (m Spotify) libRow(it libItem, idx, tw int) string {
	selected := idx == m.libCursor && m.focus == panelLibrary
	playing := m.playingFromLib(it)

	thumb := lipgloss.NewStyle().Foreground(m.thumbColor(it, idx)).Render("██\n██")

	marker := " \n "
	titleStyle := m.st.rowTitle
	switch {
	case selected:
		marker = m.st.rowSel.Render("▌") + "\n" + m.st.rowSel.Render("▌")
		titleStyle = m.st.rowSel
	case playing:
		titleStyle = m.st.barFill
	}

	textW := tw - 6
	title := titleStyle.Render(truncate(it.name, textW))
	sub := m.st.rowMuted.Render(truncate(librarySubtitle(it), textW))
	text := lipgloss.JoinVertical(lipgloss.Left, title, sub)
	return lipgloss.JoinHorizontal(lipgloss.Top, marker, " ", thumb, "  ", text)
}

func (m Spotify) playingFromLib(it libItem) bool {
	return it.kind == libPlaylist && it.plURI != "" && m.source.contextURI == it.plURI && m.state != nil && m.state.Playing
}

func (m Spotify) thumbColor(it libItem, idx int) lipgloss.Color {
	switch it.kind {
	case libLiked:
		return lipgloss.Color("#7358FF")
	case libRecent:
		return lipgloss.Color("#509BF5")
	default:
		return thumbPalette[idx%len(thumbPalette)]
	}
}

func librarySubtitle(it libItem) string {
	switch it.kind {
	case libLiked:
		return "Playlist"
	case libRecent:
		return "Played recently"
	default:
		if it.count > 0 {
			return fmt.Sprintf("Playlist • %d songs", it.count)
		}
		return "Playlist"
	}
}

// --- center ------------------------------------------------------------------

func (m Spotify) renderCenter(outerWidth int) string {
	box := panelBox(m.focus == panelTracks, 0, 1)
	sw, sh, tw, th := panelDims(box, outerWidth, m.middleHeight())

	chips := m.renderChips()

	title := m.source.title
	if title == "" {
		title = "Browse"
	}
	titleLine := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(truncate(title, tw))

	sub := fmt.Sprintf("%d tracks", len(m.tracks))
	if m.loading {
		sub = "Loading…"
	}
	if m.err != nil {
		sub = "⚠ " + m.err.Error()
	}
	cols := m.columnsHeader(tw, m.st.rowMuted.Render(truncate(sub, tw)))

	listH := th - 4 // chips, title, columns, blank
	if listH < 1 {
		listH = 1
	}
	list := m.renderTrackList(tw, listH)

	body := lipgloss.JoinVertical(lipgloss.Left, chips, titleLine, cols, "", list)
	return box.Width(sw).Height(sh).Render(clipLines(body, th))
}

func (m Spotify) renderChips() string {
	sel := lipgloss.NewStyle().Background(colorAccent).Foreground(colorBlack).Bold(true).Padding(0, 1)
	un := lipgloss.NewStyle().Background(colorCard).Foreground(colorText).Padding(0, 1)
	return sel.Render("Music") + " " + un.Render("Podcasts")
}

func (m Spotify) columnsHeader(tw int, leftLabel string) string {
	right := m.st.rowMuted.Render("⏱")
	gap := tw - lipgloss.Width(leftLabel) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return leftLabel + strings.Repeat(" ", gap) + right
}

func (m Spotify) renderTrackList(w, h int) string {
	if len(m.tracks) == 0 {
		hint := "Pick something on the left and press Enter."
		return lipgloss.NewStyle().Height(h).Foreground(colorFaint).Render(hint)
	}

	start, end := trackWindow(m.trackCursor, len(m.tracks), h)

	durW := 6
	numW := 3
	textW := w - durW - numW - 2
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

		num := fmt.Sprintf("%2d ", i+1)
		var body, durCol string
		switch {
		case i == m.trackCursor && m.focus == panelTracks:
			num = m.st.rowSel.Render(fmt.Sprintf("%2d ", i+1))
			body = m.st.rowSel.Render(padRight(truncate(text, textW), textW))
			durCol = m.st.rowSel.Render(dur)
		case string(t.ID) != "" && string(t.ID) == nowID:
			num = m.st.barFill.Render(" ♪ ")
			body = m.st.barFill.Render(padRight(truncate(text, textW), textW))
			durCol = m.st.rowMuted.Render(dur)
		default:
			num = m.st.rowMuted.Render(num)
			body = m.st.rowTitle.Render(padRight(truncate(text, textW), textW))
			durCol = m.st.rowMuted.Render(dur)
		}
		b.WriteString(num + body + " " + durCol)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return lipgloss.NewStyle().Height(h).MaxHeight(h).Render(b.String())
}

// --- right (now playing) -----------------------------------------------------

func (m Spotify) renderRight() string {
	box := panelBox(false, 0, 1)
	sw, sh, tw, th := panelDims(box, spotifyRightWidth, m.middleHeight())

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Now Playing"))
	b.WriteString("\n\n")

	if m.art != "" {
		b.WriteString(indentBlock(m.art, (tw-m.artW)/2))
	} else {
		ph := lipgloss.NewStyle().
			Foreground(colorFaint).
			Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder).
			Width(tw - 2).Align(lipgloss.Center).
			Render("\n  ♫  \n")
		b.WriteString(ph)
	}
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
	return box.Width(sw).Height(sh).Render(clipLines(b.String(), th))
}

// --- player bar --------------------------------------------------------------

func (m Spotify) renderPlayerBar() string {
	box := panelBox(false, 0, 2)
	sw, sh, tw, _ := panelDims(box, m.width, spotifyPlayerHeight)

	// Line 1: mini now-playing | centered transport | volume.
	mini := m.st.rowMuted.Render("Nothing playing")
	if m.state != nil && m.state.Track != nil {
		t := m.state.Track
		mini = m.st.nowTitle.Render(truncate(t.Title, tw/4)) +
			m.st.nowArtist.Render(" — "+truncate(t.Artist, tw/5))
	}
	vol := 50
	if m.state != nil {
		vol = m.state.Volume
	}
	volZone := m.st.rowMuted.Render(fmt.Sprintf("vol %d%%", vol))
	// Leave a few columns of slack so glyph-width differences between terminals
	// can't make this line wrap (which would grow the bar and hide the help line).
	line1 := threeCol(tw-4, mini, m.renderTransport(), volZone)

	// Line 2: elapsed + progress bar + total.
	elapsed, total, _, barW := m.progressMetrics(tw)
	var frac float64
	if m.state != nil && m.state.Track != nil && m.state.Track.Duration > 0 {
		frac = float64(m.state.Progress) / float64(m.state.Track.Duration)
	}
	line2 := m.st.rowMuted.Render(elapsed) + " " + meter(m.st, frac, barW) + " " + m.st.rowMuted.Render(total)

	// Truncate each line to the panel width so an over-wide line can't wrap and
	// grow the bar past its row budget (which would push the help line off-screen).
	noWrap := lipgloss.NewStyle().MaxWidth(tw)
	body := lipgloss.JoinVertical(lipgloss.Left, noWrap.Render(line1), noWrap.Render(line2))
	return clipLines(box.Width(sw).Height(sh).Render(body), spotifyPlayerHeight)
}

func (m Spotify) renderTransport() string {
	on := lipgloss.NewStyle().Foreground(colorAccentHi)
	off := lipgloss.NewStyle().Foreground(colorMuted)

	// NOTE: emoji glyphs (🔀 🔁 🔂) are drawn by the terminal in their own colors
	// regardless of SGR styling, so they can't show the green "active" state. Use
	// monochrome text symbols, which respect the foreground color.

	// Shuffle: green when on (local intent).
	sh := off
	if m.shuffle {
		sh = on
	}

	// Repeat: ↻ for loop-all, ↻1 for loop-one; green when active.
	repeatGlyph := "↻ "
	rp := off
	switch m.repeat {
	case "context":
		rp = on
	case "track":
		rp = on
		repeatGlyph = "↻1"
	}

	glyph := ">"
	if m.state != nil && m.state.Playing {
		glyph = "||"
	}
	btn := lipgloss.NewStyle().Background(colorAccent).Foreground(colorBlack).Bold(true).Render(" " + glyph + " ")

	return sh.Render("⇄") + "   " + off.Render("|<") + "   " + btn + "   " + off.Render(">|") + "   " + rp.Render(repeatGlyph)
}

// progressMetrics returns the player-bar progress pieces and the bar's absolute
// x position and width, so render and mouse hit-testing agree.
func (m Spotify) progressMetrics(tw int) (elapsed, total string, barX0, barW int) {
	elapsed, total = "0:00", "0:00"
	if m.state != nil && m.state.Track != nil {
		elapsed, total = fmtDur(m.state.Progress), fmtDur(m.state.Track.Duration)
	}
	barX0 = playerContentX + lipgloss.Width(elapsed) + 1
	// -4 leaves slack (incl. the knob ●) so the line can't wrap on terminals that
	// size a glyph differently than computed.
	barW = tw - lipgloss.Width(elapsed) - lipgloss.Width(total) - 4
	if barW < 4 {
		barW = 4
	}
	return elapsed, total, barX0, barW
}

// --- shared helpers ----------------------------------------------------------

// trackWindow returns the visible [start, end) track indices for a list of the
// given height, scrolled to keep cursor visible.
func trackWindow(cursor, total, h int) (start, end int) {
	if h < 1 {
		h = 1
	}
	if cursor >= h {
		start = cursor - h + 1
	}
	end = start + h
	if end > total {
		end = total
		if end-h > 0 {
			start = end - h
		} else {
			start = 0
		}
	}
	return start, end
}

// centerGeom returns the center panel's outer width and track-list height,
// matching View so mouse hit-testing agrees with the layout.
func (m Spotify) centerGeom() (outerW, listH int) {
	outerW = m.width - spotifySidebarWidth
	if m.width >= 112 {
		outerW -= spotifyRightWidth
	}
	_, _, _, th := panelDims(panelBox(false, 0, 1), outerW, m.middleHeight())
	listH = th - 4
	if listH < 1 {
		listH = 1
	}
	return outerW, listH
}

// clipLines keeps at most the first n lines of s, so a panel's content can never
// overflow its height and push the rest of the layout off-screen.
func clipLines(s string, n int) string {
	if n < 0 {
		n = 0
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// indentBlock prefixes every line of s with n spaces (used to center art).
func indentBlock(s string, n int) string {
	if n <= 0 {
		return s
	}
	pad := strings.Repeat(" ", n)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
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
	knob := ""
	if filled < width {
		knob = st.barFill.Render("●")
		filled--
		if filled < 0 {
			filled = 0
		}
	}
	return st.barFill.Render(strings.Repeat("━", filled)) + knob +
		st.barEmpty.Render(strings.Repeat("─", width-filled-lipgloss.Width(knob)))
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
		key("r/R") + dim(" loop all/one"),
		key("/") + dim(" search"),
		key("q") + dim(" quit"),
	}
	line := "  " + strings.Join(parts, dim("  •  "))
	return clipLines(lipgloss.NewStyle().MaxWidth(m.width).Render(line), spotifyHelpHeight)
}
