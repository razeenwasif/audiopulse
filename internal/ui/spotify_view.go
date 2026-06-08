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
	centerWidth := m.centerOuterWidth()

	panels := []string{m.renderLeft(), m.renderCenter(centerWidth)}
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

	// Float the full-lyrics pane, centered.
	if m.lyricsModal {
		box := m.renderLyricsModal()
		x := (m.width - lipgloss.Width(box)) / 2
		y := (m.height - lipgloss.Height(box)) / 2
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

// renderLyricsModal builds the floating, word-wrapped full-lyrics pane.
func (m Spotify) renderLyricsModal() string {
	boxW := m.lyricsModalWidth()
	innerW := boxW - 4 // padding (2 each side); border is outside Width

	title := "Lyrics"
	if m.state != nil && m.state.Track != nil {
		title = m.state.Track.Title + " — " + m.state.Track.Artist
	}
	header := lipgloss.NewStyle().Foreground(colorAccentHi).Bold(true).Render(truncate(title, innerW))
	sep := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", innerW))

	disp, _ := m.lyricsModalLines(innerW)
	bodyH := m.lyricsModalBodyH()
	if bodyH > len(disp) {
		bodyH = len(disp)
	}
	if bodyH < 1 {
		bodyH = 1
	}
	start := m.lyricsModalStart()
	end := start + bodyH
	if end > len(disp) {
		end = len(disp)
	}

	rows := make([]string, 0, bodyH)
	for i := start; i < end; i++ {
		text := disp[i].text
		if text == "" {
			text = " "
		}
		if disp[i].current {
			rows = append(rows, lipgloss.NewStyle().Foreground(colorAccentHi).Bold(true).Render(text))
		} else {
			rows = append(rows, m.st.rowTitle.Render(text))
		}
	}
	// Pad to a stable height so the box doesn't jump as lines wrap/scroll.
	for len(rows) < bodyH {
		rows = append(rows, " ")
	}

	hintText := "↑↓ scroll · esc close"
	if len(disp) > bodyH {
		hintText = fmt.Sprintf("%d–%d of %d · ↑↓ scroll · g/G ends · esc close", start+1, end, len(disp))
	}
	if m.lyricsSynced {
		state := "off"
		if m.lyricsFollow {
			state = "on"
		}
		hintText += " · f follow:" + state
	}
	hint := lipgloss.NewStyle().Foreground(colorFaint).Render(truncate(hintText, innerW))

	parts := append([]string{header, sep}, rows...)
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

// renderLeft stacks the library above the lyrics panel, filling the left column.
func (m Spotify) renderLeft() string {
	lyrH := m.lyricsPanelHeight()
	if lyrH == 0 {
		return m.renderLibrary(m.middleHeight())
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderLibrary(m.middleHeight()-lyrH),
		m.renderLyrics(lyrH),
	)
}

// lyricsPanelHeight is the outer height of the lyrics panel under the library,
// or 0 when the column is too short to split sensibly.
func (m Spotify) lyricsPanelHeight() int {
	total := m.middleHeight()
	h := total * 2 / 5
	if h > 14 {
		h = 14
	}
	if h < 7 {
		h = 7
	}
	if h > total-8 {
		h = total - 8
	}
	if h < 6 {
		return 0
	}
	return h
}

// libPanelHeight is the outer height of the library panel (column minus lyrics).
func (m Spotify) libPanelHeight() int {
	return m.middleHeight() - m.lyricsPanelHeight()
}

func (m Spotify) renderLibrary(outerHeight int) string {
	box := panelBox(m.focus == panelLibrary, 1, 2)
	sw, sh, tw, th := panelDims(box, spotifySidebarWidth, outerHeight)

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
	_, _, _, th := panelDims(panelBox(false, 1, 2), spotifySidebarWidth, m.libPanelHeight())
	v := (th - 2) / 2
	if v < 1 {
		v = 1
	}
	return v
}

// renderLyrics draws the lyrics panel under the library. Synced lyrics follow
// playback (current line highlighted and centered); plain lyrics show from top.
func (m Spotify) renderLyrics(outerHeight int) string {
	focused := m.focus == panelLyrics
	box := panelBox(focused, 0, 1)
	sw, sh, tw, th := panelDims(box, spotifySidebarWidth, outerHeight)

	label := "Lyrics"
	if focused {
		label += "  ↵ expand"
	}
	header := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(truncate(label, tw))
	bodyH := th - 1
	if bodyH < 1 {
		bodyH = 1
	}
	content := header + "\n" + m.lyricsBody(tw, bodyH)
	return box.Width(sw).Height(sh).Render(clipLines(content, th))
}

func (m Spotify) lyricsBody(w, h int) string {
	if m.lyricsState != "ready" {
		return m.st.rowMuted.Render(truncate(lyricsStateText(m.lyricsState), w))
	}

	lines := m.lyricsLines
	cur := -1
	if m.lyricsSynced && m.state != nil {
		cur = currentLyricLine(lines, m.state.Progress)
	}

	// Window h lines: keep the current synced line centered (and scrolling);
	// plain lyrics start at the top.
	start := 0
	if m.lyricsSynced && cur >= 0 {
		start = cur - h/2
	}
	if start > len(lines)-h {
		start = len(lines) - h
	}
	if start < 0 {
		start = 0
	}
	end := start + h
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		text := lines[i].Text
		if text == "" {
			text = " "
		}
		var rendered string
		switch {
		case i == cur:
			rendered = m.st.rowSel.Render(truncate(text, w)) // current line, green
		case m.lyricsSynced && i < cur:
			rendered = m.st.rowMuted.Render(truncate(text, w)) // already sung
		default:
			rendered = m.st.rowTitle.Render(truncate(text, w))
		}
		b.WriteString(rendered)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
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

// centerOuterWidth is the full width available to the center column.
func (m Spotify) centerOuterWidth() int {
	w := m.width - spotifySidebarWidth
	if m.width >= 112 {
		w -= spotifyRightWidth
	}
	return w
}

// centerSplit reports whether the center shows music and podcasts side by side.
func (m Spotify) centerSplit() bool { return m.centerOuterWidth() >= centerSplitMin }

// renderCenter lays out the center column: music | podcasts side by side when
// wide enough, otherwise a single pane toggled by the Music/Podcasts chips.
func (m Spotify) renderCenter(outerWidth int) string {
	if m.centerSplit() {
		leftW := outerWidth / 2
		rightW := outerWidth - leftW
		return lipgloss.JoinHorizontal(lipgloss.Top,
			m.renderMusicPane(leftW, false),
			m.renderPodcastPane(rightW, false),
		)
	}
	if m.centerTab == "podcasts" {
		return m.renderPodcastPane(outerWidth, true)
	}
	return m.renderMusicPane(outerWidth, true)
}

// paneHeader renders a pane's heading row — either the Music/Podcasts toggle
// chips (single-column mode) or a plain label that greens when focused.
func (m Spotify) paneHeader(label string, focused, withChips bool) string {
	if withChips {
		return m.renderChips()
	}
	st := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	if focused {
		st = st.Foreground(colorAccentHi)
	}
	return st.Render(label)
}

func (m Spotify) renderMusicPane(outerWidth int, withChips bool) string {
	box := panelBox(m.focus == panelTracks, 0, 1)
	sw, sh, tw, th := panelDims(box, outerWidth, m.middleHeight())

	head := m.paneHeader("Music", m.focus == panelTracks, withChips)

	title := m.source.title
	if title == "" {
		title = "Browse"
	}
	titleLine := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(truncate(title, tw))

	sub := fmt.Sprintf("%d tracks", len(m.tracks))
	if m.loading {
		// Pages stream in the background; show progress as they arrive.
		if m.tracksTotal > len(m.tracks) {
			sub = fmt.Sprintf("Loading… %d/%d", len(m.tracks), m.tracksTotal)
		} else {
			sub = "Loading…"
		}
	}
	if m.err != nil {
		sub = "⚠ " + m.err.Error()
	}
	cols := m.columnsHeader(tw, m.st.rowMuted.Render(truncate(sub, tw)))

	listH := th - 4 // header, title, columns, blank
	if listH < 1 {
		listH = 1
	}
	list := m.renderTrackList(tw, listH)

	body := lipgloss.JoinVertical(lipgloss.Left, head, titleLine, cols, "", list)
	return box.Width(sw).Height(sh).Render(clipLines(body, th))
}

func (m Spotify) renderPodcastPane(outerWidth int, withChips bool) string {
	box := panelBox(m.focus == panelPodcasts, 0, 1)
	sw, sh, tw, th := panelDims(box, outerWidth, m.middleHeight())

	head := m.paneHeader("Podcasts", m.focus == panelPodcasts, withChips)

	var titleStr, sub, list string
	listH := th - 4
	if listH < 1 {
		listH = 1
	}
	if m.podcastView == "episodes" {
		titleStr = m.currentShow.Name
		sub = fmt.Sprintf("esc back · %d episodes", len(m.episodes))
		if m.episodesLoading {
			sub = "Loading episodes…"
		}
		list = m.renderEpisodeList(tw, listH)
	} else {
		titleStr = "Your Shows"
		sub = fmt.Sprintf("%d shows", len(m.shows))
		if m.showsLoading {
			sub = "Loading…"
		}
		list = m.renderShowList(tw, listH)
	}
	if m.podErr != nil {
		sub = "⚠ podcasts unavailable"
	}
	titleLine := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render(truncate(titleStr, tw))
	subLine := m.st.rowMuted.Render(truncate(sub, tw))

	body := lipgloss.JoinVertical(lipgloss.Left, head, titleLine, subLine, "", list)
	return box.Width(sw).Height(sh).Render(clipLines(body, th))
}

func (m Spotify) renderShowList(w, h int) string {
	faint := func(s string) string { return lipgloss.NewStyle().Height(h).Foreground(colorFaint).Render(s) }
	if m.showsLoading && len(m.shows) == 0 {
		return faint("Loading your podcasts…")
	}
	if len(m.shows) == 0 {
		if m.podErr != nil {
			return faint("Couldn't load podcasts.")
		}
		return faint("No saved podcasts.")
	}

	start, end := trackWindow(m.showCursor, len(m.shows), h)
	var b strings.Builder
	for i := start; i < end; i++ {
		s := m.shows[i]
		text := s.Name
		if s.Publisher != "" {
			text += " · " + s.Publisher
		}
		if i == m.showCursor && m.focus == panelPodcasts {
			b.WriteString(m.st.rowSel.Render("▶ " + truncate(text, w-2)))
		} else {
			b.WriteString(m.st.rowTitle.Render("  " + truncate(text, w-2)))
		}
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return lipgloss.NewStyle().Height(h).MaxHeight(h).Render(b.String())
}

func (m Spotify) renderEpisodeList(w, h int) string {
	faint := func(s string) string { return lipgloss.NewStyle().Height(h).Foreground(colorFaint).Render(s) }
	if m.episodesLoading && len(m.episodes) == 0 {
		return faint("Loading episodes…")
	}
	if len(m.episodes) == 0 {
		return faint("No episodes.")
	}

	start, end := trackWindow(m.episodeCursor, len(m.episodes), h)
	durW := 6
	textW := w - durW - 3 // 2-cell marker + 1 space before duration
	if textW < 6 {
		textW = 6
	}
	var nowID string
	if m.state != nil && m.state.Track != nil {
		nowID = string(m.state.Track.ID)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		e := m.episodes[i]
		label := e.Title
		if e.Date != "" {
			label = e.Date + "  " + e.Title
		}
		dur := fmtDur(e.Duration)

		marker := "  "
		if !e.Playable {
			marker = m.st.rowMuted.Render("⊘ ") // region-locked / externally hosted
		}
		var body, durCol string
		switch {
		case i == m.episodeCursor && m.focus == panelPodcasts:
			marker = m.st.rowSel.Render("▶ ")
			body = m.st.rowSel.Render(padRight(truncate(label, textW), textW))
			durCol = m.st.rowSel.Render(dur)
		case string(e.ID) != "" && string(e.ID) == nowID:
			body = m.st.barFill.Render(padRight(truncate(label, textW), textW))
			durCol = m.st.rowMuted.Render(dur)
		default:
			style := m.st.rowTitle
			if !e.Playable {
				style = m.st.rowMuted
			}
			body = style.Render(padRight(truncate(label, textW), textW))
			durCol = m.st.rowMuted.Render(dur)
		}
		b.WriteString(marker + body + " " + durCol)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return lipgloss.NewStyle().Height(h).MaxHeight(h).Render(b.String())
}

func (m Spotify) renderChips() string {
	on := lipgloss.NewStyle().Background(colorAccent).Foreground(colorBlack).Bold(true).Padding(0, 1)
	off := lipgloss.NewStyle().Background(colorCard).Foreground(colorText).Padding(0, 1)
	music, pod := off, off
	if m.centerTab == "podcasts" {
		pod = on
	} else {
		music = on
	}
	return music.Render("Music") + " " + pod.Render("Podcasts")
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

// renderRight stacks the now-playing panel above the CAVA-style visualizer,
// both with a light-green border, filling the right column.
func (m Spotify) renderRight() string {
	total := m.middleHeight()

	// Give the visualizer a compact slice of the column; the rest is now-playing.
	vizOuter := total / 3
	if vizOuter > 11 {
		vizOuter = 11
	}
	if vizOuter < 7 {
		vizOuter = 7
	}
	if vizOuter > total-6 {
		vizOuter = total - 6
	}
	if vizOuter < 5 {
		// Too short to split sensibly; show now-playing across the whole column.
		return m.renderNowPlaying(total)
	}
	now := m.renderNowPlaying(total - vizOuter)
	viz := m.renderVisualizer(vizOuter)
	return lipgloss.JoinVertical(lipgloss.Left, now, viz)
}

func (m Spotify) renderNowPlaying(outerHeight int) string {
	box := lightPanelBox(0, 1)
	sw, sh, tw, th := panelDims(box, spotifyRightWidth, outerHeight)

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

// renderVisualizer draws the CAVA-style spectrum panel. The spectrum is a
// procedural animation (see vizLevels) since AudioPulse can't tap librespot's
// PCM — it animates while playing and flattens when paused.
func (m Spotify) renderVisualizer(outerHeight int) string {
	box := lightPanelBox(0, 1)
	sw, sh, tw, th := panelDims(box, spotifyRightWidth, outerHeight)

	var b strings.Builder
	b.WriteString(m.st.rowMuted.Render("Visualizer"))
	b.WriteString("\n")
	barsH := th - 1 // reserve the label line
	if barsH < 1 {
		barsH = 1
	}
	b.WriteString(m.renderBars(tw, barsH))
	return box.Width(sw).Height(sh).Render(clipLines(b.String(), th))
}

// vizBlocks are the eighth-height vertical block runes, index 0..8.
var vizBlocks = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// renderBars renders a w×h grid of thin vertical bars (1 cell wide, separated by
// a 1-cell gap) from the current spectrum levels, with a bottom-bright →
// top-pale green gradient.
func (m Spotify) renderBars(w, h int) string {
	if w < 1 || h < 1 {
		return ""
	}
	nbars := (w + 1) / 2 // a bar on even columns, a gap on odd ones
	levels := m.vizLevels(nbars)
	var b strings.Builder
	for row := 0; row < h; row++ {
		fromBottom := h - 1 - row // 0 at the bottom row
		// Color deepens toward the bottom; cells in the same row share a color.
		col := colorAccent
		switch frac := float64(fromBottom) / float64(h); {
		case frac > 0.66:
			col = colorVizTop
		case frac > 0.33:
			col = colorAccentHi
		}
		style := lipgloss.NewStyle().Foreground(col)
		var line strings.Builder
		for c := 0; c < w; c++ {
			if c%2 == 1 { // gap column between bars
				line.WriteByte(' ')
				continue
			}
			eighths := int(levels[c/2]*float64(h)*8) - fromBottom*8
			if eighths <= 0 {
				line.WriteByte(' ')
				continue
			}
			if eighths > 8 {
				eighths = 8
			}
			line.WriteRune(vizBlocks[eighths])
		}
		b.WriteString(style.Render(line.String()))
		if row < h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
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
	outerW = m.centerOuterWidth()
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
