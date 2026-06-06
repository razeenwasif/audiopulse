// Package ui implements the AudioPulse terminal UI with Bubble Tea.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"audiopulse/internal/deezer"
	"audiopulse/internal/player"
)

type focus int

const (
	focusSearch focus = iota
	focusResults
)

const sidebarWidth = 24

// Model is the root Bubble Tea model.
type Model struct {
	st     styles
	client *deezer.Client
	player *player.Player

	width, height int

	input   textinput.Model
	focus   focus
	results []deezer.Track
	cursor  int

	playing  int // index into results of the currently playing track, or -1
	paused   bool
	elapsed  time.Duration
	total    time.Duration
	volume   float64
	autoplay bool

	searching bool
	status    string
	err       error

	ended chan struct{}
}

// messages
type searchResultMsg struct {
	tracks []deezer.Track
	err    error
}
type tickMsg time.Time
type trackEndedMsg struct{}
type playStartedMsg struct {
	index int
	err   error
}

// New builds the initial model.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Search songs, artists, albums…"
	ti.Prompt = "  "
	ti.CharLimit = 80
	ti.Focus()
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorAccent)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorAccent)

	return Model{
		st:       newStyles(),
		client:   deezer.New(),
		player:   player.New(),
		input:    ti,
		focus:    focusSearch,
		playing:  -1,
		autoplay: true,
		volume:   0.66,
		status:   "Type a query and press Enter to search.",
		ended:    make(chan struct{}, 1),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.tickCmd(), m.listenEndedCmd())
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) listenEndedCmd() tea.Cmd {
	return func() tea.Msg {
		<-m.ended
		return trackEndedMsg{}
	}
}

func (m Model) searchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		tracks, err := m.client.Search(ctx, query)
		return searchResultMsg{tracks: tracks, err: err}
	}
}

func (m Model) playCmd(index int) tea.Cmd {
	track := m.results[index]
	ended := m.ended
	return func() tea.Msg {
		err := m.player.Play(track.Preview, func() {
			select {
			case ended <- struct{}{}:
			default:
			}
		})
		return playStartedMsg{index: index, err: err}
	}
}

// Quit stops audio cleanly.
func (m Model) Quit() {
	m.player.Stop()
}

func fmtDur(d time.Duration) string {
	d = d.Round(time.Second)
	return fmt.Sprintf("%d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}

func padRight(s string, w int) string {
	r := []rune(s)
	if len(r) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(r))
}
