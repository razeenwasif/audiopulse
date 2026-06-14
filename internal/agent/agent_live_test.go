package agent

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestInterpretLive classifies a batch of real phrasings against the local
// Ollama model using the exact production prompt, to spot misclassifications.
// Opt-in: set AUDIOPULSE_OLLAMA_LIVE=1 (and have Ollama running) to run it.
//
//	AUDIOPULSE_OLLAMA_LIVE=1 go test ./internal/agent/ -run TestInterpretLive -v
func TestInterpretLive(t *testing.T) {
	if os.Getenv("AUDIOPULSE_OLLAMA_LIVE") == "" {
		t.Skip("set AUDIOPULSE_OLLAMA_LIVE=1 to run the live Ollama classification test")
	}
	c := New(os.Getenv("AUDIOPULSE_OLLAMA_URL"), os.Getenv("AUDIOPULSE_OLLAMA_MODEL"), "")
	utterances := []string{
		"pause the song",
		"pause",
		"stop the music",
		"resume",
		"continue playing",
		"skip this one",
		"go back",
		"play wish you were here by pink floyd",
		"recommend something like radiohead",
		"play something like daft punk",
		"suggest some chill study music",
		"how many radiohead songs do i have",
		"what playlists do i have",
		"rebuild the library index",
		"shuffle on",
		"turn off shuffle",
		"loop this song",
		"repeat the whole playlist",
		"turn it up",
		"set the volume to 30",
	}
	for _, u := range utterances {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		cmd, err := c.Interpret(ctx, u)
		cancel()
		if err != nil {
			t.Fatalf("Interpret(%q): %v", u, err)
		}
		t.Logf("%-32q -> action=%-9s query=%-28q on=%v repeat=%s vol=%d",
			u, cmd.Action, cmd.Query, cmd.On, cmd.Repeat, cmd.Volume)
	}
}
