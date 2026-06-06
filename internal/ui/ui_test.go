package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"audiopulse/internal/deezer"
)

func mkTrack(title, artist string, dur int) deezer.Track {
	var t deezer.Track
	t.Title = title
	t.Artist.Name = artist
	t.Duration = dur
	t.Preview = "http://example/preview.mp3"
	return t
}

// renderAt drives the model through a window-size + results message and returns
// the rendered view, failing on any panic.
func TestRenderDoesNotPanic(t *testing.T) {
	var m tea.Model = New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = m.Update(searchResultMsg{tracks: []deezer.Track{
		mkTrack("Starboy", "The Weeknd", 230),
		mkTrack("Get Lucky", "Daft Punk", 248),
		mkTrack("Instant Crush", "Daft Punk", 337),
	}})

	view := m.View()
	for _, want := range []string{"AudioPulse", "Starboy", "Daft Punk", "NOW PLAYING"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n---\n%s", want, view)
		}
	}
}

func TestTinyTerminalMessage(t *testing.T) {
	var m tea.Model = New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
	if !strings.Contains(m.View(), "at least 64") {
		t.Errorf("expected min-size message, got:\n%s", m.View())
	}
}

func TestCursorNavigationAndFocus(t *testing.T) {
	var m tea.Model = New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = m.Update(searchResultMsg{tracks: []deezer.Track{
		mkTrack("A", "x", 100), mkTrack("B", "y", 100), mkTrack("C", "z", 100),
	}})
	// After results arrive, focus should move to the results pane.
	if mm := m.(Model); mm.focus != focusResults {
		t.Fatalf("expected focusResults after search, got %v", mm.focus)
	}
	// Move down twice.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if mm := m.(Model); mm.cursor != 2 {
		t.Errorf("expected cursor 2, got %d", mm.cursor)
	}
	// Switch back to search with "/".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if mm := m.(Model); mm.focus != focusSearch {
		t.Errorf("expected focusSearch after '/', got %v", mm.focus)
	}
}
