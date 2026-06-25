package agent

import "testing"

func TestParseCommand(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Command
	}{
		{
			"play with query",
			`{"action":"play","query":" Bohemian Rhapsody ","on":false,"repeat":"off","volume":0}`,
			Command{Action: ActionPlay, Query: "Bohemian Rhapsody", Repeat: "off"},
		},
		{
			"shuffle on",
			`{"action":"shuffle","on":true}`,
			Command{Action: ActionShuffle, On: true, Repeat: "off"},
		},
		{
			"repeat one synonym (track)",
			`{"action":"repeat","repeat":"track"}`,
			Command{Action: ActionRepeat, Repeat: "one"},
		},
		{
			"repeat all synonym (playlist)",
			`{"action":"repeat","repeat":"playlist"}`,
			Command{Action: ActionRepeat, Repeat: "all"},
		},
		{
			"volume clamped",
			`{"action":"volume","volume":250}`,
			Command{Action: ActionVolume, Volume: 100, Repeat: "off"},
		},
		{
			"next, uppercased action",
			`{"action":"NEXT"}`,
			Command{Action: ActionNext, Repeat: "off"},
		},
		{
			"play without query collapses to unknown",
			`{"action":"play","query":""}`,
			Command{Action: ActionUnknown, Repeat: "off"},
		},
		{
			"unrecognised action collapses to unknown",
			`{"action":"teleport"}`,
			Command{Action: ActionUnknown, Repeat: "off"},
		},
		{
			"object wrapped in prose is still extracted",
			"Sure! Here you go: {\"action\":\"pause\"} hope that helps",
			Command{Action: ActionPause, Repeat: "off"},
		},
		{
			"garbage yields unknown, not an error",
			`not json at all`,
			Command{Action: ActionUnknown},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseCommand(c.content)
			if err != nil {
				t.Fatalf("parseCommand(%q) error: %v", c.content, err)
			}
			if got != c.want {
				t.Errorf("parseCommand(%q) = %+v, want %+v", c.content, got, c.want)
			}
		})
	}
}

func TestParseCommandNewActions(t *testing.T) {
	cases := []struct {
		content string
		want    Command
	}{
		{`{"action":"recommend","query":"like daft punk"}`, Command{Action: ActionRecommend, Query: "like daft punk", Repeat: "off"}},
		{`{"action":"recommend","query":""}`, Command{Action: ActionRecommend, Repeat: "off"}}, // empty seed is ok
		{`{"action":"ask","query":"how many radiohead songs"}`, Command{Action: ActionAsk, Query: "how many radiohead songs", Repeat: "off"}},
		{`{"action":"ask","query":""}`, Command{Action: ActionUnknown, Repeat: "off"}}, // empty question → unknown
		{`{"action":"reindex"}`, Command{Action: ActionReindex, Repeat: "off"}},
		{`{"action":"smart_shuffle","query":""}`, Command{Action: ActionShuffleAI, Repeat: "off"}},
		{`{"action":"create_playlist","query":"90s classics"}`, Command{Action: ActionCreatePL, Query: "90s classics", Repeat: "off"}},
		{`{"action":"create_playlist","query":""}`, Command{Action: ActionUnknown, Repeat: "off"}}, // no theme → unknown
	}
	for _, c := range cases {
		got, err := parseCommand(c.content)
		if err != nil {
			t.Fatalf("parseCommand(%q): %v", c.content, err)
		}
		if got != c.want {
			t.Errorf("parseCommand(%q) = %+v, want %+v", c.content, got, c.want)
		}
	}
}

func TestParseSuggestions(t *testing.T) {
	got := parseSuggestions(`{"suggestions":[
		{"title":" Get Lucky ","artist":"Daft Punk"},
		{"title":"Instant Crush","artist":"Daft Punk"},
		{"title":"Get Lucky","artist":"Daft Punk"},
		{"title":"","artist":"Nobody"}
	]}`)
	if len(got) != 2 {
		t.Fatalf("want 2 (deduped, blank dropped), got %d: %+v", len(got), got)
	}
	if got[0].Title != "Get Lucky" || got[0].Artist != "Daft Punk" {
		t.Errorf("first suggestion = %+v", got[0])
	}
	if parseSuggestions("not json") != nil {
		t.Error("garbage should yield nil suggestions")
	}

	// Robustness: a bare array (no wrapper key).
	if got := parseSuggestions(`[{"title":"A","artist":"x"},{"title":"B","artist":"y"}]`); len(got) != 2 {
		t.Errorf("bare array should parse, got %d", len(got))
	}
	// A renamed wrapper key + a brace inside a title.
	if got := parseSuggestions(`{"songs":[{"name":"Blue Monday {live}","by":"New Order"}]}`); len(got) != 1 || got[0].Title != "Blue Monday {live}" || got[0].Artist != "New Order" {
		t.Errorf("renamed keys / brace-in-title should parse, got %+v", got)
	}
	// Prose around the JSON.
	if got := parseSuggestions("Sure: {\"suggestions\":[{\"title\":\"T\",\"artist\":\"A\"}]} done"); len(got) != 1 {
		t.Errorf("prose-wrapped object should parse, got %d", len(got))
	}
}

func TestParsePlaylist(t *testing.T) {
	name, sugs := parsePlaylist(`{"name":"  90s Gold  ","suggestions":[{"title":"Smells Like Teen Spirit","artist":"Nirvana"},{"title":"Wonderwall","artist":"Oasis"}]}`)
	if name != "90s Gold" {
		t.Errorf("name = %q, want %q", name, "90s Gold")
	}
	if len(sugs) != 2 || sugs[0].Title != "Smells Like Teen Spirit" {
		t.Errorf("suggestions = %+v", sugs)
	}
	// A renamed name key + still extracts the list.
	if n, s := parsePlaylist(`{"playlist_name":"Mix","suggestions":[{"title":"A","artist":"B"}]}`); n != "Mix" || len(s) != 1 {
		t.Errorf("renamed name key: name=%q sugs=%d", n, len(s))
	}
	// No name → blank (caller supplies a fallback), list still parses.
	if n, s := parsePlaylist(`{"suggestions":[{"title":"A","artist":"B"}]}`); n != "" || len(s) != 1 {
		t.Errorf("missing name should be blank: name=%q sugs=%d", n, len(s))
	}
}

func TestPickModel(t *testing.T) {
	cases := []struct {
		name       string
		available  []string
		configured string
		want       string
	}{
		{"configured wins", []string{"llama3", "gemma3:4b"}, "mistral", "mistral"},
		{"prefers gemma", []string{"llama3:latest", "gemma3:12b", "qwen2"}, "", "gemma3:12b"},
		{"falls back to first", []string{"llama3:latest", "qwen2"}, "", "llama3:latest"},
		{"skips embedding gemma", []string{"embeddinggemma:latest", "gemma3:latest"}, "", "gemma3:latest"},
		{"skips embedders in fallback", []string{"nomic-embed-text:latest", "llama3:latest"}, "", "llama3:latest"},
		{"none available", nil, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pickModel(c.available, c.configured); got != c.want {
				t.Errorf("pickModel(%v, %q) = %q, want %q", c.available, c.configured, got, c.want)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"a":1}`, `{"a":1}`},
		{`prefix {"a":{"b":2}} suffix`, `{"a":{"b":2}}`},
		{`no object here`, ``},
		{`{unbalanced`, ``},
	}
	for _, c := range cases {
		if got := extractJSON(c.in); got != c.want {
			t.Errorf("extractJSON(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
