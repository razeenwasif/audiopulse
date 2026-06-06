# AudioPulse Documentation

Welcome to the AudioPulse documentation. This directory is the canonical
reference for users, operators, and contributors.

## For users

- **[Getting Started](getting-started.md)** — install prerequisites, build, and
  run AudioPulse on Linux, macOS, WSL, and Windows.
- **[User Guide](user-guide.md)** — a tour of every screen, control, and
  workflow.
- **[Troubleshooting](troubleshooting.md)** — fixes for the most common audio,
  WSL, ALSA, and network problems.

## For developers

- **[Architecture](architecture.md)** — the system's components, data and
  control flow, concurrency model, and rendering pipeline, with diagrams.
- **[Development](development.md)** — environment setup, project layout, the
  build-tag strategy, and the testing approach.
- **[Configuration](configuration.md)** — build-time and runtime configuration.
- **[Architecture Decision Records](adr/)** — the *why* behind major technical
  choices.

## Governance

The following live at the repository root:

- [CONTRIBUTING.md](../CONTRIBUTING.md)
- [CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md)
- [SECURITY.md](../SECURITY.md)
- [CHANGELOG.md](../CHANGELOG.md)
- [LICENSE](../LICENSE)

## Documentation conventions

- Diagrams use [Mermaid](https://mermaid.js.org/) and render natively on GitHub.
- Shell snippets assume a POSIX shell unless stated otherwise.
- "The beep backend" refers to the default real-audio build; "the silent
  backend" refers to the `-tags nosound` build.
