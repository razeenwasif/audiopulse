// Package config resolves AudioPulse's runtime configuration and the on-disk
// locations it uses for the Spotify integration (OAuth token, librespot cache).
//
// The Spotify Client ID is a public PKCE client identifier — it is not a secret.
// It is read from the SPOTIFY_CLIENT_ID environment variable or from
// ~/.config/audiopulse/config.json. No client secret is ever stored.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	// AppName is the config-directory name and the librespot device name base.
	AppName = "audiopulse"
	// DeviceName is how the librespot playback device appears in Spotify.
	DeviceName = "AudioPulse"

	// CallbackAddr is the local address the OAuth redirect server listens on.
	CallbackAddr = "127.0.0.1:8888"
	// RedirectURI must be registered verbatim in the Spotify developer app.
	RedirectURI = "http://127.0.0.1:8888/callback"
)

// Scopes are the Spotify OAuth scopes AudioPulse requests.
var Scopes = []string{
	"user-read-private",
	"user-read-email",
	"playlist-read-private",
	"playlist-read-collaborative",
	"user-library-read",
	"user-library-modify", // like/unlike tracks, follow/unfollow shows
	"user-read-playback-state",
	"user-modify-playback-state",
	"user-read-currently-playing",
	"user-read-recently-played",
	"user-top-read",
	"streaming",
}

// Config holds user-provided settings.
type Config struct {
	ClientID string `json:"client_id"`
	// MusicDir is where the local library / downloads live. Empty → the default
	// ~/Music/audiopulse. Set it (e.g. "/mnt/e/Music") to use another drive.
	MusicDir string `json:"music_dir,omitempty"`
}

// MusicPath resolves the music library directory (expanding a leading ~) and
// creates it. Defaults to ~/Music/audiopulse when unset.
func (c *Config) MusicPath() (string, error) {
	dir := c.MusicDir
	switch {
	case dir == "":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, "Music", "audiopulse")
	case strings.HasPrefix(dir, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, dir[2:])
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Dir returns ~/.config/audiopulse, creating it (0700) if needed.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, AppName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// TokenPath is where the OAuth token is cached.
func TokenPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "token.json"), nil
}

// ScopesPath is where the granted-scopes fingerprint is recorded, so a scope
// change (a new app version) can force a one-time re-authorization.
func ScopesPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "scopes"), nil
}

// LibrespotCacheDir is where librespot caches its credentials and audio.
func LibrespotCacheDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	cache := filepath.Join(dir, "librespot")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return "", err
	}
	return cache, nil
}

// Load reads config.json (if present) and applies the SPOTIFY_CLIENT_ID
// environment override. A missing config file is not an error.
func Load() *Config {
	c := &Config{}
	if dir, err := Dir(); err == nil {
		if b, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
			_ = json.Unmarshal(b, c)
		}
	}
	if v := os.Getenv("SPOTIFY_CLIENT_ID"); v != "" {
		c.ClientID = v
	}
	return c
}

// Save writes config.json with 0600 permissions.
func (c *Config) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), b, 0o600)
}

// Configured reports whether a Spotify Client ID is available.
func (c *Config) Configured() bool { return c.ClientID != "" }
