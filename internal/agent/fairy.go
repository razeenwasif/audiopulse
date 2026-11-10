package agent

// Fairy is the user's desktop assistant. When she's running on this Mac, the
// AI assistant can use her instead of a local Ollama model: she runs each
// request once on her conversation provider (Claude, Gemini or ChatGPT) as a
// tool call — no persona, no tools, nothing saved as a chat — through a private
// socket in her data folder. See Lyra's desktop/audiopulse-link.mjs.
//
// Embeddings (the library RAG index) still use Ollama.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// Assistant modes for Client.WithAssistant / config "assistant".
const (
	AssistantAuto   = "auto"   // Fairy if she's running, otherwise Ollama
	AssistantFairy  = "fairy"  // only Fairy
	AssistantOllama = "ollama" // only Ollama
)

const fairyTimeout = 100 * time.Second

// errFairyUnavailable means Fairy isn't listening (not running, or too old).
var errFairyUnavailable = errors.New("Fairy isn't running on this Mac")

var fairyRequestID atomic.Int64

// FairySocketPath is where Fairy listens for AudioPulse.
func FairySocketPath() string {
	if dir := os.Getenv("LYRA_DATA_DIR"); dir != "" {
		return filepath.Join(dir, "audiopulse.sock")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "lyra-companion", "lyra", "audiopulse.sock")
}

// FairyAvailable reports whether Fairy's AudioPulse socket exists.
func FairyAvailable() bool {
	info, err := os.Stat(FairySocketPath())
	return err == nil && info.Mode()&os.ModeSocket != 0
}

// flattenForFairy turns an Ollama-style conversation into Fairy's two fields:
// the system messages become the instructions, and few-shot examples plus the
// final request become the prompt.
func flattenForFairy(msgs []chatMessage, jsonMode bool) (instructions, prompt string) {
	var sys []string
	var rest []chatMessage
	for _, m := range msgs {
		if m.Role == "system" {
			sys = append(sys, m.Content)
		} else {
			rest = append(rest, m)
		}
	}
	instructions = strings.Join(sys, "\n\n")
	if jsonMode {
		instructions += "\n\nReply with exactly one JSON object and nothing else."
	}
	if len(rest) == 0 {
		return instructions, ""
	}
	last := rest[len(rest)-1]
	var b strings.Builder
	if examples := rest[:len(rest)-1]; len(examples) > 0 {
		b.WriteString("Examples of requests and the right reply:\n")
		for _, m := range examples {
			if m.Role == "assistant" {
				fmt.Fprintf(&b, "Reply: %s\n\n", m.Content)
			} else {
				fmt.Fprintf(&b, "Request: %s\n", m.Content)
			}
		}
		b.WriteString("Now the real request.\n\n")
	}
	b.WriteString(last.Content)
	return instructions, b.String()
}

// askFairy sends one request over Fairy's socket and returns her text.
func askFairy(ctx context.Context, instructions, prompt string) (string, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", FairySocketPath())
	if err != nil {
		return "", errFairyUnavailable
	}
	defer conn.Close()
	deadline := time.Now().Add(fairyTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)

	id := fmt.Sprintf("tui-%d", fairyRequestID.Add(1))
	line, err := json.Marshal(map[string]string{"type": "ask", "id": id, "instructions": instructions, "prompt": prompt})
	if err != nil {
		return "", err
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return "", errFairyUnavailable
	}
	reader := bufio.NewReaderSize(conn, 64*1024)
	for {
		raw, err := reader.ReadBytes('\n')
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return "", errors.New("Fairy took too long to answer")
			}
			return "", fmt.Errorf("reading Fairy's answer: %w", err)
		}
		var reply struct {
			Type    string `json:"type"`
			ID      string `json:"id"`
			Text    string `json:"text"`
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &reply) != nil || (reply.ID != "" && reply.ID != id) {
			continue
		}
		switch reply.Type {
		case "done":
			return reply.Text, nil
		case "error":
			if reply.Message == "Unknown request." {
				return "", errFairyUnavailable
			}
			return "", fmt.Errorf("Fairy: %s", reply.Message)
		}
	}
}

// WithAssistant chooses where chat requests run: AssistantAuto (default),
// AssistantFairy or AssistantOllama. Unknown values mean auto.
func (c *Client) WithAssistant(mode string) *Client {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case AssistantFairy:
		c.assistant = AssistantFairy
	case AssistantOllama:
		c.assistant = AssistantOllama
	default:
		c.assistant = AssistantAuto
	}
	return c
}

// Backend names where the next chat request will run, for the UI.
func (c *Client) Backend() string {
	switch {
	case c.assistant == AssistantOllama:
		return "Ollama"
	case c.assistant == AssistantFairy || FairyAvailable():
		return "Fairy"
	default:
		return "Ollama"
	}
}

// viaFairy runs a chat on Fairy when the mode allows it. handled is false when
// the request should go to Ollama instead.
func (c *Client) viaFairy(ctx context.Context, msgs []chatMessage, jsonMode bool) (text string, handled bool, err error) {
	if c.assistant == "" {
		c.assistant = AssistantAuto
	}
	if c.assistant == AssistantOllama {
		return "", false, nil
	}
	if c.assistant == AssistantAuto && !FairyAvailable() {
		return "", false, nil
	}
	instructions, prompt := flattenForFairy(msgs, jsonMode)
	text, err = askFairy(ctx, instructions, prompt)
	if errors.Is(err, errFairyUnavailable) && c.assistant == AssistantAuto {
		return "", false, nil
	}
	return text, true, err
}
