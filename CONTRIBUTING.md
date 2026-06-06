# Contributing to AudioPulse

Thanks for your interest in improving AudioPulse. This document describes how to
set up a development environment, the standards we hold code to, and how to get
changes merged.

By participating you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Table of contents

- [Ways to contribute](#ways-to-contribute)
- [Development environment](#development-environment)
- [Project layout](#project-layout)
- [Coding standards](#coding-standards)
- [Testing](#testing)
- [Commit and PR conventions](#commit-and-pr-conventions)
- [Review process](#review-process)
- [Reporting bugs and requesting features](#reporting-bugs-and-requesting-features)

## Ways to contribute

- **Report bugs** and **request features** via the issue templates.
- **Improve documentation** — corrections, clarifications, and examples are
  always welcome.
- **Submit code** — bug fixes, new features, refactors, and tests.

If you plan a non-trivial change, open an issue first so we can agree on the
approach before you invest time.

## Development environment

**Prerequisites**

- Go 1.22 or newer
- `git`, `make`, and a C toolchain (`gcc`/`cc`)
- ALSA development headers for the audio backend:

  ```bash
  sudo apt-get install -y libasound2-dev    # Debian/Ubuntu
  # sudo dnf install alsa-lib-devel          # Fedora/RHEL
  ```

**Clone and build**

```bash
git clone <repository-url> AudioPulse
cd AudioPulse
make build          # real audio backend
make run            # build and launch
```

If you do not have audio hardware or ALSA headers, use the silent backend:

```bash
make run-silent     # no audio dependencies
```

See [docs/development.md](docs/development.md) for a deeper tour.

## Project layout

```
main.go                     Program entry point
internal/deezer/            Deezer API client
internal/player/            Audio backends (beep + silent fallback)
internal/ui/                Bubble Tea model, update, view, styles
docs/                       Documentation and ADRs
```

`internal/` packages are deliberately not part of any public API surface; treat
their exported symbols as internal-only.

## Coding standards

- **Formatting** is enforced with `gofmt`. Run `make fmt` before committing; CI
  fails on unformatted code.
- **Static analysis**: `make vet` must pass cleanly.
- **Naming and idiom**: follow [Effective Go](https://go.dev/doc/effective_go)
  and the existing style in the surrounding code.
- **Comments**: exported identifiers carry doc comments. Explain *why*, not
  *what*, where the code is non-obvious.
- **Errors**: wrap with context (`fmt.Errorf("doing X: %w", err)`); never
  discard errors silently.
- **Concurrency**: any code touching the player must respect the lock ordering
  documented in [docs/architecture.md](docs/architecture.md#concurrency-model)
  (acquire `Player.mu` before `speaker.Lock()`, never the reverse).
- **Dependencies**: keep them minimal and well-justified. Discuss new
  dependencies in an issue first.

## Testing

All changes should be covered by tests where practical.

```bash
make test                                   # unit + render tests (silent backend)
go test ./internal/deezer/ -run Live -v     # opt-in live Deezer API test
```

- UI logic is tested by driving the `tea.Model` through messages and asserting on
  state and rendered output — no terminal required.
- Tests run against the `nosound` backend so they need neither audio hardware nor
  ALSA headers.
- The live Deezer test is skipped under `-short` and in CI; run it locally when
  touching the client.

## Commit and PR conventions

- Write imperative, present-tense commit subjects ("Add volume persistence"), not
  past tense.
- Keep each PR focused on a single concern; split unrelated changes.
- Update `CHANGELOG.md` under `[Unreleased]` for user-visible changes.
- Update documentation alongside behavioural changes.
- Ensure `make fmt vet test` all pass locally before opening a PR.

## Review process

1. Open a PR using the template; link any related issue.
2. CI must be green (build, vet, gofmt, test).
3. At least one maintainer review and approval is required to merge.
4. We squash-merge by default; write a clear PR title — it becomes the commit.

## Reporting bugs and requesting features

Use the GitHub issue templates. For security-sensitive reports, **do not** open a
public issue — follow [SECURITY.md](SECURITY.md) instead.
