package library

import (
	"testing"

	"audiopulse/internal/spotify"

	zspotify "github.com/zmb3/spotify/v2"
)

func TestGenreBucket(t *testing.T) {
	cases := []struct {
		genres []string
		want   string
	}{
		{[]string{"australian indie rock"}, "Indie"}, // indie before rock
		{[]string{"classic rock", "album rock"}, "Rock"},
		{[]string{"k-pop", "pop"}, "K-Pop"},       // niche before generic pop
		{[]string{"dance pop", "pop"}, "Pop"},     // no bare "dance" → stays Pop
		{[]string{"electro house"}, "Electronic"}, // electro matches
		{[]string{"conscious hip hop", "rap"}, "Hip-Hop"},
		{[]string{"neo soul"}, "R&B / Soul"},
		{[]string{"melodic phonk"}, "Phonk"},
		{[]string{"nu jazz"}, "Jazz"},
		{nil, "Other"},
		{[]string{"polka"}, "Other"}, // no rule
	}
	for _, c := range cases {
		if got := GenreBucket(c.genres); got != c.want {
			t.Errorf("GenreBucket(%v) = %q, want %q", c.genres, got, c.want)
		}
	}
}

func TestBuildGenreGroups(t *testing.T) {
	tr := func(id, artistID string) spotify.Track {
		return spotify.Track{ID: zspotify.ID(id), ArtistID: zspotify.ID(artistID)}
	}
	tracks := []spotify.Track{
		tr("1", "rockA"), tr("2", "rockA"), tr("3", "rockB"), // 3 rock
		tr("4", "popA"), tr("5", "popA"), // 2 pop
		tr("6", "jazzA"),    // 1 jazz (below minBucket → Other)
		tr("7", "unknownX"), // no genres → Other
	}
	genres := map[zspotify.ID][]string{
		"rockA": {"classic rock"}, "rockB": {"hard rock"},
		"popA": {"dance pop"}, "jazzA": {"smooth jazz"},
		// unknownX intentionally absent
	}
	groups := BuildGenreGroups(tracks, genres, 3)

	if len(groups) != 2 {
		t.Fatalf("want 2 groups (Rock, Other), got %d: %+v", len(groups), groups)
	}
	if groups[0].Name != "Rock" || len(groups[0].Tracks) != 3 {
		t.Errorf("largest group should be Rock×3, got %s×%d", groups[0].Name, len(groups[0].Tracks))
	}
	// Pop(2) and Jazz(1) are below minBucket=3 → merged into Other with unknownX.
	if groups[1].Name != "Other" || len(groups[1].Tracks) != 4 {
		t.Errorf("second group should be Other×4 (pop2+jazz1+unknown1), got %s×%d", groups[1].Name, len(groups[1].Tracks))
	}

	// minBucket<=1 keeps every bucket separate.
	if g := BuildGenreGroups(tracks, genres, 1); len(g) != 4 {
		t.Errorf("minBucket=1 should keep Rock/Pop/Jazz/Other = 4 groups, got %d", len(g))
	}
}
