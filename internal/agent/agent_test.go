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
