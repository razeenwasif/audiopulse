//go:build vosk

package voice

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoModelPath resolves third_party/vosk/model as an absolute path from this
// test file's location, so the test works regardless of the working directory.
func repoModelPath() string {
	_, file, _, _ := runtime.Caller(0) // .../internal/voice/voice_vosk_test.go
	return filepath.Join(filepath.Dir(file), "..", "..", "third_party", "vosk", "model")
}

// TestModelLoadAndRecognize is a smoke test for the native toolchain: it loads
// the real model (proving libvosk + CGo + the model directory are wired up) and
// feeds 1 s of silence through a recognizer. Silence transcribes to empty text,
// so we only assert it loads and runs without crashing. Requires `make voice`.
func TestModelLoadAndRecognize(t *testing.T) {
	eng, err := Open(repoModelPath(), "")
	if err != nil {
		t.Skipf("model not available (run `make voice`): %v", err)
	}
	defer eng.Close()

	rec := eng.newRecognizer()
	defer rec.free()

	// 1 second of silence: 16000 samples * 2 bytes, fed in 100 ms chunks.
	chunk := make([]byte, 3200)
	for i := 0; i < 10; i++ {
		rec.accept(chunk)
	}
	if got := rec.final(); strings.TrimSpace(got) != "" {
		t.Errorf("silence should transcribe to empty text, got %q", got)
	}
}

func TestParseField(t *testing.T) {
	if got := parseField(`{"text":"  play queen "}`, "text"); got != "play queen" {
		t.Errorf("parseField text = %q, want \"play queen\"", got)
	}
	if got := parseField(`{"partial":"pl"}`, "text"); got != "" {
		t.Errorf("missing field should yield empty, got %q", got)
	}
	if got := parseField("not json", "text"); got != "" {
		t.Errorf("bad json should yield empty, got %q", got)
	}
}
