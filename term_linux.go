//go:build linux

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// terminalCellAspect returns the character cell's height/width ratio derived
// from the terminal's reported pixel size, if available. Many terminals
// (including Windows Terminal via ConPTY) report zero pixel size, in which case
// ok is false and the caller should use a default.
func terminalCellAspect() (ratio float64, ok bool) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Xpixel == 0 || ws.Ypixel == 0 || ws.Col == 0 || ws.Row == 0 {
		return 0, false
	}
	cellW := float64(ws.Xpixel) / float64(ws.Col)
	cellH := float64(ws.Ypixel) / float64(ws.Row)
	if cellW <= 0 {
		return 0, false
	}
	return cellH / cellW, true
}
