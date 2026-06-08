package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Spotify-inspired palette.
var (
	colorAccent   = lipgloss.Color("#1DB954") // Spotify green
	colorAccentHi = lipgloss.Color("#1ED760") // brighter green for highlights
	colorAccentLt = lipgloss.Color("#7EE787") // light/pale green (now-playing + visualizer borders)
	colorVizTop   = lipgloss.Color("#B6F2C4") // palest green, visualizer bar caps
	colorText     = lipgloss.Color("#FFFFFF")
	colorMuted    = lipgloss.Color("#B3B3B3")
	colorFaint    = lipgloss.Color("#535353")
	colorBg       = lipgloss.Color("#121212")
	colorPanel    = lipgloss.Color("#181818")
	colorCard     = lipgloss.Color("#1F1F1F") // selected/hover row background
	colorBorder   = lipgloss.Color("#2A2A2A") // subtle panel border (unfocused)
	colorBlack    = lipgloss.Color("#000000")
	colorErr      = lipgloss.Color("#F15E6C")
)

// thumbPalette gives library "thumbnails" varied, Spotify-like colors.
var thumbPalette = []lipgloss.Color{
	"#E13300", "#7358FF", "#1DB954", "#E8115B", "#509BF5", "#FF6437", "#BC5900", "#8C67AC",
}

// panelBox returns a rounded panel whose border is green when focused and a
// subtle gray otherwise — matching Spotify's "highlight the active area" look.
func panelBox(focused bool, padV, padH int) lipgloss.Style {
	c := colorBorder
	if focused {
		c = colorAccent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c).
		Padding(padV, padH)
}

// lightPanelBox is a rounded panel with a light-green border, used for the
// now-playing and visualizer panels on the right.
func lightPanelBox(padV, padH int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccentLt).
		Padding(padV, padH)
}

// keepTransparent leaves the terminal's background transparency intact when set
// (AUDIOPULSE_TRANSPARENT=1). By default AudioPulse paints an opaque backdrop so
// the player is readable over translucent/acrylic terminals.
var keepTransparent = os.Getenv("AUDIOPULSE_TRANSPARENT") != ""

// opaqueBGCode is the 24-bit SGR for the app background (#121212 = 18,18,18).
const opaqueBGCode = "\x1b[48;2;18;18;18m"

// fillBG forces every cell of the rendered frame to carry an explicit background
// color. Terminals (e.g. Windows Terminal) render cells with an explicit
// background opaquely, so this removes see-through transparency while the player
// runs — the effect is undone on quit when the alt-screen is restored.
//
// It re-establishes the background after every reset (so inner styles can't
// punch transparent holes) and pads each line to the full width.
func fillBG(frame string, w, h int) string {
	if keepTransparent {
		return frame
	}
	lines := strings.Split(frame, "\n")
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i := range lines {
		ln := strings.ReplaceAll(lines[i], "\x1b[0m", "\x1b[0m"+opaqueBGCode)
		if pad := w - lipgloss.Width(ln); pad > 0 {
			ln += strings.Repeat(" ", pad)
		}
		lines[i] = opaqueBGCode + ln + "\x1b[0m"
	}
	return strings.Join(lines, "\n")
}

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
