package lyrics

import (
	"testing"
	"time"
)

func TestParseLRC(t *testing.T) {
	in := "[00:12.50] Hello\n[00:15.00]World\n[notatag] skip\n[01:00.0] End"
	lines := parseLRC(in)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: %+v", len(lines), lines)
	}
	if got := lines[0]; got.At != 12500*time.Millisecond || got.Text != "Hello" {
		t.Errorf("line0 = %+v, want {12.5s Hello}", got)
	}
	if got := lines[1]; got.At != 15*time.Second || got.Text != "World" {
		t.Errorf("line1 = %+v, want {15s World}", got)
	}
	if got := lines[2]; got.At != time.Minute || got.Text != "End" {
		t.Errorf("line2 = %+v, want {60s End}", got)
	}
}

func TestParseLRCRepeatedTimestampsSorted(t *testing.T) {
	// One text line can carry several timestamps; they expand and sort by time.
	lines := parseLRC("[00:30.00][00:05.00] chorus")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0].At != 5*time.Second || lines[1].At != 30*time.Second {
		t.Errorf("timestamps not sorted: %+v", lines)
	}
}

func TestParsePlain(t *testing.T) {
	lines := parsePlain("a\r\nb")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	for i, ln := range lines {
		if ln.At != -1 {
			t.Errorf("plain line %d should be untimed, got %v", i, ln.At)
		}
	}
	if lines[0].Text != "a" || lines[1].Text != "b" {
		t.Errorf("unexpected text: %+v", lines)
	}
}

func TestParseEmpty(t *testing.T) {
	if got := parseLRC("   "); got != nil {
		t.Errorf("blank LRC should parse to nil, got %+v", got)
	}
	if got := parsePlain(""); got != nil {
		t.Errorf("blank plain should parse to nil, got %+v", got)
	}
}
