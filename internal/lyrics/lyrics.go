// Package lyrics fetches song lyrics from lrclib.net, a free, no-auth lyrics
// service that returns both plain and time-synced (LRC) lyrics. The Spotify Web
// API does not expose lyrics, so this is an independent best-effort lookup keyed
// on track/artist/album/duration.
package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL   = "https://lrclib.net"
	userAgent = "AudioPulse (https://github.com/razeenwasif/audiopulse)"
)

// Line is a single lyric line. At is its timestamp for synced lyrics, or -1 for
// plain (untimed) lyrics.
type Line struct {
	At   time.Duration
	Text string
}

// Result is the outcome of a lyrics lookup.
type Result struct {
	Synced       bool   // true when Lines carry timestamps
	Instrumental bool   // the track is marked instrumental (no lyrics by design)
	Lines        []Line // empty when nothing was found
}

// apiResponse mirrors the lrclib /api/get and /api/search item shape.
type apiResponse struct {
	ID           int    `json:"id"`
	Instrumental bool   `json:"instrumental"`
	PlainLyrics  string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
}

// Fetch looks up lyrics for a track. It tries the exact /api/get endpoint first
// (which matches on duration), then falls back to a fuzzy /api/search. It
// returns an empty (non-error) Result when nothing matches.
func Fetch(ctx context.Context, artist, track, album string, dur time.Duration) (Result, error) {
	if strings.TrimSpace(track) == "" {
		return Result{}, nil
	}

	if r, ok, err := fetchExact(ctx, artist, track, album, dur); err != nil {
		return Result{}, err
	} else if ok {
		return r, nil
	}
	return fetchSearch(ctx, artist, track)
}

func fetchExact(ctx context.Context, artist, track, album string, dur time.Duration) (Result, bool, error) {
	q := url.Values{}
	q.Set("track_name", track)
	q.Set("artist_name", artist)
	if album != "" {
		q.Set("album_name", album)
	}
	if dur > 0 {
		q.Set("duration", strconv.Itoa(int(dur.Seconds())))
	}
	var resp apiResponse
	status, err := getJSON(ctx, "/api/get?"+q.Encode(), &resp)
	if err != nil {
		return Result{}, false, err
	}
	if status == http.StatusNotFound {
		return Result{}, false, nil
	}
	if status != http.StatusOK {
		return Result{}, false, fmt.Errorf("lrclib: unexpected status %d", status)
	}
	return resultFrom(resp), true, nil
}

func fetchSearch(ctx context.Context, artist, track string) (Result, error) {
	q := url.Values{}
	q.Set("track_name", track)
	if artist != "" {
		q.Set("artist_name", artist)
	}
	var results []apiResponse
	status, err := getJSON(ctx, "/api/search?"+q.Encode(), &results)
	if err != nil {
		return Result{}, err
	}
	if status != http.StatusOK || len(results) == 0 {
		return Result{}, nil
	}
	// Prefer the first result that actually has lyrics.
	for _, r := range results {
		if r.SyncedLyrics != "" || r.PlainLyrics != "" || r.Instrumental {
			return resultFrom(r), nil
		}
	}
	return Result{}, nil
}

func resultFrom(r apiResponse) Result {
	if r.Instrumental {
		return Result{Instrumental: true}
	}
	if synced := parseLRC(r.SyncedLyrics); len(synced) > 0 {
		return Result{Synced: true, Lines: synced}
	}
	return Result{Lines: parsePlain(r.PlainLyrics)}
}

func getJSON(ctx context.Context, path string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return res.StatusCode, nil
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return res.StatusCode, err
	}
	return res.StatusCode, nil
}

var lrcTag = regexp.MustCompile(`\[(\d{1,2}):(\d{2})(?:[.:](\d{1,3}))?\]`)

// parseLRC parses LRC-format synced lyrics into timestamped lines, sorted by
// time. A single LRC line may carry several timestamps (repeated lines).
func parseLRC(s string) []Line {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []Line
	for _, raw := range strings.Split(s, "\n") {
		stamps := lrcTag.FindAllStringSubmatch(raw, -1)
		if len(stamps) == 0 {
			continue
		}
		text := strings.TrimSpace(lrcTag.ReplaceAllString(raw, ""))
		for _, m := range stamps {
			mm, _ := strconv.Atoi(m[1])
			ss, _ := strconv.Atoi(m[2])
			frac := 0.0
			if m[3] != "" {
				// Normalize 2- or 3-digit fractional seconds.
				if v, err := strconv.Atoi(m[3]); err == nil {
					frac = float64(v) / pow10(len(m[3]))
				}
			}
			at := time.Duration(mm)*time.Minute +
				time.Duration(ss)*time.Second +
				time.Duration(frac*float64(time.Second))
			out = append(out, Line{At: at, Text: text})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

func parsePlain(s string) []Line {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []Line
	for _, raw := range strings.Split(s, "\n") {
		out = append(out, Line{At: -1, Text: strings.TrimRight(raw, "\r")})
	}
	return out
}

func pow10(n int) float64 {
	p := 1.0
	for i := 0; i < n; i++ {
		p *= 10
	}
	return p
}
