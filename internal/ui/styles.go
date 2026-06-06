package ui

import "github.com/charmbracelet/lipgloss"

// Spotify-inspired palette.
var (
	colorAccent   = lipgloss.Color("#1DB954") // Spotify green
	colorAccentHi = lipgloss.Color("#1ED760") // brighter green for highlights
	colorText     = lipgloss.Color("#FFFFFF")
	colorMuted    = lipgloss.Color("#B3B3B3")
	colorFaint    = lipgloss.Color("#535353")
	colorBg       = lipgloss.Color("#121212")
	colorPanel    = lipgloss.Color("#181818")
	colorErr      = lipgloss.Color("#F15E6C")
)

type styles struct {
	app        lipgloss.Style
	title      lipgloss.Style
	sidebar    lipgloss.Style
	sidebarSel lipgloss.Style
	main       lipgloss.Style
	rowSel     lipgloss.Style
	rowTitle   lipgloss.Style
	rowArtist  lipgloss.Style
	rowMuted   lipgloss.Style
	nowBar     lipgloss.Style
	nowTitle   lipgloss.Style
	nowArtist  lipgloss.Style
	help       lipgloss.Style
	helpKey    lipgloss.Style
	errText    lipgloss.Style
	barFill    lipgloss.Style
	barEmpty   lipgloss.Style
}

func newStyles() styles {
	panelBorder := lipgloss.RoundedBorder()
	return styles{
		title: lipgloss.NewStyle().
			Foreground(colorAccentHi).Bold(true),

		sidebar: lipgloss.NewStyle().
			Border(panelBorder).BorderForeground(colorAccent).
			Padding(1, 2),

		sidebarSel: lipgloss.NewStyle().
			Foreground(colorAccentHi).Bold(true),

		main: lipgloss.NewStyle().
			Border(panelBorder).BorderForeground(colorAccent).
			Padding(0, 1),

		rowSel: lipgloss.NewStyle().
			Foreground(colorAccentHi).Bold(true),

		rowTitle:  lipgloss.NewStyle().Foreground(colorText),
		rowArtist: lipgloss.NewStyle().Foreground(colorMuted),
		rowMuted:  lipgloss.NewStyle().Foreground(colorFaint),

		nowBar: lipgloss.NewStyle().
			Border(panelBorder).BorderForeground(colorAccent).
			Padding(0, 2),

		nowTitle:  lipgloss.NewStyle().Foreground(colorText).Bold(true),
		nowArtist: lipgloss.NewStyle().Foreground(colorMuted),

		help:    lipgloss.NewStyle().Foreground(colorFaint),
		helpKey: lipgloss.NewStyle().Foreground(colorMuted).Bold(true),

		errText: lipgloss.NewStyle().Foreground(colorErr),

		barFill:  lipgloss.NewStyle().Foreground(colorAccent),
		barEmpty: lipgloss.NewStyle().Foreground(colorFaint),
	}
}
