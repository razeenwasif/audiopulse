package ui

import (
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"audiopulse/internal/agent"
	"audiopulse/internal/downloader"
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

func TestCheatsheet(t *testing.T) {
	m := sampleSpotify()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	sp := mm.(Spotify)
	if !sp.showHelp {
		t.Fatal("? should open the cheatsheet")
	}
	if !strings.Contains(sp.View(), "Keyboard shortcuts") {
		t.Error("cheatsheet overlay not rendered")
	}
	mm, _ = sp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if mm.(Spotify).showHelp {
		t.Error("any key should close the cheatsheet")
	}
}

func TestEpisodeAutoPreview(t *testing.T) {
	m := sampleSpotify()
	// Loading shows previews the first show's episodes (no focus steal).
	mm, cmd := m.Update(showsMsg{shows: []spotify.Show{{ID: "s1", Name: "A"}, {ID: "s2", Name: "B"}}})
	sp := mm.(Spotify)
	if cmd == nil || !sp.episodesLoading {
		t.Error("loading shows should auto-preview the first show's episodes")
	}

	// Browsing the (focused) show list schedules a debounced preview.
	sp.focus = panelPodcasts
	sp.podcastFocus = "shows"
	sp.currentShow = spotify.Show{ID: "s1"}
	mm, cmd = sp.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd == nil {
		t.Error("moving the show cursor should schedule an episode preview")
	}

	// A preview load (focus=false) must not move focus to the episodes box.
	mm, _ = sp.Update(episodesMsg{show: spotify.Show{ID: "s2"}, episodes: []spotify.Episode{{Title: "E"}}, focus: false})
	if mm.(Spotify).podcastFocus != "shows" {
		t.Error("an episode preview should not steal focus from the shows list")
	}
}

func TestLikeToggle(t *testing.T) {
	m := sampleSpotify()
	m.tracks[0].ID = "t1"
	m.trackCursor = 0
	m.focus = panelTracks

	// L likes the selected track optimistically and issues the API call.
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	sp := mm.(Spotify)
	if !sp.liked["t1"] {
		t.Error("L should optimistically mark the track liked")
	}
	if cmd == nil {
		t.Error("L should issue the like command")
	}
	if !strings.Contains(sp.status, "Saved") {
		t.Errorf("status = %q, want a saved confirmation", sp.status)
	}

	// L again unlikes.
	mm, _ = sp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	if mm.(Spotify).liked["t1"] {
		t.Error("a second L should unlike the track")
	}

	// A failed likeMsg reverts the optimistic state.
	sp.liked["t1"] = true
	mm, _ = sp.Update(likeMsg{id: "t1", liked: true, err: errors.New("boom")})
	if mm.(Spotify).liked["t1"] {
		t.Error("a failed like should revert the optimistic state")
	}
}

func TestUnfollowShow(t *testing.T) {
	m := sampleSpotify()
	m.focus = panelPodcasts
	m.podcastFocus = "shows"
	m.shows = []spotify.Show{{ID: "s1", Name: "Daily"}}
	m.showCursor = 0
	m.currentShow = spotify.Show{ID: "s1"}

	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("F")})
	sp := mm.(Spotify)
	if cmd == nil {
		t.Error("F should issue an unfollow command")
	}
	if sp.currentShow.ID != "" {
		t.Error("currentShow should be cleared so the list re-previews")
	}
	if !strings.Contains(sp.status, "Unfollowed") {
		t.Errorf("status = %q, want an unfollow confirmation", sp.status)
	}
}

