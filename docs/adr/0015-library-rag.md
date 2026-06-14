# ADR-0015: Library RAG — local embeddings, recommender, and chat

- Status: Accepted
- Date: 2026-06-14

## Context

On top of the local-LLM assistant ([ADR-0013](0013-local-nl-control.md)) the user
wanted three library-aware capabilities: **RAG over their library**, an **agent
recommender** based on their playlists, and **multi-turn chat**. All should stay
local (they run Ollama with Gemma + `nomic-embed-text`).

Two findings shaped the design:

1. **Spotify's recommendation engine is unavailable.** Verified against the live
   token: `/v1/recommendations` → 404, `/v1/audio-features` → 403, genre seeds →
   404 (the Nov-2024 Web API deprecation applies to newly-created apps; this app
   is new). `/v1/search` and `/v1/me` work (200). So recommendations cannot use
   Spotify's engine — they must be **LLM-driven over the user's own library and
   resolved to playable tracks via Search**.
2. **Local embeddings are cheap.** `nomic-embed-text` returns 768-dim vectors at
   ~3 ms/track batched; a 2503-track library indexes in ~33 s and gives high-
   quality semantic search ("daft punk electronic" → Starboy/I Feel It Coming by
   The Weeknd & Daft Punk).

Product decisions (from the user): recommendations favour **discovery** (suggest
new songs, library as taste signal); chat is a **multi-turn panel**.

## Decision

**A local semantic index + an LLM router, both reusing existing patterns.**

- **`internal/library`** builds and queries the index. It gathers full track
  metadata + playlist membership by adapting the `ExportURIs` pagination loop
  (deduped by track ID), embeds each track via an injected `Embedder`
  (implemented by `agent.Client`), stores **L2-normalized** vectors so `Search`
  is a dot-product top-K, and persists to `~/.config/audiopulse/library-index.gob`
  (gob — compact for float32). A `Signature` (playlist id/count + track count
  hash) marks staleness for rebuilds. `Sample`/`Filter`/`PlaylistNames` support
  the non-semantic parts of chat (counts, listings).
- **`internal/agent`** gains `Embed` (POST `/api/embed`, model from
  `ollama_embed_model`, default `nomic-embed-text`) and an expanded router: the
  existing `Interpret` now also yields `recommend` / `ask` / `reindex` actions.
  `Recommend(request, taste, n)` generates discovery suggestions as strict JSON;
  `parseSuggestions` is deliberately lenient (small models return a bare array, a
  renamed wrapper key, or braces inside titles — all handled, string-aware).
- **UI** loads the index on startup if present and **builds on demand** (first
  recommend/ask, or "reindex") behind a progress overlay that reuses the
  export/voice channel-streaming pattern. A recommend runs: embed the seed →
  `Search` the library for the closest owned tracks (the RAG taste context, or a
  `Sample` when there's no seed) → `Recommend` → resolve each suggestion via
  `client.Search` → load into the center and play. Everything is reachable from
  the same `:` prompt and `v` voice as the existing commands.

Chat (the multi-turn panel + `ask`) is built in a follow-up phase on this same
foundation.

## Consequences

**Positive**
- Recommendations work despite Spotify's dead rec API, and are grounded in the
  user's real taste; resolution via Search (which works) makes every pick
  playable. Validated end-to-end (8/8 suggestions resolved).
- Fully local index + generation; no new Go dependency (plain `net/http` +
  `encoding/gob`); no build tag.
- Reuses `runAgentCommand`, `agentPlayMsg`-style loading, and the export-style
  progress overlay — small surface area.

**Negative / trade-offs**
- First use pays a one-time index build (~30 s for ~2500 tracks); cached after.
- The index is a point-in-time snapshot; library changes need a `reindex`
  (auto-staleness via `Signature` is wired but only checked on explicit rebuild
  to avoid re-gathering on every request).
- Small-model recommendation quality varies; mitigated by lenient parsing,
  Search resolution, and a taste-grounded prompt.
- ~7.7 MB gob index on disk for a 2500-track library.

## Alternatives considered
- **Spotify's recommendations/audio-features** — dead for this app (verified).
- **Keyword/BM25 retrieval instead of embeddings** — no extra model, but misses
  semantic similarity ("electronic", "for studying"); embeddings are local and
  cheap, so not worth the downgrade.
- **A vector DB** — overkill; a linear dot-product scan over a few thousand
  normalized vectors is sub-millisecond.
- **Fine-tuning** — unnecessary; prompt + RAG context suffices.
