# ADR-0014: Offline voice control via Vosk

- Status: Accepted
- Date: 2026-06-14

## Context

After the `:` text assistant ([ADR-0013](0013-local-nl-control.md)), the user
wanted to **speak** requests ("play bohemian rhapsody", "skip", "shuffle on").
Gemma has no audio modality, and Ollama doesn't do speech recognition — so this
needs a separate **speech-to-text** stage that produces the same text the agent
already consumes.

Constraints that shaped the choice:
- **Offline / local**, consistent with the rest of AudioPulse (no cloud ASR, no
  API key). Pairs with the local-library direction.
- **No Python** (the user's stated preference).
- Must run under **WSL2**, where the microphone is the Windows mic forwarded by
  WSLg as the PulseAudio `RDPSource`.
- Terminals deliver discrete key events with **no key-release**, so classic
  hold-to-talk push-to-talk isn't possible.

## Decision

**Vosk for ASR, behind a `vosk` build tag, feeding the existing agent.**

- **`internal/voice`** wraps `libvosk` (a local Kaldi recognizer) via a thin CGo
  binding. The `#cgo` directives use `${SRCDIR}`-relative paths and embed an
  **rpath**, so a tagged build links and runs without `LD_LIBRARY_PATH` or env
  vars — `make voice` is just `go build -tags vosk`.
- **Build-tag isolation** mirrors the existing `nosound` pattern: `voice_vosk.go`
  (`//go:build vosk`) is the real implementation; `voice_stub.go`
  (`//go:build !vosk`) returns "not built in". The default build and CI need
  neither the native library nor a model; `voice.Available()` gates the UI.
- **Capture** is an `ffmpeg` subprocess reading the PulseAudio source
  (`-f pulse -i default`) as 16 kHz mono `s16le` on stdout — no extra Go audio
  dependency, and ffmpeg is already a project dependency (spotDL).
- **No push-to-talk; use endpointing.** Press **`v`** → capture starts → Vosk's
  built-in endpoint detection returns the transcript when you stop speaking (a
  12 s safety timeout bounds a silent mic). This sidesteps the terminal's lack of
  key-release events.
- **Reuse the agent pipeline.** A transcript is fed straight into `interpretCmd`,
  exactly as if typed at the `:` prompt — so spoken and typed control share one
  code path and one set of actions. The model is loaded once into a reused
  `Engine` (loading is the expensive step); each utterance gets a fresh
  recognizer.
- **Assets are fetched, not committed.** `make voice` runs
  `scripts/fetch-vosk.sh` to download `libvosk.so` + a small English model into
  `third_party/vosk/` (gitignored). `voice_model` / `voice_source` in
  `config.json` override the model path and capture source. `make doctor` checks
  ffmpeg, the Vosk files, and the default capture source (warning if muted).

## Consequences

**Positive**
- Fully offline, no Python, no API — consistent with the project's direction.
- Spoken control inherits every agent action and its device-recovery/glyph
  behaviour for free; the voice layer is just "audio → text".
- Default build, tests, and CI are untouched (stub); the native complexity is
  opt-in and contained behind one tag and one directory.
- Validated end-to-end on WSL2: WSLg's `RDPSource` captures fine at 16 kHz.

**Negative / trade-offs**
- A tagged build needs a C toolchain + `libvosk.so` (≈7 MB) and a model (≈40 MB);
  hence opt-in via `make voice`.
- The small model trades some accuracy for size/speed; swap in a larger Vosk
  model via `voice_model` if needed.
- CGo means the voice build isn't pure-Go and is currently linux-x86_64 only
  (the only prebuilt lib we fetch).
- Mic reliability depends on the OS/WSLg audio config; surfaced via `doctor`
  rather than hidden.

## Alternatives considered
- **whisper.cpp** — higher accuracy, but heavier models, slower on CPU, and a
  larger native surface; Vosk's streaming endpointing fits push-to-talk-less
  terminal UX better. Could be added later as an alternate backend behind the
  same `voice.Engine` interface.
- **Gemma 3n (audio-in multimodal)** — promising, but audio input through Ollama
  isn't a stable path yet and ASR quality is modest; revisit later.
- **OS/Windows dictation piped in** — not self-contained, awkward across the
  WSL boundary.
- **A real push-to-talk key** — terminals don't report key-release; endpointing
  is the natural substitute.

## Future
The `voice.Engine` seam (open model → `Listen` → text) can host a whisper.cpp or
Gemma-3n backend without touching the UI. Continuous "wake-word" listening, or
streaming partial transcripts into the listening overlay, are natural follow-ons.
