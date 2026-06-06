# ADR-0004: Build-tag silent-audio fallback

- Status: Accepted
- Date: 2026-06-06

## Context

The real audio backend ([ADR-0003](0003-beep-for-audio-playback.md)) depends on
cgo and the system audio library. On Linux this requires the ALSA development
headers (`libasound2-dev`) at build time and a working audio device at run time.

This is a problem in several common situations:

- **CI and headless servers** — no audio device, and installing native dev
  packages is undesirable.
- **Restricted environments** — contributors without root cannot install
  `libasound2-dev`.
- **WSL2 and containers** — frequently lack an audio bridge.

We did not want any of these to block *building, testing, or demoing* the
application, and we wanted the test suite to run anywhere.

## Decision

Provide **two implementations of the same `Player` API**, selected at compile
time with Go build tags:

- `player_beep.go` — `//go:build !nosound` — the real audio backend.
- `player_silent.go` — `//go:build nosound` — a no-sound simulation that advances
  a progress clock in real time, supports pause/resume, and fires the same
  completion callback so autoplay works.

Both files declare identical exported types and methods, so **no other package
is aware of which backend is compiled in**. The default build is real audio; the
silent build is opt-in via `-tags nosound`. The test suite and CI run against the
silent backend.

## Consequences

**Positive**

- The project **builds and runs anywhere**, including machines with no audio
  stack and no ALSA headers.
- Tests need neither hardware nor native dependencies, so CI is simple and fast
  and contributor onboarding is frictionless.
- The simulation preserves observable behaviour (progress, pause, autoplay),
  making the silent build a faithful way to develop and demo the UI.

**Negative / trade-offs**

- Two implementations of one contract must be kept in sync. This is mitigated by
  the small, stable surface of the `Player` API and by tests exercising it
  through the silent backend.
- The active backend is a compile-time choice, not a runtime one; switching
  requires a rebuild. This was an acceptable simplification versus a runtime
  abstraction (interface + factory), which would add indirection for little
  practical gain here.

## Alternatives considered

- **Runtime interface + device detection** — pick a backend at startup based on
  device availability. More flexible but adds an interface, a factory, and a
  nil-device code path throughout; the compile-time split is simpler and makes
  the no-cgo build genuinely dependency-free.
- **Always require audio** — rejected; it would make CI, headless, and
  restricted environments unable to build or test the project at all.
