package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = m.mainContentWidth() - 4
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case searchResultMsg:
		m.searching = false
		if msg.err != nil {
			m.err = msg.err
			m.status = "Search failed."
			return m, nil
		}
		m.err = nil
		m.results = msg.tracks
		m.cursor = 0
		if len(m.results) == 0 {
			m.status = "No playable results. Try another query."
		} else {
			m.status = ""
			m.focus = focusResults
			m.input.Blur()
		}
		return m, nil

	case playStartedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "Playback error."
			return m, nil
		}
		m.err = nil
		m.playing = msg.index
		m.paused = false
		m.volume = m.player.DisplayVolume()
		return m, nil

	case trackEndedMsg:
		// Re-arm the listener, then auto-advance if enabled.
		cmds := []tea.Cmd{m.listenEndedCmd()}
		if m.autoplay && m.playing >= 0 && m.playing+1 < len(m.results) {
			next := m.playing + 1
			cmds = append(cmds, m.playCmd(next))
		} else {
			m.playing = -1
			m.elapsed, m.total = 0, 0
		}
		return m, tea.Batch(cmds...)

	case tickMsg:
		if m.player.HasTrack() {
			m.elapsed, m.total = m.player.Progress()
			m.paused = m.player.Paused()
		}
		return m, m.tickCmd()
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.Quit()
		return m, tea.Quit
	}

	if m.focus == focusSearch {
		return m.handleSearchKey(msg)
	}
	return m.handleResultsKey(msg)
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		q := m.input.Value()
		if q == "" {
			return m, nil
		}
		m.searching = true
		m.status = "Searching…"
		m.err = nil
		return m, m.searchCmd(q)
	case "esc", "tab":
		if len(m.results) > 0 {
			m.focus = focusResults
			m.input.Blur()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) handleResultsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.Quit()
		return m, tea.Quit
	case "/", "tab":
		m.focus = focusSearch
		m.input.Focus()
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.results)-1 {
			m.cursor++
		}
		return m, nil
	case "g", "home":
		m.cursor = 0
		return m, nil
	case "G", "end":
		m.cursor = len(m.results) - 1
		return m, nil
	case "enter":
		if len(m.results) == 0 {
			return m, nil
		}
		return m, m.playCmd(m.cursor)
	case " ", "p":
		if m.player.HasTrack() {
			m.paused = m.player.TogglePause()
		}
		return m, nil
	case "s":
		m.player.Stop()
		m.playing = -1
		m.elapsed, m.total = 0, 0
		return m, nil
	case "n":
		if m.playing >= 0 && m.playing+1 < len(m.results) {
			return m, m.playCmd(m.playing + 1)
		}
		return m, nil
	case "b":
		if m.playing > 0 {
			return m, m.playCmd(m.playing - 1)
		}
		return m, nil
	case "+", "=":
		m.volume = m.player.VolumeUp()
		return m, nil
	case "-", "_":
		m.volume = m.player.VolumeDown()
		return m, nil
	case "a":
		m.autoplay = !m.autoplay
		return m, nil
	}
	return m, nil
}
