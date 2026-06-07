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
	for _, want := range []string{"AudioPulse", "LIBRARY", "Chill Vibes", "NOW PLAYING", "Midnight City"} {
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
