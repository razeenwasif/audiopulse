//go:build vosk

// Package voice provides offline speech-to-text via Vosk (a local Kaldi-based
// recognizer). It captures microphone audio with ffmpeg (PulseAudio source),
// streams 16 kHz mono PCM into the recognizer, and returns the transcript when
// Vosk detects the end of an utterance.
//
// This file is compiled only with `-tags vosk` (built by `make voice`), which
// also downloads libvosk + a model into third_party/vosk. The default build uses
// voice_stub.go, so neither the native library nor a model is required normally.
package voice

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/vosk
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/vosk -lvosk -Wl,-rpath,${SRCDIR}/../../third_party/vosk
#include <vosk_api.h>
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unsafe"
)

const (
	// sampleRate is what the small Vosk models expect; we resample to it.
	sampleRate = 16000
	// captureTimeout bounds one Listen call so a silent mic can't hang it.
	captureTimeout = 12 * time.Second
	// defaultModelPath is where `make voice` puts the model (relative to the repo).
	defaultModelPath = "third_party/vosk/model"
)

// Available reports whether speech recognition is compiled into this binary.
func Available() bool { return true }

// Engine owns a loaded Vosk model. Loading the model is the expensive step
// (~1–2 s, tens of MB), so an Engine is opened once and reused; each Listen call
// spins up a fresh recognizer over the shared model.
type Engine struct {
	model  *C.VoskModel
	source string // PulseAudio source name (e.g. "default", "RDPSource")
}

// Open loads the Vosk model and prepares capture from the given PulseAudio
// source. Empty modelPath/source fall back to sensible defaults.
func Open(modelPath, source string) (*Engine, error) {
	if modelPath == "" {
		modelPath = defaultModelPath
	}
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("vosk model not found at %q — run `make voice` to download it (%w)", modelPath, err)
	}
	if source == "" {
		source = "default"
	}
	C.vosk_set_log_level(C.int(-1)) // silence Vosk's verbose stderr logging

	cpath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cpath))
	model := C.vosk_model_new(cpath)
	if model == nil {
		return nil, fmt.Errorf("failed to load vosk model at %q", modelPath)
	}
	return &Engine{model: model, source: source}, nil
}

// Close frees the model.
func (e *Engine) Close() error {
	if e.model != nil {
		C.vosk_model_free(e.model)
		e.model = nil
	}
	return nil
}

// recognizer is a per-utterance Vosk recognizer over the engine's model.
type recognizer struct{ rec *C.VoskRecognizer }

func (e *Engine) newRecognizer() *recognizer {
	return &recognizer{rec: C.vosk_recognizer_new(e.model, C.float(sampleRate))}
}

func (r *recognizer) free() {
	if r.rec != nil {
		C.vosk_recognizer_free(r.rec)
		r.rec = nil
	}
}

// accept feeds one PCM chunk and reports whether Vosk reached an utterance
// endpoint, with the recognized text at that point.
func (r *recognizer) accept(buf []byte) (endpoint bool, text string) {
	if len(buf) == 0 {
		return false, ""
	}
	res := C.vosk_recognizer_accept_waveform(r.rec, (*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf)))
	if res > 0 {
		return true, parseField(C.GoString(C.vosk_recognizer_result(r.rec)), "text")
	}
	return false, ""
}

// final returns the recognized text accumulated so far (flushes the recognizer).
func (r *recognizer) final() string {
	return parseField(C.GoString(C.vosk_recognizer_final_result(r.rec)), "text")
}

// parseField pulls a string field out of Vosk's JSON result, e.g. {"text":"..."}.
func parseField(jsonStr, key string) string {
	var m map[string]any
	if json.Unmarshal([]byte(jsonStr), &m) == nil {
		if s, ok := m[key].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// Listen captures from the microphone until you stop speaking (Vosk endpoint),
// ctx is canceled, or captureTimeout elapses, and returns the transcript. An
// empty string (no error) means nothing intelligible was heard.
func (e *Engine) Listen(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, captureTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-f", "pulse", "-i", e.source,
		"-ar", fmt.Sprint(sampleRate), "-ac", "1", "-f", "s16le", "pipe:1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var errb strings.Builder
	cmd.Stderr = &errb
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("starting capture (ffmpeg): %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	rec := e.newRecognizer()
	defer rec.free()

	buf := make([]byte, 3200) // ~100 ms at 16 kHz s16le mono
	for {
		n, rerr := stdout.Read(buf)
		if n > 0 {
			if endpoint, text := rec.accept(buf[:n]); endpoint && text != "" {
				return text, nil
			}
		}
		if rerr != nil || ctx.Err() != nil {
			break
		}
	}
	if t := rec.final(); t != "" {
		return t, nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "", nil // heard nothing in time — not an error
	}
	if errb.Len() > 0 {
		return "", fmt.Errorf("capture failed: %s", strings.TrimSpace(errb.String()))
	}
	return "", nil
}
