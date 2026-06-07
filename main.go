// Command audiopulse is a terminal music player.
//
// With a Spotify Client ID configured it plays full songs from your Spotify
// account (via an embedded librespot device controlled through the Web API).
// Without one it falls back to a no-login Deezer "guest" mode that plays
// 30-second previews.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"audiopulse/internal/auth"
	"audiopulse/internal/config"
	"audiopulse/internal/librespot"
	"audiopulse/internal/spotify"
	"audiopulse/internal/ui"
)

// cellAspect resolves the terminal character-cell height/width ratio used to
// keep album art square. Order: AUDIOPULSE_CELL_ASPECT env override → detected
// pixel size → a sensible default for common terminals.
func cellAspect() float64 {
	clamp := func(f float64) float64 {
		if f < 1.5 {
			return 1.5
		}
		if f > 3.0 {
			return 3.0
		}
		return f
	}
	if v := os.Getenv("AUDIOPULSE_CELL_ASPECT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return clamp(f)
		}
	}
	if a, ok := terminalCellAspect(); ok {
		return clamp(a)
	}
	return 2.0
}

func main() {
	cfg := config.Load()

	if cfg.Configured() && os.Getenv("AUDIOPULSE_GUEST") == "" {
		if err := runSpotify(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "\nSpotify mode unavailable:", err)
			fmt.Fprintln(os.Stderr, "Falling back to Deezer guest mode…")
			runDeezer()
		}
		return
	}
	runDeezer()
}

// runDeezer runs the no-login preview UI.
func runDeezer() {
	restore := silenceNativeStderr()
	p := tea.NewProgram(ui.New(), tea.WithAltScreen())
	_, err := p.Run()
	restore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "audiopulse:", err)
		os.Exit(1)
	}
}

// runSpotify performs all interactive setup (OAuth, librespot login, device
// discovery) BEFORE entering the alt-screen, then runs the Spotify UI.
func runSpotify(cfg *config.Config) error {
	ctx := context.Background()

	// 1. Web API authorization (opens a browser on first run).
	client, user, err := connectSpotify(ctx, cfg.ClientID)
	if err != nil {
		return fmt.Errorf("spotify sign-in: %w", err)
	}
	fmt.Printf("Signed in as %s.\n", user)

	// 2. Embedded librespot playback device.
	sup, err := librespot.New()
	if err != nil {
		return err
	}
	if err := sup.EnsureCredentials(ctx); err != nil {
		return err
	}
	if err := sup.Start(); err != nil {
		return err
	}
	defer sup.Stop()

	// 3. Find the librespot device and make it the active one.
	fmt.Println("Connecting the AudioPulse playback device…")
	deviceID, err := client.WaitForDevice(ctx, config.DeviceName, 25*time.Second)
	if err != nil {
		return fmt.Errorf("playback device: %w (see %s)", err, sup.LogPath())
	}
	_ = client.Transfer(ctx, deviceID, false)

	// 4. Run the TUI (native-library noise redirected to a log).
	restore := silenceNativeStderr()
	p := tea.NewProgram(ui.NewSpotify(client, deviceID, user, cellAspect()), tea.WithAltScreen())
	_, runErr := p.Run()
	restore()
	sup.Stop()
	if runErr != nil {
		return runErr
	}
	return nil
}

// connectSpotify builds an authorized client, re-authorizing once if a cached
// token is no longer valid.
func connectSpotify(ctx context.Context, clientID string) (*spotify.Client, string, error) {
	httpClient, err := auth.HTTPClient(ctx, clientID)
	if err != nil {
		return nil, "", err
	}
	client := spotify.New(httpClient)
	user, err := client.Me(ctx)
	if err != nil {
		// Token likely expired/revoked — force a fresh authorization.
		if _, aerr := auth.Authorize(ctx, clientID); aerr != nil {
			return nil, "", aerr
		}
		httpClient, err = auth.HTTPClient(ctx, clientID)
		if err != nil {
			return nil, "", err
		}
		client = spotify.New(httpClient)
		if user, err = client.Me(ctx); err != nil {
			return nil, "", err
		}
	}
	return client, user, nil
}
