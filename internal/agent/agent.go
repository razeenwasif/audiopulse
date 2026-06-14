// Package agent turns natural-language requests ("play bohemian rhapsody",
// "turn shuffle on", "skip this") into structured AudioPulse playback commands
// using a local Ollama model such as Gemma.
//
// Nothing leaves the machine: the utterance is sent to Ollama's HTTP API on
// localhost, which returns a small JSON object describing the intended action.
// We use Ollama's JSON mode (format:"json") plus a strict, few-shot system
// prompt rather than a fine-tuned model or OpenAI-style tool tokens — a small
// local model follows a constrained JSON schema reliably this way, and it works
// across Gemma variants without special tool-calling support.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the local Ollama HTTP endpoint.
const DefaultBaseURL = "http://localhost:11434"

// Action is a recognised playback intent.
type Action string

const (
	ActionPlay     Action = "play"
	ActionPause    Action = "pause"
	ActionResume   Action = "resume"
	ActionNext     Action = "next"
	ActionPrevious Action = "previous"
	ActionShuffle  Action = "shuffle"
	ActionRepeat   Action = "repeat"
	ActionVolume   Action = "volume"
	ActionUnknown  Action = "unknown"
)

// validActions is the allowlist an interpreted action is checked against; an
// action outside it collapses to ActionUnknown.
var validActions = map[Action]bool{
	ActionPlay: true, ActionPause: true, ActionResume: true,
	ActionNext: true, ActionPrevious: true, ActionShuffle: true,
	ActionRepeat: true, ActionVolume: true,
}

// Command is the structured result of interpreting an utterance. Only the
// fields relevant to Action are meaningful.
type Command struct {
	Action Action `json:"action"`
	Query  string `json:"query"`  // song/artist to play (ActionPlay)
	On     bool   `json:"on"`     // shuffle target (ActionShuffle)
	Repeat string `json:"repeat"` // "off" | "all" | "one" (ActionRepeat)
	Volume int    `json:"volume"` // 0-100 (ActionVolume)
}

// Client talks to a local Ollama instance.
type Client struct {
	baseURL string
	http    *http.Client

	mu    sync.Mutex
	model string // configured, or auto-detected and cached on first use
}

// New builds a Client. An empty baseURL defaults to localhost Ollama; an empty
// model is auto-detected (the first installed gemma* model) on first use.
func New(baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   strings.TrimSpace(model),
		http:    &http.Client{Timeout: 35 * time.Second},
	}
}

// Available reports whether the Ollama server is reachable.
func (c *Client) Available(ctx context.Context) bool {
	_, err := c.listModels(ctx)
	return err == nil
}

// listModels returns the names of the locally installed Ollama models.
func (c *Client) listModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: /api/tags returned %s", resp.Status)
	}
	var env struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(env.Models))
	for _, m := range env.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

// pickModel chooses which model to use: the configured one if set, else the
// first installed model whose name mentions gemma, else the first model at all.
func pickModel(available []string, configured string) string {
	if configured != "" {
		return configured
	}
	for _, m := range available {
		if strings.Contains(strings.ToLower(m), "gemma") {
			return m
		}
	}
	if len(available) > 0 {
		return available[0]
	}
	return ""
}

// resolveModel returns the model to query, auto-detecting and caching it when
// none was configured.
func (c *Client) resolveModel(ctx context.Context) (string, error) {
	c.mu.Lock()
	model := c.model
	c.mu.Unlock()
	if model != "" {
		return model, nil
	}
	available, err := c.listModels(ctx)
	if err != nil {
		return "", fmt.Errorf("Ollama unreachable — is it running? (%w)", err)
	}
	model = pickModel(available, "")
	if model == "" {
		return "", errors.New("no Ollama models installed — run `ollama pull gemma3`")
	}
	c.mu.Lock()
	c.model = model
	c.mu.Unlock()
	return model, nil
}

