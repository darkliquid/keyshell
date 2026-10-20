# Keyshell card composition — one generic card, derived from item data

Date: 2026-10-20
Branch: `card-composition`
Status: design, approved in chat; not yet implemented

## Scope

Replacing the ported per-delegate card layouts with one generic card that the
host composes from typed item fields. The plugin seam gains status, artwork and
display hints; composition, budget and overflow are host policy.

Not in scope: the reveal expand/scroll animation, the expanded (console) card
that overflow hands off to, and live toplevel capture. `Preview` is carried in
the item schema here but drawn as a placeholder until a capture provider exists.

## The problem

The QML overlay had thirteen delegate bodies
(`quickshell/keyshell/components/*CardDelegate.qml` at `0b5b223`). Between them
they contain eleven distinct parts and three layouts, and every backend shipped
a delegate name on each item to select one. The repetition is layout code, not
information: the same chip, icon box, text column and footer band appear in
nine of them, differing only in pixel constants.

## Part vocabulary

| part | shape | render |
| --- | --- | --- |
| `badge` | category chip, header left | rounded rect r2, h18, width `min(advance+10, 0.5w)`, 9px, elided; tone `accent`/`alt` |
| `status` | state chip, header right | same geometry, tone by semantics |
| `visual` | icon, glyph, image or preview | `bare` (glyph, hero icon) or `framed` (44×44 box, 2px border, 26–28px art), fallback glyph |
| `title` | primary text | `hero` (centred, 2 wrapped lines), `line` (1 line, upper case), `wrap` (2–3 lines) |
| `subtitle` | secondary text | 1 line, 9–10px, muted or slab when selected |
| `detail` | multi-line body | up to 3 wrapped lines, only when there is no `visual` |
| `meter` | ratio readout | `bar` or `ticks`; optional left label, right value label |
| `timeline` | read-only ratio | bar plus elapsed/total labels |
| `note` | inline state pill | 1 line, e.g. `[ MUTED ]`, only when the value is true |
| `hold` | hold-to-confirm progress | 10 sheared ticks plus a percentage |
| `footer` | hint line | 9px muted, centred; defaults derived from actions |

`badge` and `status` share the header row and do not consume body budget.

## Layouts

Layout is derived, not declared:

- **hero**: `visual` and `title`, no `subtitle`, `detail`, accessories or
  `footer`, and a primary action that launches (`exec`, `exec-terminal`,
  `open-url`). App-style centred stack. A copy or plugin action makes the same
  item a row card instead, which is what keeps a clipboard entry from becoming
  a hero card.
- **body**: `detail` present. Full-width text block, optional leading glyph,
  footer.
- **row**: everything else. Header row, then leading `visual` beside a text
  column, then the footer band.

## Budget

A card is a vertical budget. The header and footer are fixed claims; only
accessories compete.

| zone | contents | cap |
| --- | --- | --- |
| header | `badge`, `status` | one row, both optional |
| body | `title` | 1 row (hero: 2 wrapped) |
| body | `subtitle` | 1 row |
| body | `detail` | 3 rows, only without `visual` |
| body | accessories: `meter`, `timeline`, `note` | **2 rows** |
| footer | `hint` plus overflow marker | 1 row |

The cap is hard-coded at two accessory rows. Geometry allows more, but density
does not: two rows is what a 320–360px card reads as before it becomes a table.

## Eligibility

The control kind is the first filter, applied before any ranking:

| control kind | card form |
| --- | --- |
| `range` | `meter`; 10 ticks when the control is ratio-like (`Min <= 0 < Max <= 1`, as volume is), otherwise a bar |
| `toggle` | `note` pill, rendered only when true |
| `choice` | value chip, only when pinned |
| `progress` | `timeline` |
| `text` | never on a card; expanded view only |

`text` is excluded outright. A passphrase, a filename edit or a command argument
has no card form, which is also what keeps the network PSK item honest.

## Which accessories, when there are more than two

Ranked, first match wins, declaration order as the tiebreak:

1. **Pinned** — `Control.Display == "card"`. Plugin intent beats every
   heuristic; pinned controls keep their declared order.
2. **Primary** — `Control.Primary`. At most one per item, and the host never
   invents it: it is the value the item is adjustable by.
