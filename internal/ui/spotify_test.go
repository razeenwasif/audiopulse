package ui

import (
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"audiopulse/internal/lyrics"
	"audiopulse/internal/spotify"
)

func sampleSpotify() Spotify {
	m := NewSpotify(nil, "dev123", "Tester", 2.0)
	m.width, m.height = 130, 34
	m.lib = []libItem{
		{kind: libLiked, name: "Liked Songs"},
		{kind: libRecent, name: "Recently Played"},
		{kind: libPlaylist, name: "Chill Vibes", count: 42},
	}
	m.libCursor = 2
	m.source = trackSource{title: "Chill Vibes"}
	m.tracks = []spotify.Track{
		{Title: "Midnight City", Artist: "M83", Duration: 244 * time.Second},
		{Title: "Redbone", Artist: "Childish Gambino", Duration: 327 * time.Second},
	}
	m.focus = panelTracks
	now := m.tracks[0]
	m.state = &spotify.PlayerState{Track: &now, Playing: true, Progress: 60 * time.Second, Volume: 70}
	return m
}

func TestSpotifyRenderNoPanic(t *testing.T) {
	view := sampleSpotify().View()
	for _, want := range []string{"AudioPulse", "Your Library", "Chill Vibes", "Now Playing", "Midnight City"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n---\n%s", want, view)
		}
	}
}

func TestSpotifyTinyTerminal(t *testing.T) {
	m := sampleSpotify()
	m.width, m.height = 40, 10
	if !strings.Contains(m.View(), "at least 88") {
		t.Errorf("expected min-size message, got:\n%s", m.View())
	}
}

func TestHalfBlocks(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 4), 128, 255})
		}
	}
	_, h := artDims(2.0)
	art := halfBlocks(img, artCellW, h)
	if lines := strings.Count(art, "\n") + 1; lines != h {
		t.Errorf("art has %d lines, want %d", lines, h)
	}
	if !strings.Contains(art, "\x1b[38;2;") || !strings.Contains(art, "▀") {
		t.Error("art missing 24-bit ANSI half-blocks")
	}
	if halfBlocks(image.NewRGBA(image.Rect(0, 0, 0, 0)), 4, 4) != "" {
		t.Error("empty image should render empty")
	}
}

func TestVisualizerPanel(t *testing.T) {
	m := sampleSpotify() // playing, 130×34 → right column visible

	if !strings.Contains(m.View(), "Visualizer") {
		t.Fatalf("expected Visualizer panel in view:\n%s", m.View())
	}

	// While playing, the animation tick advances vizFrame and reschedules itself.
	updated, cmd := m.Update(vizTickMsg(time.Now()))
	m = updated.(Spotify)
	if m.vizFrame == 0 {
		t.Error("vizFrame should advance on a viz tick while playing")
	}
	if cmd == nil {
		t.Error("viz tick should reschedule itself while playing")
	}
	if bars := m.renderBars(20, 6); !strings.ContainsAny(bars, "▁▂▃▄▅▆▇█") {
		t.Errorf("expected bar glyphs while playing, got %q", bars)
	}

	// Paused: the tick loop ends and bars collapse to a flat resting line.
	m.state.Playing = false
	_, cmd = m.Update(vizTickMsg(time.Now()))
	if cmd != nil {
		t.Error("viz tick should not reschedule while paused")
	}
	bars := m.renderBars(20, 6)
	if strings.ContainsAny(bars, "▆▇█") {
		t.Errorf("expected a flat baseline while paused, got tall bars: %q", bars)
	}
}

func TestLyricsPanelHighlightsCurrentLine(t *testing.T) {
	m := sampleSpotify() // 130×34 → left column tall enough to split
	m.lyricsState = "ready"
	m.lyricsSynced = true
	m.lyricsLines = []lyrics.Line{
		{At: 0, Text: "first line"},
		{At: 10 * time.Second, Text: "second line"},
		{At: 20 * time.Second, Text: "third line"},
	}
	m.state.Progress = 12 * time.Second // between lines 1 and 2 → current is index 1

	if cur := currentLyricLine(m.lyricsLines, m.state.Progress); cur != 1 {
		t.Fatalf("currentLyricLine = %d, want 1", cur)
	}
	view := m.View()
	for _, want := range []string{"Lyrics", "second line"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n---\n%s", want, view)
		}
	}
}

func TestLyricsPanelEmptyState(t *testing.T) {
	m := sampleSpotify()
	m.lyricsState = "none"
	if !strings.Contains(m.View(), "No lyrics found") {
		t.Errorf("expected empty-state message in lyrics panel")
	}
}

