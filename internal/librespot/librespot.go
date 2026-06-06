// Package librespot supervises a librespot child process that acts as the
// "AudioPulse" Spotify Connect device. librespot streams and decodes the audio;
// AudioPulse controls it through the Spotify Web API.
//
// Audio is emitted via librespot's ALSA backend, which (on this setup) routes
// through ~/.asoundrc to PulseAudio. Credentials are cached under the cache dir
// so the interactive device login is a one-time step.
package librespot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"audiopulse/internal/config"
)

// Supervisor manages the librespot process lifecycle.
type Supervisor struct {
	bin      string
	cacheDir string
	logPath  string
	cmd      *exec.Cmd
}

// Find locates the librespot binary on PATH or in common install locations.
func Find() (string, error) {
	if p, err := exec.LookPath("librespot"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, ".cargo", "bin", "librespot"),
		"/usr/local/bin/librespot",
	} {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("librespot not found — run 'make librespot' to install it")
}

// New creates a Supervisor, locating the binary and preparing the cache dir.
func New() (*Supervisor, error) {
	bin, err := Find()
	if err != nil {
		return nil, err
	}
	cache, err := config.LibrespotCacheDir()
	if err != nil {
		return nil, err
	}
	tmp := os.TempDir()
	return &Supervisor{
		bin:      bin,
		cacheDir: cache,
		logPath:  filepath.Join(tmp, "audiopulse-librespot.log"),
	}, nil
}

// LogPath is where the running device's logs are written.
func (s *Supervisor) LogPath() string { return s.logPath }

// HasCredentials reports whether librespot has cached login credentials.
func (s *Supervisor) HasCredentials() bool {
	_, err := os.Stat(filepath.Join(s.cacheDir, "credentials.json"))
	return err == nil
}

func (s *Supervisor) baseArgs() []string {
	return []string{
		"--name", config.DeviceName,
		"--backend", "alsa",
		"--bitrate", "320",
		"--cache", s.cacheDir,
		"--disable-audio-cache",
		"--autoplay", "off",
	}
}

// EnsureCredentials runs the one-time interactive OAuth device login if no
// credentials are cached yet. It attaches to the terminal so the user can see
// and complete the browser prompt, then stops once credentials are written.
//
// Must be called BEFORE the alt-screen TUI starts.
func (s *Supervisor) EnsureCredentials(ctx context.Context) error {
	if s.HasCredentials() {
		return nil
	}

	fmt.Println("\n♫ First-time setup: authorizing the AudioPulse playback device.")
	fmt.Println("A browser will open — approve access for librespot. This is a one-time step.")

	args := append(s.baseArgs(), "--enable-oauth")
	cmd := exec.CommandContext(ctx, s.bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting librespot for login: %w", err)
	}

	// Wait until credentials are cached (or time out), then stop this instance.
	deadline := time.Now().Add(3 * time.Minute)
	for {
		if s.HasCredentials() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			fmt.Println("Playback device authorized ✓")
			return nil
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return errors.New("timed out waiting for librespot device authorization")
		}
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Start launches the long-lived device process using cached credentials,
// writing logs to LogPath(). Call Stop on shutdown.
func (s *Supervisor) Start() error {
	if s.cmd != nil {
		return errors.New("librespot already started")
	}
	logFile, err := os.Create(s.logPath)
	if err != nil {
		return fmt.Errorf("creating librespot log: %w", err)
	}

	cmd := exec.Command(s.bin, s.baseArgs()...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("starting librespot: %w", err)
	}
	s.cmd = cmd
	return nil
}

// Stop terminates the librespot process if running.
func (s *Supervisor) Stop() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	_ = s.cmd.Process.Kill()
	_ = s.cmd.Wait()
	s.cmd = nil
}
