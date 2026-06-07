# Architecture Decision Records

This directory records the significant architectural decisions made on
AudioPulse, using lightweight
[ADRs](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions).

Each record captures the **context**, the **decision**, and its
**consequences** at a point in time. ADRs are immutable once accepted: if a
decision changes, add a new ADR that supersedes the old one rather than editing
history.

## Index

| ADR                                              | Title                                   | Status   |
| ------------------------------------------------ | --------------------------------------- | -------- |
| [0001](0001-language-and-tui-framework.md)       | Language and TUI framework              | Accepted |
| [0002](0002-deezer-as-data-source.md)            | Deezer as the catalogue data source     | Accepted |
| [0003](0003-beep-for-audio-playback.md)          | `faiface/beep` for audio playback       | Accepted |
| [0004](0004-build-tag-fallback-strategy.md)      | Build-tag silent-audio fallback         | Accepted |
| [0005](0005-spotify-via-librespot.md)            | Spotify full-song playback via librespot| Accepted |
| [0006](0006-spotify-desktop-ui.md)               | Spotify-desktop visual design           | Accepted |
| [0007](0007-spotlight-search-overlay.md)         | Spotlight-style search overlay          | Accepted |

## Statuses

- **Proposed** — under discussion.
- **Accepted** — the decision is in effect.
- **Superseded** — replaced by a later ADR (linked).
- **Deprecated** — no longer relevant.

## Template

```markdown
# ADR-NNNN: Title

- Status: Proposed | Accepted | Superseded by ADR-XXXX | Deprecated
- Date: YYYY-MM-DD

## Context
What problem are we solving? What constraints and forces apply?

## Decision
What did we decide to do?

## Consequences
What becomes easier or harder? What are the trade-offs and follow-ups?

## Alternatives considered
What else did we weigh, and why not those?
```