func TestLyricsTabFocusAndModal(t *testing.T) {
	m := sampleSpotify() // focus starts on tracks; 130×34 → lyrics panel visible
	m.lyricsState = "ready"
	m.lyricsLines = []lyrics.Line{
		{At: -1, Text: "a very long lyric line that the narrow side panel would truncate but the modal must show in full without cutting"},
	}

	// Cycle Tab from tracks → podcasts → lyrics.
	var mm tea.Model = m
	mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyTab}) // → podcasts
	mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyTab}) // → lyrics
	if sp := mm.(Spotify); sp.focus != panelLyrics {
		t.Fatalf("two Tabs from tracks → focus %d, want panelLyrics (%d)", sp.focus, panelLyrics)
	}

	// Enter opens the floating modal showing the full, wrapped lyric.
	mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sp := mm.(Spotify)
	if !sp.lyricsModal {
		t.Fatal("Enter on the lyrics panel should open the modal")
	}
	if !strings.Contains(sp.View(), "without cutting") {
		t.Errorf("modal should show the full wrapped lyric (tail word missing)\n%s", sp.View())
	}

	// Esc closes it.
	mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if mm.(Spotify).lyricsModal {
		t.Error("Esc should close the lyrics modal")
	}
}

func TestPodcastsSplitAndEpisodes(t *testing.T) {
	m := sampleSpotify() // 130 wide → center splits music | podcasts
	if !m.centerSplit() {
		t.Fatal("expected a side-by-side center split at width 130")
	}
	m.shows = []spotify.Show{
		{Name: "The Daily", Publisher: "NYT"},
		{Name: "Reply All", Publisher: "Gimlet"},
	}
	m.showsLoaded = true

	// Split shows both panes at once.
	view := m.View()
	for _, want := range []string{"Music", "Podcasts", "The Daily", "Midnight City"} {
		if !strings.Contains(view, want) {
			t.Errorf("split center missing %q", want)
		}
	}

	// Tab cycles library → tracks(=start) → podcasts.
	var mm tea.Model = m
	mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyTab})
	if sp := mm.(Spotify); sp.focus != panelPodcasts || sp.centerTab != "podcasts" {
		t.Fatalf("Tab from tracks → focus %d tab %q, want podcasts", mm.(Spotify).focus, mm.(Spotify).centerTab)
	}

	// Episodes view: unplayable episodes are marked and de-emphasized.
	sp := mm.(Spotify)
	sp.podcastFocus = "episodes"
	sp.currentShow = spotify.Show{Name: "The Daily"}
	sp.episodes = []spotify.Episode{
		{Title: "Latest Episode", Playable: true, Date: "2026-06-08"},
		{Title: "Region Locked", Playable: false},
	}
	ev := sp.View()
	for _, want := range []string{"The Daily", "Latest", "⊘"} {
		if !strings.Contains(ev, want) {
			t.Errorf("episode view missing %q", want)
		}
	}
}

func TestCenterToggleWhenNarrow(t *testing.T) {
	m := sampleSpotify()
	m.width, m.height = 95, 30 // center ~63 cols → single pane + chip toggle
	if m.centerSplit() {
		t.Fatal("expected a single center pane at width 95")
	}
	m.shows = []spotify.Show{{Name: "The Daily"}}
	m.showsLoaded = true

	// Music tab: the track list shows; podcasts hidden.
	if !strings.Contains(m.View(), "Redbone") {
		t.Error("music tab should show the track list")
	}

	// Switch to podcasts tab: shows appear, music list is gone.
	m.centerTab = "podcasts"
	m.focus = panelPodcasts
	v := m.View()
	if !strings.Contains(v, "The Daily") {
		t.Error("podcast tab should show the show list")
	}
	if strings.Contains(v, "Redbone") {
		t.Error("music list should be hidden on the podcast tab")
	}
}

func TestDeviceRecovery(t *testing.T) {
	if !isDeviceError(errors.New("Device not found")) {
		t.Error("should classify a device error")
	}
	if isDeviceError(errors.New("rate limited")) {
		t.Error("non-device error misclassified as device error")
	}

	m := sampleSpotify()
	// A device error schedules recovery and says so, instead of a generic fail.
	mm, cmd := m.Update(actionMsg{err: errors.New("Device not found")})
	sp := mm.(Spotify)
	if cmd == nil {
		t.Error("a device error should schedule device recovery")
	}
	if !strings.Contains(strings.ToLower(sp.status), "reconnect") {
		t.Errorf("status = %q, want a reconnecting message", sp.status)
	}

	// A recovered device id is adopted.
	mm, _ = sp.Update(deviceMsg{id: "newdevice123"})
	if got := mm.(Spotify).deviceID; got != "newdevice123" {
		t.Errorf("recovered device id = %q, want newdevice123", got)
	}
}