func TestExportFlow(t *testing.T) {
	// Confirmation screen shows the track count and is dismissible.
	m := sampleSpotify()
	m.exportState = "confirm"
	m.exportURIs = []string{"spotify:track:a", "spotify:track:b"}
	m.exportDir = "/tmp/music"
	if !strings.Contains(m.View(), "2 tracks") {
		t.Error("confirm overlay should show the track count")
	}
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if mm.(Spotify).exportState != "" {
		t.Error("Esc on confirm should cancel the export")
	}

	// An empty gather goes straight to the done/nothing state.
	g := sampleSpotify()
	g.exportState = "gathering"
	mm, _ = g.Update(exportGatheredMsg{uris: nil, dir: "/tmp/m"})
	if mm.(Spotify).exportState != "done" {
		t.Error("an empty gather should end in the done state")
	}

	// A finished progress message ends the run.
	r := sampleSpotify()
	r.exportState = "running"
	mm, _ = r.Update(exportProgressMsg{p: downloader.Progress{Total: 2, Done: 2, Finished: true}})
	if mm.(Spotify).exportState != "done" {
		t.Error("a finished progress update should move to done")
	}

	// `e` without spotdl installed explains instead of starting.
	if !downloader.Available() {
		e := sampleSpotify()
		mm, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
		if sp := mm.(Spotify); sp.exportState != "" || !strings.Contains(strings.ToLower(sp.status), "spotdl") {
			t.Errorf("e without spotdl should explain; state=%q status=%q", sp.exportState, sp.status)
		}
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

func TestAgentPromptFlow(t *testing.T) {
	rune := func(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
	var m tea.Model = sampleSpotify()

	// ":" opens the assistant prompt.
	m, _ = m.Update(rune(":"))
	if m.(Spotify).focus != panelAgent {
		t.Fatal("':' should open the AI assistant prompt")
	}
	// Enter on a typed request flips agentBusy and issues an Interpret command.
	m, _ = m.Update(rune("s"))
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.(Spotify).agentBusy || cmd == nil {
		t.Error("enter should mark the assistant busy and issue an interpret command")
	}
	// An interpret error keeps the overlay open and surfaces the error.
	m, _ = m.Update(agentResultMsg{err: errors.New("Ollama unreachable")})
	if sp := m.(Spotify); sp.focus != panelAgent || sp.agentBusy || sp.agentErr == nil {
		t.Error("an interpret error should keep the prompt open and show the error")
	}
	if !strings.Contains(m.(Spotify).View(), "Ollama unreachable") {
		t.Error("the assistant overlay should render the error text")
	}

	// A shuffle command closes the prompt, sets local intent, and issues a command.
	m, cmd = m.(Spotify).Update(agentResultMsg{cmd: agent.Command{Action: agent.ActionShuffle, On: true}})
	if sp := m.(Spotify); sp.focus == panelAgent || !sp.shuffle || cmd == nil {
		t.Errorf("a shuffle command should close the prompt, set shuffle, and act (focus=%d shuffle=%v)", m.(Spotify).focus, m.(Spotify).shuffle)
	}

	// A repeat "one" maps to the Spotify "track" state.
	m, _ = m.(Spotify).Update(agentResultMsg{cmd: agent.Command{Action: agent.ActionRepeat, Repeat: "one"}})
	if got := m.(Spotify).repeat; got != "track" {
		t.Errorf("repeat one → %q, want track", got)
	}

	// A play command kicks off a search; the results load into the center and play.
	m, cmd = m.(Spotify).Update(agentResultMsg{cmd: agent.Command{Action: agent.ActionPlay, Query: "midnight city"}})
	if cmd == nil {
		t.Fatal("a play command should issue a search")
	}
	m, cmd = m.(Spotify).Update(agentPlayMsg{query: "midnight city", tracks: []spotify.Track{{Title: "Midnight City", Artist: "M83"}}})
	sp := m.(Spotify)
	if cmd == nil || sp.source.title != "Ask: midnight city" || len(sp.tracks) != 1 {
		t.Errorf("play results should load into the center and start playback (source=%q tracks=%d)", sp.source.title, len(sp.tracks))
	}

	// An unrecognised command just sets an explanatory status.
	m, cmd = sp.Update(agentResultMsg{cmd: agent.Command{Action: agent.ActionUnknown}})
	if cmd != nil || !strings.Contains(strings.ToLower(m.(Spotify).status), "didn't catch") {
		t.Errorf("unknown command should explain, not act; status=%q", m.(Spotify).status)
	}
}

func TestVoiceKeyAndTranscript(t *testing.T) {
	rune := func(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

	// Tests build without the `vosk` tag, so voice support is absent → `v`
	// explains how to enable it instead of trying to listen.
	var m tea.Model = sampleSpotify()
	m, cmd := m.Update(rune("v"))
	if cmd != nil || m.(Spotify).voiceListening {
		t.Error("without voice support, 'v' should not start listening")
	}
	if !strings.Contains(strings.ToLower(m.(Spotify).status), "make voice") {
		t.Errorf("status = %q, want a 'make voice' hint", m.(Spotify).status)
	}

	// A transcript feeds the same assistant pipeline as a typed request.
	mm, cmd := sampleSpotify().Update(voiceMsg{text: "play midnight city"})
	sp := mm.(Spotify)
	if cmd == nil {
		t.Error("a transcript should kick off interpretation")
	}
	if sp.voiceListening {
		t.Error("voiceMsg should clear the listening flag")
	}
	if !strings.Contains(sp.status, "Heard:") {
		t.Errorf("status = %q, want it to echo what was heard", sp.status)
	}

	// An empty transcript (nothing heard) prompts a retry, no command issued.
	mm, cmd = sampleSpotify().Update(voiceMsg{text: "   "})
	if cmd != nil || !strings.Contains(mm.(Spotify).status, "Didn't catch") {
		t.Errorf("empty transcript should ask to retry; status=%q", mm.(Spotify).status)
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