// chatRequest is the Ollama /api/chat request body.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Format   string        `json:"format"`
	Options  chatOptions   `json:"options"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatOptions struct {
	Temperature float64 `json:"temperature"`
}

// Interpret asks the model to translate an utterance into a Command.
func (c *Client) Interpret(ctx context.Context, utterance string) (Command, error) {
	utterance = strings.TrimSpace(utterance)
	if utterance == "" {
		return Command{Action: ActionUnknown}, nil
	}
	model, err := c.resolveModel(ctx)
	if err != nil {
		return Command{}, err
	}

	body, err := json.Marshal(chatRequest{
		Model:    model,
		Stream:   false,
		Format:   "json",
		Options:  chatOptions{Temperature: 0},
		Messages: append(promptMessages(), chatMessage{Role: "user", Content: utterance}),
	})
	if err != nil {
		return Command{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Command{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return Command{}, fmt.Errorf("contacting Ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Command{}, fmt.Errorf("ollama: /api/chat returned %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	var env struct {
		Message chatMessage `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return Command{}, fmt.Errorf("decoding Ollama response: %w", err)
	}
	return parseCommand(env.Message.Content)
}

// parseCommand decodes a model's JSON content into a normalized Command. It
// tolerates a model that wraps the object in stray prose by extracting the
// first {...} span.
func parseCommand(content string) (Command, error) {
	raw := extractJSON(content)
	if raw == "" {
		return Command{Action: ActionUnknown}, nil
	}
	var cmd Command
	if err := json.Unmarshal([]byte(raw), &cmd); err != nil {
		return Command{Action: ActionUnknown}, nil
	}
	return normalize(cmd), nil
}

// normalize lower-cases and validates the action, trims the query, canonicalises
// the repeat mode, and clamps the volume.
func normalize(c Command) Command {
	c.Action = Action(strings.ToLower(strings.TrimSpace(string(c.Action))))
	if !validActions[c.Action] {
		c.Action = ActionUnknown
	}
	c.Query = strings.TrimSpace(c.Query)
	switch strings.ToLower(strings.TrimSpace(c.Repeat)) {
	case "all", "context", "playlist":
		c.Repeat = "all"
	case "one", "track", "song", "this":
		c.Repeat = "one"
	default:
		c.Repeat = "off"
	}
	if c.Volume < 0 {
		c.Volume = 0
	}
	if c.Volume > 100 {
		c.Volume = 100
	}
	// A play action with no query is meaningless.
	if c.Action == ActionPlay && c.Query == "" {
		c.Action = ActionUnknown
	}
	return c
}

// extractJSON returns the first balanced {...} object found in s, or "" if none.
// JSON mode normally returns a bare object, but this guards against a model that
// adds surrounding text anyway.
func extractJSON(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// promptMessages is the system prompt plus a few worked examples that anchor the
// output format for small local models.
func promptMessages() []chatMessage {
	const system = `You convert a user's music command into ONE JSON object and nothing else.

Schema (always include every field):
{"action": one of "play","pause","resume","next","previous","shuffle","repeat","volume","unknown",
 "query": string — the song/artist to play, only for "play", otherwise "",
 "on": boolean — for "shuffle": true to enable, false to disable,
 "repeat": one of "off","all","one" — only for "repeat",
 "volume": integer 0-100 — only for "volume"}

Rules:
- "play X" / "put on X" / "I want to hear X" -> action "play", query "X".
- "skip" / "next" / "next song" -> "next". "go back" / "previous" / "last song" -> "previous".
- "shuffle on" / "turn on shuffle" -> "shuffle", on true. "shuffle off" / "stop shuffling" -> "shuffle", on false.
- "repeat" / "loop this song" -> "repeat", repeat "one". "repeat all" / "loop the playlist" -> "repeat", repeat "all". "stop repeating" / "turn off repeat" -> "repeat", repeat "off".
- "pause" / "stop" -> "pause". "resume" / "continue" / "unpause" -> "resume".
- "set volume to 40" / "volume up to 70" -> "volume", volume N.
- If the request is unclear, action "unknown".
Output only the JSON object.`

	examples := []struct{ user, json string }{
		{"play bohemian rhapsody by queen", `{"action":"play","query":"Bohemian Rhapsody Queen","on":false,"repeat":"off","volume":0}`},
		{"skip this one", `{"action":"next","query":"","on":false,"repeat":"off","volume":0}`},
		{"turn shuffle on", `{"action":"shuffle","query":"","on":true,"repeat":"off","volume":0}`},
		{"loop this song", `{"action":"repeat","query":"","on":false,"repeat":"one","volume":0}`},
		{"set the volume to 30", `{"action":"volume","query":"","on":false,"repeat":"off","volume":30}`},
	}
	msgs := []chatMessage{{Role: "system", Content: system}}
	for _, e := range examples {
		msgs = append(msgs,
			chatMessage{Role: "user", Content: e.user},
			chatMessage{Role: "assistant", Content: e.json},
		)
	}
	return msgs
}