func TestQueuePollingAndCadence(t *testing.T) {
	m := sampleSpotify() // playing

	// Fast while playing, slow while paused.
	if m.pollInterval() != pollPlaying {
		t.Error("playing should use the fast poll cadence")
	}
	m.state.Playing = false
	if m.pollInterval() != pollPaused {
		t.Error("paused should use the slow poll cadence")
	}

	// A poll that didn't fetch the queue must not wipe the displayed queue.
	m.queue = []spotify.Track{{Title: "Up Next"}}
	mm, _ := m.Update(playerMsg{state: m.state, hadQueue: false})
	if len(mm.(Spotify).queue) != 1 {
		t.Error("a queue-less poll should leave the queue unchanged")
	}
	// A poll that did fetch replaces it.
	mm, _ = m.Update(playerMsg{state: m.state, queue: nil, hadQueue: true})
	if len(mm.(Spotify).queue) != 0 {
		t.Error("a hadQueue poll should replace the queue")
	}

	// A track change marks the queue dirty so it refetches next tick.
	m2 := sampleSpotify()
	m2.lastTrackID = "old"
	nt := spotify.Track{ID: "new"}
	mm, _ = m2.Update(playerMsg{state: &spotify.PlayerState{Track: &nt, Playing: true}})
	if !mm.(Spotify).queueDirty {
		t.Error("a track change should mark the queue dirty")
	}
}

func TestAddToQueue(t *testing.T) {
	m := sampleSpotify() // focus is panelTracks with 2 tracks
	m.tracks[0].ID = "trk1"
	m.trackCursor = 0

	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	sp := mm.(Spotify)
	if cmd == nil {
		t.Fatal("'a' should queue the selected track")
	}
	if !strings.Contains(strings.ToLower(sp.status), "queue") {
		t.Errorf("status = %q, want a queue confirmation", sp.status)
	}
	if !sp.queueDirty {
		t.Error("queueing should mark the up-next queue dirty")
	}

	// 'a' on a non-track pane is a no-op.
	m.focus = panelLibrary
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if cmd != nil {
		t.Error("'a' should do nothing when the music pane isn't focused")
	}
}

func TestWrapText(t *testing.T) {
	if got := wrapText("alpha beta gamma", 11); len(got) != 2 || got[0] != "alpha beta" || got[1] != "gamma" {
		t.Errorf("wrapText word-wrap = %q, want [alpha beta gamma]", got)
	}
	// A single word longer than the width is hard-split.
	if got := wrapText("supercalifragilistic", 5); len(got) != 4 || got[0] != "super" {
		t.Errorf("wrapText hard-split = %q", got)
	}
}

