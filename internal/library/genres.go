package library

import (
	"sort"
	"strings"

	"audiopulse/internal/spotify"

	zspotify "github.com/zmb3/spotify/v2"
)

// genreRule maps a keyword found in an artist's Spotify genre tags to a coarse
// bucket. Within GenreBucket the *count* of matching tags decides the bucket
// (majority vote), and ties are broken by this list's order — so order still
// encodes priority (specific before generic: "k-pop" before "pop"). Spotify's
// genres are hyper-granular ("australian indie rock", "indietronica", "melodic
// phonk"), so substring matching against a curated list collapses them.
type genreRule struct {
	keyword string
	bucket  string
}

var genreRules = []genreRule{
	// Regional / niche first (these contain words like "pop"/"rock" themselves).
	{"k-pop", "K-Pop"}, {"j-pop", "J-Pop"}, {"j-rock", "J-Rock"}, {"j-rap", "J-Hip-Hop"},
	{"city pop", "City Pop"}, {"anime", "Anime"}, {"c-pop", "C-Pop"}, {"mandopop", "C-Pop"},
	{"afrobeat", "Afrobeats"}, {"amapiano", "Afrobeats"},
	{"phonk", "Phonk"}, {"vaporwave", "Vaporwave"}, {"synthwave", "Synthwave"},
	{"lo-fi", "Lo-Fi"}, {"lofi", "Lo-Fi"}, {"chillhop", "Lo-Fi"},
	// Hip-hop family.
	{"hip hop", "Hip-Hop"}, {"hip-hop", "Hip-Hop"}, {"rap", "Hip-Hop"},
	{"trap", "Hip-Hop"}, {"drill", "Hip-Hop"}, {"grime", "Hip-Hop"}, {"hxc", "Hip-Hop"},
	// R&B / soul.
	{"r&b", "R&B / Soul"}, {"rnb", "R&B / Soul"}, {"soul", "R&B / Soul"},
	{"funk", "R&B / Soul"}, {"motown", "R&B / Soul"}, {"neo soul", "R&B / Soul"},
	{"gospel", "R&B / Soul"},
	// Rock / alternative family (specific before generic "rock").
	{"metal", "Metal"}, {"metalcore", "Metal"}, {"djent", "Metal"},
	{"punk", "Punk"}, {"emo", "Emo"}, {"hardcore", "Hardcore"}, {"screamo", "Hardcore"},
	{"shoegaze", "Indie"}, {"bedroom pop", "Indie"}, {"indie", "Indie"},
	{"grunge", "Rock"}, {"new wave", "Rock"}, {"britpop", "Rock"},
	{"post-rock", "Rock"}, {"alt z", "Indie"}, {"alternative", "Rock"}, {"rock", "Rock"},
	// Acoustic / roots.
	{"country", "Country"}, {"americana", "Country"}, {"bluegrass", "Country"},
	{"folk", "Folk"}, {"singer-songwriter", "Folk"}, {"acoustic", "Folk"},
	{"jazz", "Jazz"}, {"bossa nova", "Jazz"}, {"blues", "Blues"},
	{"classical", "Classical"}, {"orchestra", "Classical"}, {"opera", "Classical"}, {"baroque", "Classical"},
	// Latin / Caribbean / world.
	{"reggaeton", "Latin"}, {"latin", "Latin"}, {"salsa", "Latin"},
	{"bachata", "Latin"}, {"corrido", "Latin"}, {"banda", "Latin"}, {"cumbia", "Latin"},
	{"reggae", "Reggae"}, {"ska", "Reggae"}, {"dancehall", "Reggae"},
	// Electronic (no bare "dance", so "dance pop" stays Pop).
	{"house", "Electronic"}, {"techno", "Electronic"}, {"edm", "Electronic"},
	{"electro", "Electronic"}, {"dubstep", "Electronic"}, {"drum and bass", "Electronic"},
	{"dnb", "Electronic"}, {"trance", "Electronic"}, {"electronica", "Electronic"},
	{"future bass", "Electronic"}, {"downtempo", "Electronic"}, {"hardstyle", "Electronic"},
	{"disco", "Disco"}, {"ambient", "Ambient"},
	{"soundtrack", "Soundtrack"}, {"score", "Soundtrack"}, {"video game", "Soundtrack"},
	// Generic pop catches the rest of the *-pop tags (hyperpop, electropop, …).
	{"hyperpop", "Pop"}, {"pop", "Pop"},
}

