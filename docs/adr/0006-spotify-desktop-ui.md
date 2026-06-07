# ADR-0006: Spotify-desktop visual design

- Status: Accepted
- Date: 2026-06-07
- Refines: earlier choice of green panel borders

## Context

The initial UI used the Spotify green as a prominent colour everywhere —
every panel had a green border. After Spotify mode landed, the goal became to
make the TUI resemble the **Spotify desktop client** (a reference screenshot was
provided): a top search bar, a left "Your Library" with thumbnailed two-line
rows, a center feed with filter chips and a track table, a right now-playing
column with album art, and a bottom player bar with centered transport controls.

The Spotify desktop UI is predominantly **monochrome** (near-black, white, gray)
with green used **sparingly** as an accent for active/playing state and the play
button — not as an everywhere-border colour.

## Decision

Adopt a Spotify-desktop-style layout and a "green-as-accent" theme:

- **Borders** are a subtle gray (`#2A2A2A`) by default; the **focused** panel's
  border turns green. Implemented via the `panelBox(focused, …)` helper.
- **Green** (`#1DB954`/`#1ED760`) is reserved for accents: the focused panel,
  the selected row, the playing track, the progress fill, the circular play
  button, and active shuffle/repeat.
- **Layout:** top bar (brand + centered search pill + user); left "Your Library"
  with colored thumbnail blocks, title, and subtitle per entry (two rows each);
  center with filter chips, a bold source title, a column header, and a numbered
  track table; right now-playing column with half-block album art, metadata, and
  the up-next queue; a 3-zone player bar (mini track · centered transport · volume)
  with a progress bar spanning the width.

This reverses the earlier "green borders everywhere" look in favour of matching
the reference.

## Consequences

**Positive**
- Much closer to the familiar Spotify desktop appearance.
- Green now signals *state* (focus/active), which is more informative.
- Layout geometry is centralised in helpers (`panelBox`, `panelDims`,
  `trackWindow`, `centerGeom`, `progressMetrics`) shared by rendering and mouse
  hit-testing, so clicks line up with what's drawn.

**Negative / trade-offs**
- Emoji-width glyphs (🔀 🔁 🔊 ⏱) in the transport/columns depend on the
  terminal reporting widths consistently; on terminals with different emoji
  widths, alignment can drift by a cell.
- Two-line library rows show fewer entries at once than the old one-line list.
- The center "feed" is a track table rather than Spotify's cover-art card
  carousels (rendering many cover thumbnails per frame is deferred as too heavy).

## Alternatives considered
- **Keep green borders** — rejected; it does not match the reference and over-uses
  the accent colour.
- **Cover-art card carousels in the center** — deferred: downloading and
  half-block-rendering many covers each frame is expensive; a numbered table is
  lighter and still readable.
