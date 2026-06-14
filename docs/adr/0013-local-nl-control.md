# ADR-0013: Natural-language playback control via a local LLM (Ollama/Gemma)

- Status: Accepted
- Date: 2026-06-14

## Context

The user wanted to control playback in plain language — "play bohemian
rhapsody", "turn shuffle on", "loop this song", "skip", "pause", "set the volume
to 30" — and already runs several Gemma variants locally through
[Ollama](https://ollama.com). The question raised was whether a model needs
fine-tuning to drive the app.

The set of controllable actions is small and fully covered by existing
`spotify.Client` methods: search+play, pause/resume, next/previous, shuffle,
repeat, volume. So the only hard part is turning a free-text utterance into one
of those actions with its parameters — a constrained classification + slot-filling
task, not open-ended generation.

## Decision

**No fine-tuning. Prompt + JSON mode, against the user's local Ollama.** A new
`internal/agent` package sends the utterance to Ollama's `/api/chat` with:

- a **strict system prompt** describing a fixed JSON schema
  (`action`, `query`, `on`, `repeat`, `volume`) plus a handful of worked
  **few-shot examples**, and
- `"format": "json"` and `temperature: 0`, so the model returns one valid JSON
  object deterministically.

The result is parsed and **normalized** in pure, tested code: the action is
lower-cased and checked against an allowlist (anything else → `unknown`), the
query trimmed, repeat synonyms canonicalised (`track`/`song`→`one`,
`playlist`/`context`→`all`), volume clamped to 0–100, and a query-less "play"
downgraded to `unknown`. A balanced-brace extractor tolerates a model that wraps
the object in stray prose.

**Model selection is automatic.** With no `ollama_model` configured, the client
queries `/api/tags` and picks the first installed `gemma*` model (else the first
model at all), caching the choice. `ollama_model` / `ollama_url` in `config.json`
override the model and endpoint.

**UI.** `:` opens a floating prompt mirroring the Spotlight overlay
(`panelAgent` + its own `textinput`). `enter` runs an async `Interpret` (the
prompt shows "Thinking…"); the returned `agent.Command` is dispatched through the
**same** `m.action(...)` path as the keyboard shortcuts — so playback errors get
the existing lost-device recovery for free, and the shuffle/repeat glyphs update
from local intent exactly as the `s`/`r`/`R` keys do. "Play" runs a Spotify
search and plays the top hit, reusing the context-less windowed-play path.

**It is optional and self-contained.** No new Go dependency (plain
`net/http`+`encoding/json`). If Ollama is down or has no model, `Interpret`
returns a friendly error shown in the prompt; nothing else in AudioPulse is
affected. `make ollama-model` pulls a default model and `make doctor` reports
Ollama's status.

## Consequences

**Positive**
- Works across the user's Gemma variants with zero training — JSON mode + few-shot
  is reliable for this constrained schema on small local models.
- Fully local: no data leaves the machine, no API key, no cost.
- Dispatch reuses existing command paths, so the assistant inherits device
  recovery and glyph behaviour and stays a thin layer.
- The parse/normalize/model-pick logic is pure and unit-tested without a network.

**Negative / trade-offs**
- Small models occasionally mis-slot an ambiguous request; mitigated by the
  allowlist + `unknown` fallback (it explains rather than guessing wildly) and a
  visible echo of the action in the status line.
- "Play" quality is bounded by Spotify search ranking on a short query.
- Requires the user to run Ollama and pull a model — hence strictly opt-in.

## Alternatives considered
- **Fine-tune a Gemma variant.** Unnecessary for slot-filling over a fixed schema;
  large effort, brittle to action-set changes, no quality win here.
- **OpenAI-style tool/function-calling tokens.** Support is inconsistent across
  Gemma builds in Ollama; a plain JSON object + `format:"json"` is more portable.
- **Local regex/keyword parser.** No new dependency at all, but brittle on
  phrasing ("put on", "I want to hear", "stop repeating") — the whole point was
  natural language, which the LLM handles far better.
- **A cloud LLM.** Rejected: defeats the local/private goal and adds cost and a
  key; the task is small enough for a local model.

## Future
This intent layer generalizes beyond transport control: a recommender ("queue
something like this"), conversational library Q&A, or RAG over the local export
are natural extensions (see the backlog in `CONTEXT.md`). They would add actions
to the schema and, for retrieval, a context-building step — but keep this same
prompt-not-fine-tune, local-only shape.
