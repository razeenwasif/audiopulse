//go:build linux

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// silenceNativeStderr redirects the process's stderr (file descriptor 2) to a
// log file for the duration of the program.
//
// Native libraries — notably ALSA — write diagnostics straight to fd 2, bypassing
// Go's os.Stderr. In a full-screen TUI that output lands on the alternate screen
// and corrupts the layout. Redirecting fd 2 to a file keeps the UI clean while
// preserving the messages for debugging at $TMPDIR/audiopulse.log.
//
// It returns a function that restores the original stderr; the caller should
// invoke it before printing any fatal error.
func silenceNativeStderr() (restore func()) {
	noop := func() {}

	saved, err := unix.Dup(int(os.Stderr.Fd()))
	if err != nil {
		return noop
	}

	logPath := filepath.Join(os.TempDir(), "audiopulse.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		unix.Close(saved)
		return noop
	}

	if err := unix.Dup3(int(f.Fd()), int(os.Stderr.Fd()), 0); err != nil {
		f.Close()
		unix.Close(saved)
		return noop
	}

	var done bool
	return func() {
		if done {
			return
		}
		done = true
		unix.Dup3(saved, int(os.Stderr.Fd()), 0)
		unix.Close(saved)
		f.Close()
	}
}
