package library

import (
	"path/filepath"
	"testing"
)

func rec(id, title, artist string, vec ...float32) Record {
	return Record{ID: id, URI: "spotify:track:" + id, Title: title, Artist: artist, Vec: vec}
}

func TestSearchTopKByCosine(t *testing.T) {
	ix := &Index{Records: []Record{
		rec("a", "A", "x", 1, 0, 0),
		rec("b", "B", "y", 0, 1, 0),
		rec("c", "C", "z", 0.9, 0.1, 0),
	}}
	// Normalize stored vectors as Build would.
	for i := range ix.Records {
		normalize(ix.Records[i].Vec)
	}
	got := ix.Search([]float32{1, 0, 0}, 2)
	if len(got) != 2 {
		t.Fatalf("want 2 results, got %d", len(got))
	}
	if got[0].Record.ID != "a" || got[1].Record.ID != "c" {
		t.Errorf("ranking = %s,%s; want a,c", got[0].Record.ID, got[1].Record.ID)
	}
	if got[0].Score < got[1].Score {
		t.Error("scores should be descending")
	}
}

func TestNormalizeUnitLength(t *testing.T) {
	v := []float32{3, 4}
	normalize(v)
	if d := dot(v, v); d < 0.999 || d > 1.001 {
		t.Errorf("normalized vector dot-self = %f, want ~1", d)
	}
	// Zero vector must not divide by zero.
	z := []float32{0, 0}
	normalize(z)
	if z[0] != 0 || z[1] != 0 {
		t.Error("zero vector should stay zero")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idx.gob")
	ix := &Index{
		Records:   []Record{rec("a", "Song", "Artist", 0.6, 0.8)},
		Signature: "sig123",
		Dim:       2,
	}
	if err := ix.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Signature != "sig123" || len(got.Records) != 1 || got.Records[0].Title != "Song" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	// Missing file → (nil, nil), not an error.
	missing, err := Load(filepath.Join(t.TempDir(), "nope.gob"))
	if err != nil || missing != nil {
		t.Errorf("missing index should be (nil,nil), got (%v,%v)", missing, err)
	}
}

func TestStaleAndSignature(t *testing.T) {
	s1 := signature([]string{"p1:10", "p2:5"}, 15)
	s2 := signature([]string{"p2:5", "p1:10"}, 15) // order-independent
	if s1 != s2 {
		t.Error("signature should be order-independent")
	}
	if signature([]string{"p1:10"}, 15) == s1 {
		t.Error("different playlists should change the signature")
	}
	ix := &Index{Signature: s1}
	if ix.Stale(s1) {
		t.Error("matching signature should not be stale")
	}
	if !ix.Stale("other") {
		t.Error("changed signature should be stale")
	}
	var nilIx *Index
	if !nilIx.Stale(s1) {
		t.Error("nil index is always stale")
	}
}

func TestFilterAndSample(t *testing.T) {
	ix := &Index{Records: []Record{
		rec("a", "Karma Police", "Radiohead"),
		rec("b", "Get Lucky", "Daft Punk"),
		rec("c", "Creep", "Radiohead"),
	}}
	if n := len(ix.Filter("radiohead")); n != 2 {
		t.Errorf("Filter(radiohead) = %d, want 2", n)
	}
	if n := len(ix.Filter("")); n != 0 {
		t.Errorf("empty filter should match nothing, got %d", n)
	}
	if got := ix.Sample(2); len(got) != 2 {
		t.Errorf("Sample(2) = %d records, want 2", len(got))
	}
	if got := ix.Sample(10); len(got) != 3 {
		t.Errorf("Sample(>len) should return all 3, got %d", len(got))
	}
}

func TestLabelAndEmbedText(t *testing.T) {
	r := Record{Title: "Doors", Artist: "Noah Kahan", Album: "Stick Season"}
	if r.Label() != "Doors — Noah Kahan" {
		t.Errorf("Label = %q", r.Label())
	}
	if et := embedText(r); et != "Doors — Noah Kahan (album: Stick Season)" {
		t.Errorf("embedText = %q", et)
	}
}
