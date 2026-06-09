package downloader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestFailureName(t *testing.T) {
	got := failureName("LookupError: No results found for song: Hp Boyz - SATIVA - Spotify Singles")
	if got != "Hp Boyz - SATIVA - Spotify Singles" {
		t.Errorf("failureName = %q, want the song name", got)
	}
	if got := failureName("Error: weird"); got != "Error: weird" {
		t.Errorf("fallback failureName = %q", got)
	}
}

func TestWriteFailures(t *testing.T) {
	dir := t.TempDir()
	writeFailures(dir, []string{"A - B", "C - D"})
	b, err := os.ReadFile(filepath.Join(dir, reportName))
	if err != nil {
		t.Fatalf("report not written: %v", err)
	}
	if s := string(b); !strings.Contains(s, "A - B") || !strings.Contains(s, "C - D") {
		t.Errorf("report missing entries:\n%s", s)
	}
	// No failures → the report is removed.
	writeFailures(dir, nil)
	if _, err := os.Stat(filepath.Join(dir, reportName)); !os.IsNotExist(err) {
		t.Error("empty failures should remove a stale report")
	}
}
