package downloader

import "testing"

func TestApplyLineCounts(t *testing.T) {
	var p Progress
	cases := []struct {
		line             string
		done, skip, fail int
		emit             bool
	}{
		{`Downloaded "Daft Punk - Instant Crush": https://music.youtube.com/watch?v=abc`, 1, 0, 0, true},
		{`Skipping Daft Punk - Get Lucky (file already exists) (duplicate)`, 1, 1, 0, true},
		{`LookupError: No results found for song: Obscure Track`, 1, 1, 1, true},
		{`Error: something went wrong`, 1, 1, 2, true},
		{`Processing query: spotify:track:xyz`, 1, 1, 2, false}, // info line, no change
	}
	for i, c := range cases {
		if got := applyLine(&p, c.line); got != c.emit {
			t.Errorf("case %d emit = %v, want %v", i, got, c.emit)
		}
		if p.Done != c.done || p.Skipped != c.skip || p.Failed != c.fail {
			t.Errorf("case %d: done=%d skip=%d fail=%d, want %d/%d/%d",
				i, p.Done, p.Skipped, p.Failed, c.done, c.skip, c.fail)
		}
	}
	if p.Processed() != 4 {
		t.Errorf("processed = %d, want 4", p.Processed())
	}
}

func TestApplyLineCurrent(t *testing.T) {
	var p Progress
	applyLine(&p, `Downloaded "M83 - Midnight City": https://x`)
	if p.Current != "M83 - Midnight City" {
		t.Errorf("current = %q, want the downloaded title", p.Current)
	}
}
