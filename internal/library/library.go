// Package library builds and queries a local semantic index of the user's
// Spotify library (playlists + Liked Songs) for RAG: recommendations and
// library chat. Track metadata is gathered via the Spotify Web API, embedded
// with a local Ollama model (nomic-embed-text), and searched by cosine
// similarity — all on-device. The index is persisted under the config dir so it
// is built once and reused.
package library

import (
	"context"
	"crypto/sha1"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"audiopulse/internal/spotify"

	zspotify "github.com/zmb3/spotify/v2"
)

// Record is one indexed track.
type Record struct {
	ID        string
	URI       string
	Title     string
	Artist    string
	Album     string
	Playlists []string  // playlist names containing it ("Liked Songs" included)
	Vec       []float32 // L2-normalized embedding
}

// Label is the "Title — Artist" form used in taste/context prompts.
func (r Record) Label() string {
	if r.Artist == "" {
		return r.Title
	}
	return r.Title + " — " + r.Artist
}

// embedText is the string fed to the embedding model for a record.
func embedText(r Record) string {
	s := r.Title + " — " + r.Artist
	if r.Album != "" {
		s += " (album: " + r.Album + ")"
	}
	return s
}

// Index is the persisted semantic index of the library.
type Index struct {
	Records   []Record
	Signature string // fingerprint of the source library (staleness check)
	Built     string // RFC3339 build time (informational)
	Dim       int    // embedding dimension
}

// Embedder turns strings into vectors (implemented by internal/agent.Client).
type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

// Spotify is the subset of the Spotify client the gather step needs.
type Spotify interface {
	Playlists(ctx context.Context) ([]spotify.Playlist, error)
	LikedSongsPage(ctx context.Context, offset int) (spotify.TrackPage, error)
	PlaylistTracksPage(ctx context.Context, id zspotify.ID, offset int) (spotify.TrackPage, error)
}

// embedBatch is how many tracks are embedded per Ollama call.
const embedBatch = 64

// Build gathers the library, embeds every track, and returns an index. onProgress
// (if non-nil) is called as embedding advances (done, total).
func Build(ctx context.Context, sp Spotify, emb Embedder, onProgress func(done, total int)) (*Index, error) {
	recs, sig, err := gather(ctx, sp)
	if err != nil {
		return nil, err
	}
	total := len(recs)
	if onProgress != nil {
		onProgress(0, total)
	}
	dim := 0
	for i := 0; i < total; i += embedBatch {
		end := min(i+embedBatch, total)
		inputs := make([]string, end-i)
		for j := i; j < end; j++ {
			inputs[j-i] = embedText(recs[j])
		}
		vecs, err := emb.Embed(ctx, inputs)
		if err != nil {
			return nil, fmt.Errorf("embedding tracks: %w", err)
		}
		for j := i; j < end; j++ {
			v := vecs[j-i]
			normalize(v)
			recs[j].Vec = v
			dim = len(v)
		}
		if onProgress != nil {
			onProgress(end, total)
		}
	}
	return &Index{
		Records:   recs,
		Signature: sig,
		Built:     time.Now().Format(time.RFC3339),
		Dim:       dim,
	}, nil
}

// gather collects full track metadata + playlist membership across Liked Songs
// and every playlist, deduped by track ID, plus a staleness signature.
func gather(ctx context.Context, sp Spotify) ([]Record, string, error) {
	playlists, err := sp.Playlists(ctx)
	if err != nil {
		return nil, "", err
	}

	byID := make(map[string]*Record)
	var order []string
	add := func(t spotify.Track, playlist string) {
		id := string(t.ID)
		if id == "" {
			return
		}
		r, ok := byID[id]
		if !ok {
			r = &Record{ID: id, URI: string(t.URI), Title: t.Title, Artist: t.Artist, Album: t.Album}
			byID[id] = r
			order = append(order, id)
		}
		if playlist != "" && !contains(r.Playlists, playlist) {
			r.Playlists = append(r.Playlists, playlist)
		}
	}

	for offset := 0; ; {
		page, err := sp.LikedSongsPage(ctx, offset)
		if err != nil {
			return nil, "", err
		}
		for _, t := range page.Tracks {
			add(t, "Liked Songs")
		}
		if !page.HasMore() {
			break
		}
		offset = page.NextOffset()
	}

	sigParts := make([]string, 0, len(playlists))
	for _, p := range playlists {
		sigParts = append(sigParts, string(p.ID)+":"+strconv.Itoa(p.Count))
		for offset := 0; ; {
			page, err := sp.PlaylistTracksPage(ctx, p.ID, offset)
			if err != nil {
				break // skip a failed playlist, keep going
			}
			for _, t := range page.Tracks {
				add(t, p.Name)
			}
			if !page.HasMore() {
				break
			}
			offset = page.NextOffset()
		}
	}

	recs := make([]Record, 0, len(order))
	for _, id := range order {
		recs = append(recs, *byID[id])
	}
	return recs, signature(sigParts, len(recs)), nil
}

