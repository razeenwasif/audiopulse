package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeFairy listens like Fairy's AudioPulse link (in a short temp dir, since
// Unix socket paths are limited to ~104 bytes) and answers with reply(request).
func fakeFairy(t *testing.T, reply func(map[string]string) map[string]string) (seen chan map[string]string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "fa")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("LYRA_DATA_DIR", dir)
	ln, err := net.Listen("unix", filepath.Join(dir, "audiopulse.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	seen = make(chan map[string]string, 4)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				line, err := bufio.NewReader(c).ReadBytes('\n')
				if err != nil {
					return
				}
				var req map[string]string
				_ = json.Unmarshal(line, &req)
				seen <- req
				out, _ := json.Marshal(reply(req))
				c.Write(append(out, '\n'))
			}(conn)
		}
	}()
	return seen
}

func TestFlattenForFairyKeepsInstructionsAndExamples(t *testing.T) {
	ins, prompt := flattenForFairy([]chatMessage{
		{Role: "system", Content: "Convert to JSON."},
		{Role: "user", Content: "skip"},
		{Role: "assistant", Content: `{"action":"next"}`},
		{Role: "user", Content: "play radiohead"},
	}, true)
	if !strings.HasPrefix(ins, "Convert to JSON.") || !strings.Contains(ins, "exactly one JSON object") {
		t.Fatalf("instructions = %q", ins)
	}
	for _, want := range []string{"Request: skip", `Reply: {"action":"next"}`, "play radiohead"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	if !strings.HasSuffix(prompt, "play radiohead") {
		t.Fatalf("the real request should come last:\n%s", prompt)
	}
}

func TestInterpretUsesFairyWhenSheIsRunning(t *testing.T) {
	seen := fakeFairy(t, func(req map[string]string) map[string]string {
		return map[string]string{"type": "done", "id": req["id"], "text": `{"action":"next","query":"","on":false,"repeat":"off","volume":0}`}
	})
	// An unreachable Ollama proves the request never went there.
	c := New("http://127.0.0.1:1", "", "")
	cmd, err := c.Interpret(context.Background(), "skip this one")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Action != ActionNext {
		t.Fatalf("action = %q", cmd.Action)
	}
	req := <-seen
	if req["type"] != "ask" || !strings.HasSuffix(req["prompt"], "skip this one") || req["instructions"] == "" {
		t.Fatalf("request = %#v", req)
	}
	if c.Backend() != "Fairy" {
		t.Fatalf("backend = %q", c.Backend())
	}
}

func TestFairyErrorsReachTheUser(t *testing.T) {
	fakeFairy(t, func(req map[string]string) map[string]string {
		return map[string]string{"type": "error", "id": req["id"], "message": "AudioPulse needs a subscription provider."}
	})
	_, err := New("http://127.0.0.1:1", "", "").Interpret(context.Background(), "skip")
	if err == nil || !strings.Contains(err.Error(), "subscription provider") {
		t.Fatalf("err = %v", err)
	}
}

func TestAutoFallsBackToOllamaWithoutFairy(t *testing.T) {
	t.Setenv("LYRA_DATA_DIR", t.TempDir())
	c := New("http://127.0.0.1:1", "", "")
	if c.Backend() != "Ollama" {
		t.Fatalf("backend = %q", c.Backend())
	}
	if _, err := c.Interpret(context.Background(), "skip"); err == nil || !strings.Contains(err.Error(), "Ollama") {
		t.Fatalf("expected an Ollama error, got %v", err)
	}
	// Forcing Fairy without her running says so instead of trying Ollama.
	if _, err := New("http://127.0.0.1:1", "", "").WithAssistant("fairy").Interpret(context.Background(), "skip"); err == nil || !strings.Contains(err.Error(), "Fairy") {
		t.Fatalf("expected a Fairy error, got %v", err)
	}
}
