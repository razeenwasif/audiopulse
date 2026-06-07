package ui

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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

	// The track-repeat glyph (🔂) should appear in the transport.
	if !strings.Contains(m.View(), "🔂") {
		t.Error("loop-one should render the 🔂 glyph")
	}
}

func TestShuffleStickyAcrossPoll(t *testing.T) {
	var m tea.Model = sampleSpotify()
	// First poll seeds shuffle=false.
	m, _ = m.Update(playerMsg{state: &spotify.PlayerState{Shuffle: false, Repeat: "off"}})
	// User presses s → shuffle on.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if !m.(Spotify).shuffle {
		t.Fatal("expected shuffle on after s")
	}
	// A later poll where the API still reports shuffle=false must NOT revert it
	// (the Web API doesn't reliably report shuffle for a librespot device).
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
