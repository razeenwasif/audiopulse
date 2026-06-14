package library_test

import (
	"context"
	"os"
	"testing"
	"time"

	"audiopulse/internal/agent"
	"audiopulse/internal/auth"
	"audiopulse/internal/config"
	"audiopulse/internal/library"
	"audiopulse/internal/spotify"
)

// TestRAGLive exercises the whole Phase-A pipeline against the real Spotify
// library + local Ollama: build the index, semantic-search it, and generate
// recommendations. Opt-in (needs the cached token + Ollama running):
//
//	AUDIOPULSE_RAG_LIVE=1 go test ./internal/library/ -run TestRAGLive -v
func TestRAGLive(t *testing.T) {
	if os.Getenv("AUDIOPULSE_RAG_LIVE") == "" {
		t.Skip("set AUDIOPULSE_RAG_LIVE=1 to run the live RAG pipeline test")
	}
	cfg := config.Load()
	if cfg.ClientID == "" {
		t.Skip("no Spotify client id configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	httpClient, err := auth.HTTPClient(ctx, cfg.ClientID)
	if err != nil {
		t.Skipf("no cached Spotify token: %v", err)
	}
	client := spotify.New(httpClient)
	ag := agent.New(cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaEmbedModel)

	t0 := time.Now()
	ix, err := library.Build(ctx, client, ag, func(done, total int) {
		if total > 0 && (done == total || done%256 == 0) {
			t.Logf("  embedding %d/%d", done, total)
		}
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	t.Logf("indexed %d tracks (dim=%d) in %s", len(ix.Records), ix.Dim, time.Since(t0).Round(time.Second))
	if len(ix.Records) == 0 {
		t.Fatal("index is empty")
	}

	// Semantic search.
	seed := "daft punk electronic"
	vecs, err := ag.Embed(ctx, []string{seed})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	t.Logf("nearest in library to %q:", seed)
	var taste []string
	for _, s := range ix.Search(vecs[0], 8) {
		t.Logf("  %.3f  %s", s.Score, s.Record.Label())
		taste = append(taste, s.Record.Label())
	}

	// Recommendations (discovery).
	sugs, err := ag.Recommend(ctx, "something like daft punk", taste, 8)
	if err != nil {
		t.Fatalf("Recommend: %v", err)
	}
	t.Logf("recommended (%d):", len(sugs))
	resolved := 0
	for _, s := range sugs {
		q := s.Title + " " + s.Artist
		res, err := client.Search(ctx, q)
		hit := "—"
		if err == nil && len(res) > 0 {
			hit = res[0].Title + " / " + res[0].Artist
			resolved++
		}
		t.Logf("  %-40s -> %s", s.Title+" — "+s.Artist, hit)
	}
	if resolved == 0 {
		t.Error("no recommendations resolved via Spotify search")
	}

	// Grounded chat (multi-turn).
	var history []agent.Turn
	for _, q := range []string{
		"what kind of music is in my library?",
		"name a few artists I clearly like",
	} {
		lines := ix.Context(ctx, ag, q, 20)
		ans, err := ag.Answer(ctx, q, lines, history)
		if err != nil {
			t.Fatalf("Answer(%q): %v", q, err)
		}
		t.Logf("Q: %s\n   A: %s", q, ans)
		if ans == "" {
			t.Errorf("empty answer for %q", q)
		}
		history = append(history, agent.Turn{Role: "user", Text: q}, agent.Turn{Role: "assistant", Text: ans})
	}
}
