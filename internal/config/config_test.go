package config

import "testing"

func TestEnvOverride(t *testing.T) {
	t.Setenv("SPOTIFY_CLIENT_ID", "abc123")
	c := Load()
	if c.ClientID != "abc123" {
		t.Fatalf("env override: got %q want abc123", c.ClientID)
	}
	if !c.Configured() {
		t.Error("Configured() should be true when ClientID is set")
	}
}

func TestConfigured(t *testing.T) {
	if (&Config{}).Configured() {
		t.Error("empty config should not be Configured")
	}
	if !(&Config{ClientID: "x"}).Configured() {
		t.Error("config with ClientID should be Configured")
	}
}

func TestScopesIncludeStreaming(t *testing.T) {
	found := false
	for _, s := range Scopes {
		if s == "streaming" {
			found = true
		}
	}
	if !found {
		t.Error("streaming scope is required for librespot playback")
	}
}