// otherBucket holds tracks whose artists have no genre data or no rule match,
// plus the contents of buckets too small to deserve their own playlist.
const otherBucket = "Other"

// bucketRank gives each bucket a priority index from genreRules order, used to
// break ties when two buckets get the same number of votes (earlier = higher).
var bucketRank = func() map[string]int {
	m := make(map[string]int)
	for _, r := range genreRules {
		if _, ok := m[r.bucket]; !ok {
			m[r.bucket] = len(m)
		}
	}
	return m
}()

// genreToBucket maps a single Spotify genre tag to a bucket, or ("", false).
func genreToBucket(tag string) (string, bool) {
	tag = strings.ToLower(tag)
	for _, r := range genreRules {
		if strings.Contains(tag, r.keyword) {
			return r.bucket, true
		}
	}
	return "", false
}

// GenreBucket picks the best bucket for a set of genre tags (an artist's, or the
// union across a track's artists) by majority vote across the tags — more robust
// than first-match, since e.g. ["pop rap","dance pop","pop"] votes Pop 2 / Hip-Hop
// 1 → Pop. Ties break by genreRules priority. "Other" when nothing matches.
func GenreBucket(genres []string) string {
	votes := make(map[string]int)
	for _, g := range genres {
		if b, ok := genreToBucket(g); ok {
			votes[b]++
		}
	}
	best, bestN, bestRank := otherBucket, 0, 1<<30
	for b, n := range votes {
		if n > bestN || (n == bestN && bucketRank[b] < bestRank) {
			best, bestN, bestRank = b, n, bucketRank[b]
		}
	}
	return best
}

// GenreGroup is one bucket of an organize plan: a bucket name and its tracks.
type GenreGroup struct {
	Name   string
	Tracks []spotify.Track
}

// BuildGenreGroups buckets tracks by genre and returns groups sorted largest-
// first with "Other" pinned last. A track's genre is voted from the union of all
// its artists' genre tags (genres maps artist id → tags), which catches far more
// than the primary artist alone. Buckets smaller than minBucket are merged into
// "Other" so the result is a handful of meaningful playlists, not a pile of
// one-song ones — every track still lands in a group. minBucket <= 1 disables the
// merge.
func BuildGenreGroups(tracks []spotify.Track, genres map[zspotify.ID][]string, minBucket int) []GenreGroup {
	byBucket := make(map[string][]spotify.Track)
	for _, t := range tracks {
		ids := t.ArtistIDs
		if len(ids) == 0 && t.ArtistID != "" {
			ids = []zspotify.ID{t.ArtistID}
		}
		var tags []string
		for _, aid := range ids {
			tags = append(tags, genres[aid]...)
		}
		b := GenreBucket(tags)
		byBucket[b] = append(byBucket[b], t)
	}
	if minBucket > 1 {
		for name, ts := range byBucket {
			if name != otherBucket && len(ts) < minBucket {
				byBucket[otherBucket] = append(byBucket[otherBucket], ts...)
				delete(byBucket, name)
			}
		}
	}
	groups := make([]GenreGroup, 0, len(byBucket))
	for name, ts := range byBucket {
		groups = append(groups, GenreGroup{Name: name, Tracks: ts})
	}
	sort.Slice(groups, func(i, j int) bool {
		// "Other" always sorts last; the rest by size (largest first), then name.
		if (groups[i].Name == otherBucket) != (groups[j].Name == otherBucket) {
			return groups[j].Name == otherBucket
		}
		if len(groups[i].Tracks) != len(groups[j].Tracks) {
			return len(groups[i].Tracks) > len(groups[j].Tracks)
		}
		return groups[i].Name < groups[j].Name
	})
	return groups
}
