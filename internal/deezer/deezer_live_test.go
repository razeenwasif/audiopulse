package deezer

import (
	"context"
	"testing"
	"time"
)

// TestSearchLive hits the real Deezer API. Skipped with -short.
func TestSearchLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live API test in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tracks, err := New().Search(ctx, "daft punk")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(tracks) == 0 {
		t.Fatal("expected at least one track")
	}
	for _, tr := range tracks {
		if tr.Preview == "" {
			t.Errorf("track %q has empty preview URL", tr.Title)
		}
	}
	t.Logf("got %d tracks; first: %q — %s (%s)",
		len(tracks), tracks[0].Title, tracks[0].ArtistName(), tracks[0].Preview)
}
