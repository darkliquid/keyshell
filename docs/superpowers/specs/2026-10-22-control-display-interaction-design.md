# Keyshell control display and interaction — read-only cards vs an interactive control panel

Date: 2026-10-22
Status: design, pending approval; not yet implemented
Owner: keyshell
Branch: `feat/ksl-parser`

## Summary

Controls (audio volume, mute, media position, network toggles, text passphrases)
currently have exactly one visual form on a card: the **read-only accessory**
derived by `card.Compose` (a meter, a note pill, a chip, a timeline). There is
no way to *change* a control, and `launcher.openFocusView` is a stub. Audio
devices in particular "show no controls" because their volume/mute only appear
as passive meters/pills and Enter does nothing (the item has no launch action).

This spec defines two distinct display modes per control:

1. **On-card display (read-only)** — unchanged, already shipped by the card
   composition work: `range → Meter`, `toggle → note pill`, `choice → chip`,
   `progress → Timeline`, `text → never`.
2. **Interactive panel (controls)** — a panel that slides out from under the
   selected card when the card is activated (Enter). It hosts one widget per
   interactive control and is the *only* place controls become editable.

It then specifies the input contract for the panel, the scene bindings that
drive it, the launcher state that owns it, and the default UI KSL that
implements it.

## Goals

- A control has **one display face** on the card and **one interactive face**
  in the panel; nothing is both at once.
- Activation of a card with controls opens the panel; the panel is navigable
  with up/down between rows and left/right/enter to manipulate the focused row.
- The panel is expressed **entirely in KSL** with the existing primitive kit
  (Box/Text/Meter/Cursor/Stack/Column) plus two new generic widgets; the Go
  renderer gains no hard-coded control chrome.
- Input routing is mode-aware: reel navigation and panel manipulation never
  fight over the same keys.
- Runs on the shipped default scene without config.

## Current state (read from the tree)

- `internal/backend`: `Control{ID, Kind, Label, Value, Min, Max, Step, Bool,
  Text, Options, Display, Primary}`; kinds `range|toggle|choice|text|progress`.
- `internal/card` `Compose` maps controls to read-only parts (`range→meter`,
  `toggle→note`, `choice→chip`, `progress→timeline`, `text→excluded`) and
  ranks them (pinned > primary > anomalous > declaration order) into a 2-row
  accessory budget, with overflow folded into the footer marker.
- `internal/render` (`cardpainter.go`) draws that plan through `drawCard`; the
  scene's card template is just `Group#card { width… Card{} }`.
- `internal/launcher`: `Enter()` routes action → `openFocusView` (stub) →
  `descend`; `Control`/`DebouncedControl` already settle value changes by
  item+control id. `$active` binding already exposes the selected item.
- `internal/input` maps Up/Down/Left/Right → `Select(±1)`, Enter → `Enter()`,
  Escape → `Escape()`.
- `internal/ksl/eval`: `Item.Controls` is `map[string]ItemControl{Value, Ratio,
  On}` — order, kind, min/max/step, options and label are **lost** at the
  scene boundary.

## Design

### 1. Two faces, one contract

| kind | on-card (read-only) | panel widget (interactive) | manipulation |
| --- | --- | --- | --- |
| `range` | Meter (ticks/bar) | Slider | Left/Right step ±Step; PageUp/PageDown coarse |
| `toggle` | note pill (only when true) | Toggle row | Enter flips Bool |
| `choice` | value chip | Cycle row | Left/Right cycle Options; Enter commits |
| `text` | never shown | Text field | type appends, Backspace deletes, Enter commits |
| `progress` | Timeline | shown read-only, never focusable | none |

An item's **action row** is the first panel row whenever the item has a
primary action; it is a row like any other (focusable, Enter runs it) but has
no value to manipulate.

A control is **interactive** iff its kind is not `progress` and it is not
marked `Display: card` only when the item is *purely* display. In practice:
every control an item lists becomes a panel row except `progress` rows (which
are rendered read-only and skipped by focus).

### 2. Activation and navigation

- **Enter on a card**: if the item has at least one interactive control *or* a
  primary action, the panel opens for that card (this is the change — it takes
  precedence over the launch action *while a panel can exist*). The action
  becomes the first row of the panel, ahead of the controls (see §4).
  Otherwise Enter keeps today's behaviour (run action / descend / expand focus
  view).
- **Panel mode** (while open):
  - `Up`/`Down` — move focus between control rows.
  - `Left`/`Right` — manipulate the focused row (slider step, choice cycle).
  - `Enter` — activate the focused row (toggle flip, choice commit, text
    commit / move to edit).
  - typing (`OnTextInput`) — append to the focused `text` row's buffer.
  - `Backspace` — delete from the focused `text` buffer.
  - `Escape` — close the panel, return focus to the reel at the same card.
- Focus is a **single index** into the ordered interactive row list; it never
  leaves the panel while the panel is open.

### 3. Scene bindings

Replace `eval.Item.Controls` (map) with an ordered slice `[]Control` carrying
`ID, Kind, Label, Value, Min, Max, Step, Options, Bool, Text` (order =
declaration order from the backend). Keep `$item.<id>` access working by lookup
on ID. Add scene-wide bindings:

- `$active` — the selected item (already present).
- `$panel.open` — bool, whether the panel is open.
- `$panel.rows` — the ordered rows for `$active`: the action row first (when
  the item has a primary action), then the interactive control rows (progress
  excluded). `$panel.focus` indexes into this list.
