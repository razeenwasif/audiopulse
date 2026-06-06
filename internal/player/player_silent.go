//go:build nosound

// Package player — silent fallback used when built with `-tags nosound`.
//
// It emits no audio but faithfully simulates a 30-second preview: the progress
// bar advances in real time, pause/resume freezes the clock, and onDone fires
// when the simulated track ends (so autoplay still works). This lets AudioPulse
// build and run on machines without ALSA dev headers or a sound device.
package player

import (
	"sync"
	"time"
)

const previewLen = 30 * time.Second

// Player simulates playback with no sound output.
type Player struct {
	mu sync.Mutex

	playing     bool
	paused      bool
	total       time.Duration
	startedAt   time.Time
	pausedAt    time.Time
	pausedAccum time.Duration
	gain        float64
	stop        chan struct{}
}

// New returns a silent Player.
func New() *Player { return &Player{gain: 0} }

// Play "starts" a track. The url is ignored; onDone fires after previewLen of
// (un-paused) wall-clock time.
func (p *Player) Play(_ string, onDone func()) error {
	p.mu.Lock()
	if p.stop != nil {
		close(p.stop)
	}
	stop := make(chan struct{})
	p.stop = stop
	p.playing = true
	p.paused = false
	p.total = previewLen
	p.startedAt = time.Now()
	p.pausedAccum = 0
	p.mu.Unlock()

	go p.monitor(stop, onDone)
	return nil
}

func (p *Player) monitor(stop chan struct{}, onDone func()) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			elapsed, total := p.Progress()
			if total > 0 && elapsed >= total {
				p.mu.Lock()
				p.playing = false
				if p.stop == stop {
					p.stop = nil
				}
				p.mu.Unlock()
				if onDone != nil {
					onDone()
				}
				return
			}
		}
	}
}

// TogglePause freezes/unfreezes the simulated clock.
func (p *Player) TogglePause() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.playing {
		return p.paused
	}
	if p.paused {
		p.pausedAccum += time.Since(p.pausedAt)
		p.paused = false
	} else {
		p.paused = true
		p.pausedAt = time.Now()
	}
	return p.paused
}

// Paused reports the simulated pause state.
func (p *Player) Paused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

// Stop ends the simulation.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stop != nil {
		close(p.stop)
		p.stop = nil
	}
	p.playing = false
	p.paused = false
}

// Progress returns simulated elapsed/total durations.
func (p *Player) Progress() (elapsed, total time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.playing {
		return 0, 0
	}
	ref := time.Now()
	if p.paused {
		ref = p.pausedAt
	}
	e := ref.Sub(p.startedAt) - p.pausedAccum
	if e < 0 {
		e = 0
	}
	if e > p.total {
		e = p.total
	}
	return e, p.total
}

// HasTrack reports whether a simulated track is loaded.
func (p *Player) HasTrack() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}

const volumeStep = 0.5

// VolumeUp / VolumeDown adjust the (display-only) volume.
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
	return p.displayVolume()
}

// DisplayVolume maps gain to a 0..1 scalar for the UI bar.
func (p *Player) DisplayVolume() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.displayVolume()
}

func (p *Player) displayVolume() float64 {
	v := (p.gain + 4) / 6
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return v
}
