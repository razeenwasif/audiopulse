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

// Run launches the device process and keeps it alive, restarting it with
// exponential backoff if it exits unexpectedly, until ctx is cancelled. It is
// meant to run in its own goroutine for the lifetime of the app; cancelling ctx
// kills the child and returns. Logs (including restart notices) go to LogPath().
func (s *Supervisor) Run(ctx context.Context) {
	const (
		minBackoff   = time.Second
		maxBackoff   = 30 * time.Second
		healthyAfter = 10 * time.Second // a child that ran this long resets backoff
	)
	backoff := minBackoff
	for {
		start := time.Now()
		cmd, logFile, err := s.startOnce()
		if err != nil {
			s.logf("could not start librespot: %v", err)
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff = capDur(backoff*2, maxBackoff)
			continue
		}

		exited := make(chan error, 1)
		go func() { exited <- cmd.Wait() }()

		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			<-exited
			logFile.Close()
			return
		case werr := <-exited:
			logFile.Close()
			if ctx.Err() != nil {
				return
			}
			if time.Since(start) >= healthyAfter {
				backoff = minBackoff // ran fine for a while → recover quickly
			}
			s.logf("librespot exited (%v); restarting in %s", werr, backoff)
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff = capDur(backoff*2, maxBackoff)
		}
	}
}

// startOnce launches a single librespot process, appending output to the log.
func (s *Supervisor) startOnce() (*exec.Cmd, *os.File, error) {
	logFile, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("creating librespot log: %w", err)
	}
	cmd := exec.Command(s.bin, s.baseArgs()...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, nil, fmt.Errorf("starting librespot: %w", err)
	}
	return cmd, logFile, nil
}

func (s *Supervisor) logf(format string, a ...any) {
	f, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[audiopulse %s] "+format+"\n", append([]any{time.Now().Format("15:04:05")}, a...)...)
}

func capDur(d, max time.Duration) time.Duration {
	if d > max {
		return max
	}
	return d
}

// sleepCtx sleeps for d or until ctx is cancelled; it returns false if cancelled.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
