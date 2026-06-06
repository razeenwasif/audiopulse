---
name: Bug report
about: Report a problem with AudioPulse
title: "[Bug]: "
labels: bug
---

## Description

<!-- A clear, concise description of the bug. -->

## Steps to reproduce

1.
2.
3.

## Expected behaviour

<!-- What you expected to happen. -->

## Actual behaviour

<!-- What actually happened. Include the status-line message if any. -->

## Build variant

- [ ] Real audio (`make build` / `make run`)
- [ ] Silent (`-tags nosound` / `make run-silent`)

## Environment

<!-- Paste the output of these commands. -->

```
go version
uname -a
pkg-config --modversion alsa   # Linux, if applicable
echo "$TERM / $COLORTERM / $LANG"
```

## Additional context

<!-- Screenshots, logs, or anything else relevant. For security issues, do NOT
file here — see SECURITY.md. -->
