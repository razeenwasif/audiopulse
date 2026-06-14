//go:build !vosk

// Package voice provides offline speech-to-text for hands-free control. The real
// implementation (build tag `vosk`) wraps the Vosk recognizer and captures from
// the microphone; this stub is compiled by default so the standard build and CI
// need neither the native libvosk nor a model. Build with `make voice` to enable
// it.
package voice

import (
	"context"
	"errors"
)

// ErrUnsupportedBuild is returned when the binary was built without voice support.
var ErrUnsupportedBuild = errors.New("voice support not built in — rebuild with `make voice`")

// Available reports whether speech recognition is compiled into this binary.
func Available() bool { return false }

// Engine is a no-op placeholder in builds without the `vosk` tag.
type Engine struct{}

// Open always fails in the stub build.
func Open(modelPath, source string) (*Engine, error) { return nil, ErrUnsupportedBuild }

// Close is a no-op.
func (e *Engine) Close() error { return nil }

// Listen always fails in the stub build.
func (e *Engine) Listen(ctx context.Context, onReady func()) (string, error) {
	return "", ErrUnsupportedBuild
}