func TestSearchFocusToggle(t *testing.T) {
	var m tea.Model = sampleSpotify()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if m.(Spotify).focus != panelSearch {
		t.Fatal("'/' should focus the search box")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.(Spotify).focus != panelTracks {
		t.Error("esc should leave search focus")
	}
}

func TestMouseWheelScroll(t *testing.T) {
	// Wheel over the center panel (x>=sidebar) moves the track cursor.
	var m tea.Model = sampleSpotify() // 2 tracks, cursor 0
	m, _ = m.Update(tea.MouseMsg{X: 40, Button: tea.MouseButtonWheelDown})
	if got := m.(Spotify).trackCursor; got != 1 {
		t.Errorf("wheel down: trackCursor = %d, want 1", got)
	}
	// Wheel over the library panel (x<sidebar) moves the library cursor.
	m, _ = m.Update(tea.MouseMsg{X: 5, Button: tea.MouseButtonWheelUp})
	if got := m.(Spotify).libCursor; got != 1 { // sample libCursor starts at 2
		t.Errorf("wheel up over library: libCursor = %d, want 1", got)
	}
}

func TestMouseClickPlaysTrack(t *testing.T) {
	var m tea.Model = sampleSpotify()
	// Second visible track row is trackFirstRowY+1; center column x=40.
	m, cmd := m.Update(tea.MouseMsg{X: 40, Y: trackFirstRowY + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if got := m.(Spotify).trackCursor; got != 1 {
		t.Errorf("click track row: trackCursor = %d, want 1", got)
	}
	if cmd == nil {
		t.Error("clicking a track should return a play command")
	}
}

func TestMouseSeekOnProgressBar(t *testing.T) {
	m := sampleSpotify()
	_, _, tw, _ := panelDims(panelBox(false, 0, 2), m.width, spotifyPlayerHeight)
	_, _, barX0, barW := m.progressMetrics(tw)

	// Click in the middle of the progress bar → seek command.
	var mm tea.Model = m
	_, cmd := mm.Update(tea.MouseMsg{X: barX0 + barW/2, Y: m.progressRowY(), Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if cmd == nil {
		t.Error("clicking the progress bar should return a seek command")
	}
	// Click far left of the bar (x=0) → no seek.
	if _, cmd := mm.Update(tea.MouseMsg{X: 0, Y: m.progressRowY(), Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}); cmd != nil {
		t.Error("click outside the progress bar should not seek")
	}
}

func TestRepeatAndShuffleKeys(t *testing.T) {
	rune := func(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
	var m tea.Model = sampleSpotify()

	m, _ = m.Update(rune("r")) // loop all
	if got := m.(Spotify).repeat; got != "context" {
		t.Errorf("after r: repeat = %q, want context", got)
	}
	m, _ = m.Update(rune("r")) // toggle off
	if got := m.(Spotify).repeat; got != "off" {
		t.Errorf("after r,r: repeat = %q, want off", got)
	}
	m, _ = m.Update(rune("R")) // loop one
	if got := m.(Spotify).repeat; got != "track" {
		t.Errorf("after R: repeat = %q, want track", got)
	}
	m, _ = m.Update(rune("s")) // shuffle on
	if !m.(Spotify).shuffle {
		t.Error("after s: shuffle should be on")
	}

	// The loop-one indicator (↻1) should appear in the transport.
	if !strings.Contains(m.View(), "↻1") {
		t.Error("loop-one should render the ↻1 indicator")
	}
}

func TestShuffleStickyAcrossPoll(t *testing.T) {
	var m tea.Model = sampleSpotify()
	// Shuffle starts off and the user turns it on.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if !m.(Spotify).shuffle {
		t.Fatal("expected shuffle on after s")
	}
	// A poll where the API reports shuffle=false must NOT revert it (the Web API
	// doesn't reliably report shuffle for a librespot device).
	m, _ = m.Update(playerMsg{state: &spotify.PlayerState{Shuffle: false, Repeat: "off"}})
	if !m.(Spotify).shuffle {
		t.Error("poll clobbered the user's shuffle intent")
	}
}

func TestSmartShuffleIsInformative(t *testing.T) {
	var m tea.Model = sampleSpotify()
	before := m.(Spotify).shuffle
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	if m.(Spotify).shuffle != before {
		t.Error("smart shuffle should not change shuffle state")
	}
	if cmd != nil {
		t.Error("smart shuffle should not issue an API command")
	}
	if !strings.Contains(m.(Spotify).status, "Smart shuffle") {
		t.Error("smart shuffle should set an explanatory status")
	}
}

func TestSpotlightFlow(t *testing.T) {
	rune := func(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
	var m tea.Model = sampleSpotify()

	// "/" opens the Spotlight overlay.
	m, _ = m.Update(rune("/"))
	if m.(Spotify).focus != panelSearch {
		t.Fatal("'/' should open the Spotlight search")
	}
	// Typing bumps the generation and schedules a debounced search.
	m, cmd := m.Update(rune("d"))
	if m.(Spotify).searchGen == 0 {
		t.Error("typing should bump searchGen")
	}
	if cmd == nil {
		t.Error("typing should schedule a debounced search")
	}
	// Results for the current generation populate the list.
	gen := m.(Spotify).searchGen
	m, _ = m.Update(spotlightResultsMsg{gen: gen, tracks: []spotify.Track{
		{Title: "A", Artist: "x"}, {Title: "B", Artist: "y"},
	}})
	if len(m.(Spotify).spotlightResults) != 2 {
		t.Error("results should populate the overlay")
	}
	// Stale results (old generation) are ignored.
	m, _ = m.Update(spotlightResultsMsg{gen: gen - 1, tracks: nil})
	if len(m.(Spotify).spotlightResults) != 2 {
		t.Error("stale results should be ignored")
	}
	// Enter plays the selection and loads it into the center.
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.(Spotify).focus == panelSearch {
		t.Error("enter should close the Spotlight overlay")
	}
	if cmd == nil {
		t.Error("enter should start playback")
	}
}

func TestClampHelper(t *testing.T) {
	cases := []struct{ v, lo, hi, want int }{
		{5, 0, 10, 5}, {-1, 0, 10, 0}, {11, 0, 10, 10}, {3, 0, -1, 0},
	}
	for _, c := range cases {
		if got := clamp(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clamp(%d,%d,%d)=%d want %d", c.v, c.lo, c.hi, got, c.want)
		}
	}
}
