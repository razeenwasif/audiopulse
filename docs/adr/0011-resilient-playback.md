# ADR-0011: Resilient playback (librespot supervision + device recovery)

- Status: Accepted
- Date: 2026-06-09

## Context

AudioPulse depends on a librespot child process (the "AudioPulse" Connect device)
and a captured `device_id`. Two failure modes wedged a long-running session:

1. **librespot crashed** — `Supervisor.Start()` launched it once; if it died,
   playback was gone until the user restarted the whole app.
2. **The device id went stale** — the id was discovered once at startup and held
   by the UI. If librespot restarted (new id) or the Connect device dropped,
   every Web API playback command failed silently against the old id.

Some related concerns were already covered and didn't need new work: the Web API
client is built with `WithRetry(true)` (429 `Retry-After` backoff), the OAuth
token source auto-refreshes, and `pollCmd` already ignores transient poll errors.

## Decision

Split the responsibility in two, so neither side has to know the other's
internals:

**Keep the process alive (librespot package).** Replace `Start`/`Stop` with
`Supervisor.Run(ctx)`, a goroutine that starts librespot and restarts it on
unexpected exit with exponential backoff (1s → 30s; reset to 1s once a child has
run ≥10s, so a one-off crash recovers fast but a crash loop doesn't busy-spin).
Cancelling `ctx` kills the child and returns; `main` waits on a `done` channel so
the process is reaped on exit. Logs are opened append-mode so history survives
restarts.

**Keep targeting the right device (UI + client).** Add `Client.FindDevice` (a
single-shot lookup by name). When a playback action returns a device-shaped error
(`isDeviceError`), the UI fires `recoverDeviceCmd`, which re-resolves the
"AudioPulse" device by name, transfers playback back to it, and updates
`m.deviceID`. The UI never talks to the supervisor; it just re-discovers the
device through the Web API whenever an action fails.

## Consequences

**Positive**
- A librespot crash self-heals; the next action reconnects to the fresh device.
- The two mechanisms are decoupled and independently testable (backoff/sleep
  helpers; `isDeviceError` + the `actionMsg`/`deviceMsg` flow).
- No new scopes, dependencies, or user-visible setup.

**Negative / trade-offs**
- Recovery is reactive: the user may see one failed action and a brief
  "reconnecting…" status before the next succeeds (not a fully seamless gap).
- Device re-resolution is by name; if two "AudioPulse" devices briefly coexist
  during a restart, the first is chosen.
- `Run` spawning real processes isn't unit-tested (only its pure helpers are);
  end-to-end restart behavior is verified manually.

## Alternatives considered
- **Supervisor pushes the new device id to the UI** (channel/callback) — tighter
  coupling across the process boundary; the by-name re-resolution is simpler and
  also covers non-crash drops.
- **Proactive health checks** (poll device presence every tick and pre-emptively
  reconnect) — more API traffic for little gain; reacting to the first failed
  action is sufficient and cheaper.
