//go:build !nosound

// Package player streams and plays 30-second MP3 previews using faiface/beep.
//
// Previews are short, so each one is downloaded fully into memory (a seekable
// bytes.Reader). That keeps MP3 decoding robust and gives us an accurate track
// length for the progress bar.
//
// This is the real-audio implementation. A silent fallback (build tag
// `nosound`) lives in player_silent.go for environments without ALSA dev
// headers / a working audio device.
package player

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

// outputRate is the speaker's sample rate. Anything decoded at a different rate
// is resampled to this.
const outputRate = beep.SampleRate(44100)

// Player owns the single shared speaker and the currently playing stream.
type Player struct {
	mu sync.Mutex

	initOnce sync.Once
	initErr  error

	streamer beep.StreamSeekCloser // underlying MP3 stream (source-rate samples)
	format   beep.Format
	ctrl     *beep.Ctrl      // pause/resume
	volume   *effects.Volume // gain control
	body     io.Closer       // current decoder's ReadCloser

	paused bool
	gain   float64 // in beep "Volume" units (0 == unchanged)

	httpClient *http.Client
}

// New returns a Player. The speaker is initialised lazily on first Play so that
// constructing the UI never fails on a machine without working audio.
func New() *Player {
	return &Player{
		gain:       0,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (p *Player) ensureSpeaker() error {
	p.initOnce.Do(func() {
		p.initErr = speaker.Init(outputRate, outputRate.N(time.Second/10))
	})
	return p.initErr
}

// readSeekCloser adapts a bytes.Reader (seekable) to an io.ReadCloser, which is
// what mp3.Decode wants.
type readSeekCloser struct{ *bytes.Reader }

func (readSeekCloser) Close() error { return nil }

// Play downloads the preview at url and starts playback from the beginning.
// onDone, if non-nil, is invoked when the track finishes naturally (not when
// it is stopped or replaced).
func (p *Player) Play(url string, onDone func()) error {
	if err := p.ensureSpeaker(); err != nil {
		return fmt.Errorf("audio init failed: %w", err)
	}

	resp, err := p.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("downloading preview: %w", err)
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("reading preview: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("preview download returned status %d", resp.StatusCode)
	}

	rc := readSeekCloser{bytes.NewReader(data)}
	streamer, format, err := mp3.Decode(rc)
	if err != nil {
		return fmt.Errorf("decoding mp3: %w", err)
	}

	// Resample to the speaker rate if the source differs.
	var source beep.Streamer = streamer
	if format.SampleRate != outputRate {
		source = beep.Resample(4, format.SampleRate, outputRate, streamer)
	}

	p.mu.Lock()
	ctrl := &beep.Ctrl{Streamer: source, Paused: false}
	vol := &effects.Volume{Streamer: ctrl, Base: 2, Volume: p.gain, Silent: false}
	p.ctrl = ctrl
	p.volume = vol
	p.paused = false
	p.mu.Unlock()

	// Stop whatever was playing, then start the new chain. Seq + Callback lets
	// us learn when the stream drains on its own.
	speaker.Clear()
	p.closeStreamer()

	p.mu.Lock()
	p.streamer = streamer
	p.format = format
	p.body = rc
	p.mu.Unlock()

	done := beep.Callback(func() {
		if onDone != nil {
			onDone()
		}
	})
	speaker.Play(beep.Seq(vol, done))
	return nil
}

// TogglePause flips between paused and playing. Returns the new paused state.
func (p *Player) TogglePause() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctrl == nil {
		return p.paused
	}
	speaker.Lock()
	p.ctrl.Paused = !p.ctrl.Paused
	p.paused = p.ctrl.Paused
	speaker.Unlock()
	return p.paused
}

// Paused reports whether playback is currently paused.
func (p *Player) Paused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

// Stop halts playback and releases the current stream.
func (p *Player) Stop() {
	if p.initErr != nil {
		return
	}
	speaker.Clear()
	p.closeStreamer()
	p.mu.Lock()
	p.ctrl = nil
	p.volume = nil
	p.paused = false
	p.mu.Unlock()
}

func (p *Player) closeStreamer() {
	p.mu.Lock()
	s, b := p.streamer, p.body
	p.streamer, p.body = nil, nil
	p.mu.Unlock()
	if s != nil {
		s.Close()
	}
	if b != nil {
		b.Close()
	}
}

// Progress returns the elapsed and total duration of the current track.
func (p *Player) Progress() (elapsed, total time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.streamer == nil {
		return 0, 0
	}
	speaker.Lock()
	pos, length := p.streamer.Position(), p.streamer.Len()
	speaker.Unlock()
	return p.format.SampleRate.D(pos), p.format.SampleRate.D(length)
}

// HasTrack reports whether a stream is currently loaded.
func (p *Player) HasTrack() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.streamer != nil
}

// volumeStep is how much each +/- press changes the gain (beep Volume units).
const volumeStep = 0.5

// VolumeUp / VolumeDown adjust gain. Returned value is a 0..1-ish scalar purely
// for display; beep's Volume is logarithmic so we clamp to a friendly range.
func (p *Player) VolumeUp() float64   { return p.adjustVolume(volumeStep) }
func (p *Player) VolumeDown() float64 { return p.adjustVolume(-volumeStep) }

func (p *Player) adjustVolume(delta float64) float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gain += delta
	if p.gain > 2 {
		p.gain = 2
	}
	if p.gain < -4 {
		p.gain = -4
	}
	if p.volume != nil {
		speaker.Lock()
		p.volume.Volume = p.gain
		p.volume.Silent = p.gain <= -4
		speaker.Unlock()
	}
	return p.displayVolume()
}

// DisplayVolume maps the internal gain to a 0..1 scalar for the UI bar.
func (p *Player) DisplayVolume() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.displayVolume()
}

func (p *Player) displayVolume() float64 {
	// gain ranges -4..2 -> map to 0..1
	v := (p.gain + 4) / 6
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return v
}
