// Command audiopulse is a Spotify-style terminal music player. It searches the
// public Deezer catalogue and plays 30-second previews, all from the terminal.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"audiopulse/internal/ui"
)

func main() {
	// Keep native-library noise (e.g. ALSA) off the alternate screen.
	restore := silenceNativeStderr()

	p := tea.NewProgram(ui.New(), tea.WithAltScreen())
	_, err := p.Run()

	restore() // put stderr back before reporting anything to the user
	if err != nil {
		fmt.Fprintln(os.Stderr, "audiopulse:", err)
		os.Exit(1)
	}
}
