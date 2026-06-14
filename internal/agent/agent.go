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
	ActionPlay      Action = "play"
	ActionPause     Action = "pause"
	ActionResume    Action = "resume"
	ActionNext      Action = "next"
	ActionPrevious  Action = "previous"
	ActionShuffle   Action = "shuffle"
	ActionRepeat    Action = "repeat"
	ActionVolume    Action = "volume"
	ActionRecommend Action = "recommend" // suggest tracks (RAG over the library)
	ActionAsk       Action = "ask"       // answer a question about the library
	ActionReindex   Action = "reindex"   // rebuild the library RAG index
	ActionUnknown   Action = "unknown"
)

// validActions is the allowlist an interpreted action is checked against; an
// action outside it collapses to ActionUnknown.
var validActions = map[Action]bool{
	ActionPlay: true, ActionPause: true, ActionResume: true,
	ActionNext: true, ActionPrevious: true, ActionShuffle: true,
	ActionRepeat: true, ActionVolume: true,
	ActionRecommend: true, ActionAsk: true, ActionReindex: true,
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

// defaultEmbedModel is the local embedding model used for the library RAG index.
const defaultEmbedModel = "nomic-embed-text"

// Client talks to a local Ollama instance.
type Client struct {
	baseURL    string
	http       *http.Client
	embedModel string

	mu    sync.Mutex
	model string // configured, or auto-detected and cached on first use
}

// New builds a Client. An empty baseURL defaults to localhost Ollama; an empty
// model is auto-detected (the first installed gemma* model) on first use; an
// empty embedModel defaults to nomic-embed-text.
func New(baseURL, model, embedModel string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if embedModel = strings.TrimSpace(embedModel); embedModel == "" {
		embedModel = defaultEmbedModel
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      strings.TrimSpace(model),
		embedModel: embedModel,
		http:       &http.Client{Timeout: 60 * time.Second},
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

// chat sends messages to /api/chat and returns the assistant's reply content.
// jsonMode forces a single JSON object (format:"json"); temp is the sampling
// temperature.
func (c *Client) chat(ctx context.Context, msgs []chatMessage, jsonMode bool, temp float64) (string, error) {
	model, err := c.resolveModel(ctx)
	if err != nil {
		return "", err
	}
	reqBody := chatRequest{
		Model:    model,
		Stream:   false,
		Options:  chatOptions{Temperature: temp},
		Messages: msgs,
	}
	if jsonMode {
		reqBody.Format = "json"
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("contacting Ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("ollama: /api/chat returned %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	var env struct {
		Message chatMessage `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return "", fmt.Errorf("decoding Ollama response: %w", err)
	}
	return env.Message.Content, nil
}

// Interpret asks the model to route an utterance into a Command (control,
// recommend, ask, or reindex).
func (c *Client) Interpret(ctx context.Context, utterance string) (Command, error) {
	utterance = strings.TrimSpace(utterance)
	if utterance == "" {
		return Command{Action: ActionUnknown}, nil
	}
	content, err := c.chat(ctx, append(promptMessages(), chatMessage{Role: "user", Content: utterance}), true, 0)
	if err != nil {
		return Command{}, err
	}
	return parseCommand(content)
}

// embedRequest is the Ollama /api/embed request body.
type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// Embed returns one vector per input string, using the configured embedding
// model (default nomic-embed-text). It implements library.Embedder.
func (c *Client) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embedRequest{Model: c.embedModel, Input: inputs})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contacting Ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("embedding model %q not found — run `ollama pull %s`", c.embedModel, c.embedModel)
		}
		return nil, fmt.Errorf("ollama: /api/embed returned %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	var env struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("decoding embeddings: %w", err)
	}
	if len(env.Embeddings) != len(inputs) {
		return nil, fmt.Errorf("embeddings count %d != inputs %d", len(env.Embeddings), len(inputs))
	}
	return env.Embeddings, nil
}

// Suggestion is one recommended track (resolved to a playable track via search).
type Suggestion struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// Recommend suggests up to n songs the user would enjoy. request is the user's
// phrasing ("something like daft punk", "focus music", or ""); taste is a sample
// of their library ("Title — Artist" lines) used as a grounding signal. Per the
// product decision, it favours *discovery* — songs not necessarily already owned.
func (c *Client) Recommend(ctx context.Context, request string, taste []string, n int) ([]Suggestion, error) {
	if n <= 0 {
		n = 12
	}
	var b strings.Builder
	fmt.Fprintf(&b, "You are a music recommender. Suggest exactly %d songs the user would enjoy.\n", n)
	b.WriteString("Favour discovery — real, well-known songs that fit, NOT necessarily ones already listed.\n")
	b.WriteString("Avoid duplicates and avoid recommending the exact tracks listed below.\n")
	if len(taste) > 0 {
		b.WriteString("\nThe user's library / taste (a sample):\n")
		for _, t := range taste {
			b.WriteString("- ")
			b.WriteString(t)
			b.WriteString("\n")
		}
	}
	if r := strings.TrimSpace(request); r != "" {
		fmt.Fprintf(&b, "\nThe user asked for: %s\n", r)
	}
	b.WriteString(`
Return ONLY a JSON object: {"suggestions":[{"title":"...","artist":"..."}, ...]}.`)

	content, err := c.chat(ctx, []chatMessage{
		{Role: "system", Content: "You recommend real songs as strict JSON. No commentary."},
		{Role: "user", Content: b.String()},
	}, true, 0.4)
	if err != nil {
		return nil, err
	}
	return parseSuggestions(content), nil
}

// parseSuggestions extracts the suggestion list from a model JSON reply. Small
// models are inconsistent here — they return {"suggestions":[…]}, a bare array,
// or rename the key/fields — so this tolerates all of those, plus stray prose.
func parseSuggestions(content string) []Suggestion {
	raw := firstJSONSpan(content)
	if raw == "" {
		return nil
	}
	var items []map[string]any
	if strings.HasPrefix(strings.TrimSpace(raw), "[") {
		_ = json.Unmarshal([]byte(raw), &items)
	} else {
		var obj map[string]json.RawMessage
		if json.Unmarshal([]byte(raw), &obj) == nil {
			for _, k := range []string{"suggestions", "songs", "recommendations", "tracks", "results", "list", "items"} {
				if v, ok := obj[k]; ok && json.Unmarshal(v, &items) == nil && len(items) > 0 {
					break
				}
			}
			if len(items) == 0 { // any array-valued field
				for _, v := range obj {
					var arr []map[string]any
					if json.Unmarshal(v, &arr) == nil && len(arr) > 0 {
						items = arr
						break
					}
				}
			}
		}
	}

	out := make([]Suggestion, 0, len(items))
	seen := make(map[string]bool)
	for _, it := range items {
		title := strings.TrimSpace(firstStr(it, "title", "name", "song", "track"))
		artist := strings.TrimSpace(firstStr(it, "artist", "artists", "by", "singer"))
		if title == "" {
			continue
		}
		key := strings.ToLower(title + "|" + artist)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Suggestion{Title: title, Artist: artist})
	}
	return out
}

// firstStr returns the first key in m whose value is a non-empty string.
func firstStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// firstJSONSpan returns the first balanced JSON object or array in s, whichever
// appears first. It is string-aware (braces/brackets inside song titles don't
// throw off the depth count).
func firstJSONSpan(s string) string {
	start, open, close := -1, byte('{'), byte('}')
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			start, open, close = i, '{', '}'
			break
		}
		if s[i] == '[' {
			start, open, close = i, '[', ']'
			break
		}
	}
	if start < 0 {
		return ""
	}
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
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
	// Actions that need a target are meaningless without one.
	if (c.Action == ActionPlay || c.Action == ActionAsk) && c.Query == "" {
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
	const system = `You convert a user's music request into ONE JSON object and nothing else.

Schema (always include every field):
{"action": one of "play","pause","resume","next","previous","shuffle","repeat","volume","recommend","ask","reindex","unknown",
 "query": string — for "play": the exact song/artist to play; for "recommend": the vibe/seed; for "ask": the question; otherwise "",
 "on": boolean — for "shuffle": true to enable, false to disable,
 "repeat": one of "off","all","one" — only for "repeat",
 "volume": integer 0-100 — only for "volume"}

Rules:
- "play X" / "put on X" / "I want to hear X" -> action "play", query "X" (a SPECIFIC song or artist).
- "play something like X" / "recommend X" / "suggest some Y" / "songs like Z" / "make me a playlist for studying" / "music for working out" -> action "recommend", query = the vibe or seed (NOT "play").
- A library QUESTION ("how many X songs do I have", "what playlists do I have", "do I have any Y", "which album…") -> action "ask", query = the question.
- "reindex" / "rebuild the library index" / "refresh my library" -> action "reindex".
- "skip" / "next" / "next song" -> "next". "go back" / "previous" / "last song" -> "previous".
- "shuffle on" / "turn on shuffle" -> "shuffle", on true. "shuffle off" / "stop shuffling" -> "shuffle", on false.
- "repeat" / "loop this song" -> "repeat", repeat "one". "repeat all" / "loop the playlist" -> "repeat", repeat "all". "stop repeating" / "turn off repeat" -> "repeat", repeat "off".
- "pause" / "stop" -> "pause". "resume" / "continue" / "unpause" -> "resume".
- "set volume to 40" / "volume up to 70" -> "volume", volume N.
- If the request is unclear, action "unknown".
Output only the JSON object.`

	examples := []struct{ user, json string }{
		{"play bohemian rhapsody by queen", `{"action":"play","query":"Bohemian Rhapsody Queen","on":false,"repeat":"off","volume":0}`},
		{"play something like daft punk", `{"action":"recommend","query":"like Daft Punk","on":false,"repeat":"off","volume":0}`},
		{"recommend some chill focus music", `{"action":"recommend","query":"chill focus music","on":false,"repeat":"off","volume":0}`},
		{"how many radiohead songs do i have", `{"action":"ask","query":"how many radiohead songs do i have","on":false,"repeat":"off","volume":0}`},
		{"rebuild the library index", `{"action":"reindex","query":"","on":false,"repeat":"off","volume":0}`},
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
