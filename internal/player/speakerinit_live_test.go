//go:build !nosound

package player

import (
	"testing"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
)

// TestSpeakerInitLive verifies the audio device can actually be opened.
// Skipped with -short. Requires a working audio backend (e.g. PulseAudio).
func TestSpeakerInitLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live speaker init in -short mode")
	}
	rate := beep.SampleRate(44100)
	if err := speaker.Init(rate, rate.N(time.Second/10)); err != nil {
		t.Fatalf("speaker init failed: %v", err)
	}
	t.Log("speaker initialized successfully")
}
