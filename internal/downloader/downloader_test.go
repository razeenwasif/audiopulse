package downloader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct{ line, kind, name string }{
		{`Downloaded "Daft Punk - Instant Crush": https://x`, "done", "Daft Punk - Instant Crush"},
		{`Skipping Daft Punk - Get Lucky (file already exists)`, "skip", "Daft Punk - Get Lucky"},
		{`LookupError: No results found for song: Obscure Track`, "fail", "Obscure Track"},
		{`Processing query: spotify:track:xyz`, "", ""},
	}
	for _, c := range cases {
		if k, n := classify(c.line); k != c.kind || n != c.name {
			t.Errorf("classify(%q) = (%q,%q), want (%q,%q)", c.line, k, n, c.kind, c.name)
		}
	}
}

// TestTallyDedup reproduces the real spotdl output that inflated the counts:
// retries print the same failure repeatedly, and duplicate URIs print the same
// song repeatedly. The tally must count each distinct song once.
func TestTallyDedup(t *testing.T) {
	lines := []string{
		`LookupError: No results found for song: Hp Boyz - SATIVA - Spotify Singles`,
		`LookupError: No results found for song: Hp Boyz - SATIVA - Spotify Singles`, // retry
		`Downloaded "sombr - back to friends": https://x`,
		`Downloaded "Noah Kahan - Doors": https://y`,
		`Downloaded "sombr - back to friends": https://x`,        // duplicate URI
		`Skipping sombr - back to friends (file already exists)`, // duplicate URI
		`LookupError: No results found for song: LE SSERAFIM - CRAZY`,
	}
	ta := newTally()
	for _, l := range lines {
		ta.add(l)
	}
	if ta.done() != 2 {
		t.Errorf("done = %d, want 2 (sombr, Noah — sombr counted once)", ta.done())
	}
	if ta.failed() != 2 {
		t.Errorf("failed = %d, want 2 (Hp Boyz, LE SSERAFIM — retries deduped)", ta.failed())
	}
	if ta.skipped() != 0 {
		t.Errorf("skipped = %d, want 0 (sombr already counted downloaded)", ta.skipped())
	}
	if f := ta.failures(); len(f) != 2 {
		t.Errorf("failures = %v, want 2 distinct names", f)
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