- `$panel.focus` — index of the focused row (when open).
- `$panel.dirty` — true while a `text` row's buffer is being edited (drives
  cursor blink and commit affordance).

These are derived by the launcher/interpreter from live state on each frame,
like `$selected`/`$query`.

### 4. The panel itself

The panel is a KSL structure authored in the default scene (and reusable via
`def`). It is a `Column` anchored under the selected card, rendered inside the
same rotated content frame so it inherits the band's tilt — this is what makes
it "perpendicular to the angle of the band" (the reel's cross-axis). It slides
out with a `translate`/`opacity` transition on open.

Each row is one widget, chosen by `kind` via a `when`-gated `def`:

- **Slider row**: `Text` label + `Meter` (bar) whose `ratio` binds to the row's
  `Value`, plus a focused-state callout (accent border / brighter ink). The
  handle position is the ratio; no new draw primitive is required.
- **Toggle row**: `Pill`/`Box` whose fill reflects `Bool`, with a "ON/OFF"
  label.
- **Choice row**: `Text` showing `Options[Value]` with left/right chevrons
  (`Text` glyphs).
- **Text row**: a `Box` field with `Text` showing the buffer + the existing
  `Cursor` (blink animation) when focused.
- **Action row** (first row, only when the item has a primary action): a
  `Pill` labelled by the action hint; Enter runs it. It occupies index 0 of
  the focused row list, so it is the first thing Up/Down lands on and the
  primary action stays reachable once controls own Enter.

Focus is drawn via the scene selector `:selected`/a `state` block keyed on
`$panel.focus == $row.index`, exactly like the reel highlights the selected
card today.

### 5. Launcher state and verbs

Add to `Launcher`:

- State: `panel *panelState` where `panelState{ itemID ResultID, rows []row,
  focus int, edit string (text buffer) }`, nil when closed. A `row` is either
  the action row (`action *backend.Action`) or a control row (`control
  *backend.Control`); progress rows are never in the list.
- `Enter()` — if the selected item has interactive controls *or* a primary
  action, open `panel` (rows = [action?] + interactive controls; focus 0;
  keep selection); else current behaviour.
- `PanelNavigate(delta int)` — move `focus` within `[0, len(rows))`, clamping.
- `PanelAdjust(delta int)` — for the focused control row: `range` → step
  `Value` by `Step` (coarse ×5 with Shift), clamp to `[Min,Max]`; `choice` →
  move index into `Options`, wrap. Then settle via `DebouncedControl` (range)
  or `Control` (choice). No-op on the action row.
- `PanelActivate()` — action row → run the action (same path as today's
  `Enter` action routing); `toggle` → flip `Bool` → `Control`; `choice` →
  commit current option → `Control`; `text` → if not editing, enter edit mode
  (copy current `Text` to `edit`); if editing, commit `edit` → `Control` and
  leave edit mode.
- `PanelType(char string)` / `PanelBackspace()` — mutate `edit` while in edit
  mode on a `text` row.
- `Escape()` / `Back()` — if `panel != nil`, close it (clear `panel`) and do
  not pop the level; else current behaviour.

All mutations reuse the existing `Control`/`DebouncedControl` plumbing; the
backend/plugin already owns the actual effect (pactl, media, etc.).

### 6. Input routing (`internal/input`)

`Adapter.onKeyPress`/`onTextInput` query `launcher.PanelOpen()`:

- panel open → route to the panel verbs above (arrows, enter, escape, text,
  backspace);
- panel closed → today's routing (arrows = select, enter = activate, escape =
  dismiss).

This is the only place the two modes diverge.

### 7. Default UI KSL

The shipped `default.ksl` card template becomes a `Stack`/`Column` per card:
the `Card{}` (read-only plan) on top, and a `Panel` child revealed when
`$panel.open && $active.id == $item.id`. The panel is positioned at the card's
bottom edge and translated outward (the reel's cross-axis) with an open
transition, inheriting the band's `Group#content` rotation. The rest of the
scene is unchanged.

## Files touched

- `internal/backend` — (none; `Control` already carries everything).
- `internal/ksl/eval` — `Item.Controls` map → ordered `[]Control`; `$panel.*`
  bindings; keep `$item.<id>` lookup.
- `internal/launcher` — panel state + verbs; `Enter`/`Escape`/`Back` routing;
  `PanelOpen()` accessor.
- `internal/input` — mode-aware routing.
- `internal/render` — wire the new bindings into `state()`/`itemFromBackend`;
  a `Slider`/`Toggle`/`Choice`/`TextField` *component* may be authored in KSL
  alone (no new Go painter) or, where measurement is awkward, small painters.
  Prefer KSL-only first; add painters only if needed.
- `internal/render/default.ksl` — panel structure in the card template.
- tests: launcher panel verbs, input routing, eval bindings, and a render test
  asserting the panel's accent pixels appear in the open state.

## Out of scope

- Pointer/mouse manipulation of controls (keyboard-first v1).
- Multiple simultaneous panels (one open card at a time).
- Drag/slider handle rendering beyond a ratio bar (a real thumb is a later
  cosmetic).
- Remote/plugin-published control widgets; plugins keep sending typed
  `Control` data.

## Key decisions

1. **Enter opens the panel when controls or a primary action exist**, and the
   action becomes the first row — otherwise audio/media items, which have
   controls and often no action, would stay dead on Enter, and items with both
   would lose their launch path.
2. **One focus index** over the interactive rows; progress rows are excluded
   from the list rather than focusable.
3. **The panel is KSL**, reusing the existing kit, so a user scene can restyle
   or relocate it without touching Go.
