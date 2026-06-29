package library

import (
	"sort"
	"strings"

	"audiopulse/internal/spotify"

	zspotify "github.com/zmb3/spotify/v2"
)

// genreRule maps a keyword found in an artist's Spotify genre tags to a coarse
// bucket. Rules are checked in order and the first whose keyword appears in any
// of the artist's genres wins, so more specific buckets must come before generic
// ones (e.g. "k-pop" before "pop", "indie" before "rock"). Spotify's genres are
// hyper-granular ("australian indie rock", "indietronica"), so substring matching
// against a curated, ordered list collapses them into a handful of useful piles.
type genreRule struct {
	keyword string
	bucket  string
}

var genreRules = []genreRule{
	// Regional / niche first (these contain words like "pop"/"rock" themselves).
	{"k-pop", "K-Pop"}, {"j-pop", "J-Pop"}, {"j-rock", "J-Rock"},
	{"city pop", "City Pop"}, {"anime", "Anime"},
	{"phonk", "Phonk"}, {"vaporwave", "Vaporwave"}, {"synthwave", "Synthwave"},
	{"lo-fi", "Lo-Fi"}, {"lofi", "Lo-Fi"},
	// Hip-hop family.
	{"hip hop", "Hip-Hop"}, {"hip-hop", "Hip-Hop"}, {"rap", "Hip-Hop"},
	{"trap", "Hip-Hop"}, {"drill", "Hip-Hop"}, {"grime", "Hip-Hop"},
	// R&B / soul.
	{"r&b", "R&B / Soul"}, {"rnb", "R&B / Soul"}, {"soul", "R&B / Soul"},
	{"funk", "R&B / Soul"}, {"motown", "R&B / Soul"},
	// Rock family (specific before generic "rock").
	{"metal", "Metal"}, {"punk", "Punk"}, {"emo", "Emo"}, {"hardcore", "Hardcore"},
	{"grunge", "Rock"}, {"indie", "Indie"}, {"rock", "Rock"},
	// Acoustic / roots.
	{"country", "Country"}, {"folk", "Folk"}, {"singer-songwriter", "Folk"},
	{"acoustic", "Folk"}, {"bluegrass", "Country"},
	{"jazz", "Jazz"}, {"blues", "Blues"},
	{"classical", "Classical"}, {"orchestra", "Classical"}, {"opera", "Classical"},
	// Latin / Caribbean.
	{"reggaeton", "Latin"}, {"latin", "Latin"}, {"salsa", "Latin"}, {"bachata", "Latin"},
	{"reggae", "Reggae"}, {"ska", "Reggae"}, {"dancehall", "Reggae"},
	// Electronic (no bare "dance", so "dance pop" stays Pop).
	{"house", "Electronic"}, {"techno", "Electronic"}, {"edm", "Electronic"},
	{"electro", "Electronic"}, {"dubstep", "Electronic"}, {"drum and bass", "Electronic"},
	{"dnb", "Electronic"}, {"trance", "Electronic"}, {"electronica", "Electronic"},
	{"ambient", "Ambient"}, {"soundtrack", "Soundtrack"}, {"score", "Soundtrack"},
	// Generic pop catches the rest of the *-pop tags.
	{"pop", "Pop"},
}

// otherBucket holds tracks whose artist has no genre data or no rule match, plus
// the contents of buckets too small to deserve their own playlist.
const otherBucket = "Other"

// GenreBucket maps an artist's genre tags to a single coarse bucket, or "Other"
// when there's no genre data or no rule matches.
func GenreBucket(genres []string) string {
	for _, rule := range genreRules {
		for _, g := range genres {
			if strings.Contains(strings.ToLower(g), rule.keyword) {
				return rule.bucket
			}
		}
	}
	return otherBucket
}

// GenreGroup is one bucket of an organize plan: a bucket name and its tracks.
type GenreGroup struct {
	Name   string
	Tracks []spotify.Track
}

// BuildGenreGroups buckets tracks by their primary artist's genre (genres maps
// artist id → tags) and returns groups sorted largest-first. Buckets smaller than
// minBucket are merged into "Other" so the result is a handful of meaningful
// playlists instead of a pile of one-song ones — every track still lands in a
// group. A minBucket <= 1 disables merging.
func BuildGenreGroups(tracks []spotify.Track, genres map[zspotify.ID][]string, minBucket int) []GenreGroup {
	byBucket := make(map[string][]spotify.Track)
	for _, t := range tracks {
		byBucket[GenreBucket(genres[t.ArtistID])] = append(byBucket[GenreBucket(genres[t.ArtistID])], t)
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