3. **Anomalous** — a value in a state worth naming: a toggle that is true
   (muted, disconnected) or a ratio below 20% of its range (battery, signal).
   Healthy state stays silent.
4. Declaration order, with `progress` last within that.

An explicit `Display == "expanded"` is never overridden by the heuristic.

## Overflow

When eligible accessories exceed the budget, the host sets `Expandable` itself
and the footer gains a marker: `[ENTER] CONNECT  //  [SPACE]  +2`, where `+2` is
the count of undisplayed controls. That marker is the only place a hidden value
is summarised, and it is the affordance that teaches the expanded view exists.

The expanded view is the follow-on step: the QML console views
(`VolumeSliderView`, `NetworkConsoleView`, `PowerConsoleView`,
`MediaConsoleView`, `TrayMenuView`, `BluetoothConsoleView`, and
`NetworkPasswordView` for the text-entry case) collapse into one generic
expanded card.

## Override path

`Item.Card []CardPart` is an ordered list naming part kinds, optional control
ids and optional text/tone. When present it replaces the derived body wholesale;
header and footer still come from the derive rules, because every card has them.
It exists for vocabulary gaps, not for per-backend layout code.

## Type changes

```go
type Tone string // "accent", "neutral", "info", "positive", "warn", "alert",
                 // "attention"

type ControlKind string // adds "progress" to range, toggle, choice, text

type Display string // "auto" (default), "card", "expanded"

type PartKind string // badge, status, visual, title, subtitle, detail,
                     // meter, timeline, note, hold, footer

type CardPart struct {
    Kind    PartKind
    Control string
    Text    string
    Tone    Tone
}
```

`Item` gains `Status`, `StatusTone`, `Icon`, `Image`, `Preview`, `Detail`,
`Hint` and `Card`. `Icon` is an icon-theme name and `Glyph` stays literal text,
so a plugin can send an emoji without the host looking it up in a theme. `Control` gains `Display` and `Primary`. The additions are mirrored in
`internal/plugin/proto`, the SDK and the plugin host, all optional and
backward-compatible at protocol version 1.

## Module shape

Composition is a pure function over item data, kept apart from drawing so the
budget and ranking rules are testable without a GPU context:

- `internal/card` — `Layout(item backend.Item) Plan`, where `Plan` carries the
  chosen layout, the ordered parts with resolved values, and the overflow count.
  No gg, no theme, no measurement.
- `internal/render` — draws a `Plan` inside the existing sheared card quad:
  measures text, elides, draws chips, meters, ticks and images.

The seam between them is the plan. Drawing may measure; it may not choose.

## Rendering notes

- Content is axis-aligned and inset, matching the QML (`clip: true` with
  `shearInset`); only the card background is sheared. `shearInset` is
  `ceil(|shear|*h/2)+4`, and content clips to the quad's inner rect.
- gg already provides what the parts need: `MeasureString`, `WordWrap`,
  `DrawStringWrapped` and `DrawStringAnchored` for text, `DrawImageEx` /
  `DrawImageRounded` for art under the current transform, `DrawGPUTexture` for a
  live preview, `ClipRect` for the content inset.
- gg has no elide, so one `elide(face, s, width)` helper is written against
  `Face.Advance`.
- Meter ticks reproduce the QML: 10 bars, 0.25 shear around their centre, filled
  by `round(ratio*10)`. Signal and battery reuse the same part with 5 bars.
- Selected cards invert as they do today: solid accent fill, slab text.

## Testing

- Unit tests in `internal/card` over fixture items that reproduce each of the
  thirteen QML delegates' data: assert chosen layout, part order, visible
  accessories, overflow count, and that `text` controls never reach the card.
- Ranking tests: pinned beats primary beats anomaly beats declaration order, and
  `expanded` suppresses a pin.
- Render tests: existing buffer-based tests extend to a card drawn from a plan,
  asserting ink pixels appear for a title and accent pixels for a selected chip.
- Live verification: grim screenshot pixel scans as before, since the renderer
  cannot judge composition by eye.

## Rollout

One change: `internal/render`'s card body switches from the two-field
`drawCard` (glyph and title) to drawing a plan. The sheared quad, shadow and
selection inversion are unchanged, so the visual identity is preserved while the
content becomes data-driven.