// signature fingerprints the library so a changed library (added/removed
// playlists or tracks) is detected and the index rebuilt.
func signature(playlistParts []string, trackCount int) string {
	sorted := append([]string(nil), playlistParts...)
	sort.Strings(sorted)
	h := sha1.New()
	fmt.Fprintf(h, "tracks=%d\n", trackCount)
	for _, p := range sorted {
		h.Write([]byte(p))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Stale reports whether the index no longer matches the current library
// signature (so it should be rebuilt).
func (ix *Index) Stale(currentSignature string) bool {
	return ix == nil || ix.Signature != currentSignature
}

// Scored is a record paired with its similarity to a query (1.0 = identical).
type Scored struct {
	Record Record
	Score  float32
}

// Search returns the k records most similar to vec (cosine; vectors are stored
// normalized, so this is a dot product), best first.
func (ix *Index) Search(vec []float32, k int) []Scored {
	q := append([]float32(nil), vec...)
	normalize(q)
	scored := make([]Scored, 0, len(ix.Records))
	for _, r := range ix.Records {
		if len(r.Vec) != len(q) {
			continue
		}
		scored = append(scored, Scored{Record: r, Score: dot(r.Vec, q)})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
	if k > 0 && len(scored) > k {
		scored = scored[:k]
	}
	return scored
}

// Filter returns records whose title/artist/album contains q (case-insensitive).
// Used for exact "how many X" style questions where similarity isn't enough.
func (ix *Index) Filter(q string) []Record {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var out []Record
	for _, r := range ix.Records {
		hay := strings.ToLower(r.Title + " " + r.Artist + " " + r.Album)
		if strings.Contains(hay, q) {
			out = append(out, r)
		}
	}
	return out
}

// Sample returns up to n records spread evenly across the library — a
// representative taste signal when there is no specific seed.
func (ix *Index) Sample(n int) []Record {
	total := len(ix.Records)
	if n <= 0 || total == 0 {
		return nil
	}
	if total <= n {
		return append([]Record(nil), ix.Records...)
	}
	step := float64(total) / float64(n)
	out := make([]Record, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ix.Records[int(float64(i)*step)])
	}
	return out
}

// Context builds grounding lines for a chat question: a one-line library
// summary, the semantically nearest tracks (via emb), and keyword matches (which
// help exact "how many X" / specific-artist questions), capped.
func (ix *Index) Context(ctx context.Context, emb Embedder, question string, k int) []string {
	if ix == nil {
		return nil
	}
	const maxLines = 60
	lines := []string{ix.summary()}
	seen := make(map[string]bool)
	add := func(r Record) bool {
		if seen[r.ID] {
			return true
		}
		seen[r.ID] = true
		s := r.Label()
		if len(r.Playlists) > 0 {
			s += " [" + strings.Join(r.Playlists, ", ") + "]"
		}
		lines = append(lines, s)
		return len(lines) < maxLines
	}
	if emb != nil {
		if vecs, err := emb.Embed(ctx, []string{question}); err == nil && len(vecs) > 0 {
			for _, sc := range ix.Search(vecs[0], k) {
				if !add(sc.Record) {
					return lines
				}
			}
		}
	}
	for _, w := range salientWords(question) {
		for _, r := range ix.Filter(w) {
			if !add(r) {
				return lines
			}
		}
	}
	return lines
}

// summary is a one-line description of the library (count + playlists).
func (ix *Index) summary() string {
	pls := ix.PlaylistNames()
	if len(pls) > 30 {
		extra := len(pls) - 30
		pls = append(pls[:30:30], fmt.Sprintf("…and %d more", extra))
	}
	return fmt.Sprintf("The library has %d tracks across these playlists: %s.",
		len(ix.Records), strings.Join(pls, ", "))
}

// chatStopwords are dropped when extracting keywords from a question.
var chatStopwords = map[string]bool{
	"the": true, "a": true, "an": true, "do": true, "have": true, "how": true,
	"many": true, "what": true, "which": true, "are": true, "any": true,
	"songs": true, "song": true, "track": true, "tracks": true, "playlist": true,
	"playlists": true, "music": true, "some": true, "and": true, "with": true,
	"got": true, "for": true, "you": true, "can": true, "give": true, "show": true,
}

// salientWords pulls the meaningful (≥3-char, non-stopword) tokens from a query.
func salientWords(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(w) >= 3 && !chatStopwords[w] {
			out = append(out, w)
		}
	}
	return out
}

// PlaylistNames returns the distinct playlist names in the library.
func (ix *Index) PlaylistNames() []string {
	seen := make(map[string]bool)
	var out []string
	for _, r := range ix.Records {
		for _, p := range r.Playlists {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Save writes the index to path using gob (compact for float32 vectors).
func (ix *Index) Save(path string) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(ix); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// Load reads an index previously written by Save. A missing file returns
// (nil, nil) so callers can treat "no index yet" as a normal state.
func Load(path string) (*Index, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ix Index
	if err := gob.NewDecoder(f).Decode(&ix); err != nil {
		return nil, err
	}
	return &ix, nil
}

// --- vector helpers ----------------------------------------------------------

func normalize(v []float32) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return
	}
	inv := float32(1 / math.Sqrt(sum))
	for i := range v {
		v[i] *= inv
	}
}

func dot(a, b []float32) float32 {
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
