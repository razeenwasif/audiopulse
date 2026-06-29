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
		{[]string{"k-pop", "k-pop boy group", "pop"}, "K-Pop"}, // 2 k-pop vs 1 pop
		{[]string{"dance pop", "pop"}, "Pop"},                  // no bare "dance" → stays Pop
		{[]string{"pop rap", "dance pop", "pop"}, "Pop"},       // vote: Pop 2 / Hip-Hop 1
		{[]string{"electro house"}, "Electronic"},              // electro matches
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

func TestBucketNamesAndGroupByBuckets(t *testing.T) {
	names := BucketNames()
	if len(names) < 10 {
		t.Fatalf("expected many bucket names, got %d", len(names))
	}
	for _, n := range names {
		if n == OtherBucket {
			t.Error("BucketNames must not include Other")
		}
	}

	// GroupByBuckets honours an explicit per-track bucket slice (the LLM override
	// join point): "" falls to Other, sub-minBucket buckets merge into Other.
	tracks := []spotify.Track{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}}
	buckets := []string{"Pop", "Pop", "Pop", ""} // 3 Pop + 1 untagged
	g := GroupByBuckets(tracks, buckets, 2)
	if len(g) != 2 || g[0].Name != "Pop" || len(g[0].Tracks) != 3 || g[1].Name != "Other" {
		t.Errorf("GroupByBuckets = %+v", g)
	}
}

func TestBuildGenreGroupsUnionsAllArtists(t *testing.T) {
	// A track with two artists: a featured rapper (no genre) and a pop singer.
	// The union over both artists' genres should still classify it as Pop.
	track := spotify.Track{
		ID:        "x",
		ArtistIDs: []zspotify.ID{"feat", "popstar"},
	}
	genres := map[zspotify.ID][]string{
		"popstar": {"dance pop", "pop"},
		// "feat" has no genres
	}
	g := BuildGenreGroups([]spotify.Track{track}, genres, 1)
	if len(g) != 1 || g[0].Name != "Pop" {
		t.Errorf("union over all artists should bucket as Pop, got %+v", g)
	}
}
