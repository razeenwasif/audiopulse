// Package deezer is a tiny client for the public Deezer API.
//
// The Deezer search endpoint requires no API key and returns, for each track,
// a 30-second `preview` MP3 URL that we can stream and play. See
// https://developers.deezer.com/api/search
package deezer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const apiBase = "https://api.deezer.com"

// Track is a single search result.
type Track struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Duration int    `json:"duration"` // full-track length in seconds (preview is 30s)
	Preview  string `json:"preview"`  // 30s MP3 URL
	Artist   struct {
		Name string `json:"name"`
	} `json:"artist"`
	Album struct {
		Title       string `json:"title"`
		CoverMedium string `json:"cover_medium"`
	} `json:"album"`
}

// ArtistName is a nil-safe accessor.
func (t Track) ArtistName() string { return t.Artist.Name }

// AlbumName is a nil-safe accessor.
func (t Track) AlbumName() string { return t.Album.Title }

type searchResponse struct {
	Data  []Track `json:"data"`
	Total int     `json:"total"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Client talks to the Deezer API.
type Client struct {
	http *http.Client
}

// New returns a Client with a sensible timeout.
func New() *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}}
}

// Search returns tracks matching the query. Tracks without a playable preview
// are filtered out so the UI never offers something it can't play.
func (c *Client) Search(ctx context.Context, query string) ([]Track, error) {
	if query == "" {
		return nil, nil
	}
	u := fmt.Sprintf("%s/search?q=%s&limit=50", apiBase, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deezer request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer returned status %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding deezer response: %w", err)
	}
	if sr.Error != nil {
		return nil, fmt.Errorf("deezer error: %s", sr.Error.Message)
	}

	tracks := make([]Track, 0, len(sr.Data))
	for _, t := range sr.Data {
		if t.Preview != "" {
			tracks = append(tracks, t)
		}
	}
	return tracks, nil
}
