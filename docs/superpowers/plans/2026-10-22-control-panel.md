# Control Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every card with controls (audio volume/mute, media, network) an interactive panel that slides out when the card is activated, with an action row first, keyboard-navigable control rows, and mode-aware input routing — implemented entirely in default KSL plus minimal Go wiring.

**Architecture:** The launcher owns a `panel` state (open flag, ordered rows = action row + interactive controls, focus index, text edit buffer). The `View` interface the renderer reads gains panel accessors. The eval layer exposes `$panel.open/rows/focus/dirty` bindings; the default scene renders a panel under the selected card, one row per `$panel.rows` entry, with per-kind widgets. The input adapter routes keys to panel verbs when `PanelOpen()` is true, else to reel navigation.

**Tech Stack:** Go, gg renderer, KSL (existing interpreter), existing launcher/backend/input packages.

**Spec:** `docs/superpowers/specs/2026-10-22-control-display-interaction-design.md`

## Global Constraints

- The action row, when present, is row index 0 and is focusable; Enter runs the action.
- `progress` controls are never panel rows (read-only on the card only).
- Panel manipulation reuses the existing `Control`/`DebouncedControl` plumbing; no new backend surface.
- `Escape`/`Back` close the panel before popping a level.
- No new Go painters for control chrome unless a widget cannot be expressed in KSL; prefer KSL-only.
- Keep `$item.<id>` control lookup working (map → ordered slice change must preserve it).
- Tests follow existing per-package table/`{{` patterns in `*_test.go` beside source.

---

### Task 1: `backend` — PanelRow value type

The scene boundary currently loses control order and metadata (`Kind`, `Min/Max/Step`, `Options`, `Label`). Introduce a value type the launcher can hand the renderer.

**Files:**
- Modify: `internal/backend/backend.go` (after the `Control` struct)

**Interfaces:**
- Produces: `type PanelRow struct { Kind ControlKind; Label string; Action *Action; Control *Control }` — `Kind` is `"action"` for the action row; otherwise the control's kind. `Control` is non-nil for control rows, `Action` non-nil for the action row.

- [ ] **Step 1: Write the failing test in `internal/backend/backend_test.go`**
```go
func TestPanelRowKinds(t *testing.T) {
    r := PanelRow{Kind: ControlRange, Label: "Volume", Control: &Control{ID: "volume"}}
    if r.Kind != ControlRange { t.Fatalf("kind = %q", r.Kind) }
}
```

- [ ] **Step 2: Run test, verify it fails to compile** (type undefined)

- [ ] **Step 3: Add the struct** to `internal/backend/backend.go` after `Control`.

- [ ] **Step 4: Run test, verify it passes**
```bash
go test ./internal/backend/
```

- [ ] **Step 5: Commit**
```bash
git add internal/backend/backend.go internal/backend/backend_test.go
git commit -m "feat(backend): add PanelRow value type"
```

---

### Task 2: `eval` — ordered controls and `$panel` bindings

Two linked changes: `Item.Controls` becomes an ordered `[]ItemControl` (with richer fields) so row order and metadata survive, and the `Env` exposes `$panel.*`.

**Files:**
- Modify: `internal/ksl/eval/env.go`
- Modify: `internal/render/scene.go` (`itemFromBackend`, `state`)

**Interfaces:**
- Consumes: `backend.PanelRow` (Task 1).
- Produces:
  - `type Panel struct { Open bool; Rows []PanelRow; Focus int; Dirty bool }` in `eval`.
  - `State.Panel Panel` field.
  - `Env` lookups: `$panel.open` (Bool), `$panel.rows` (array), `$panel.focus` (Num), `$panel.dirty` (Bool).
  - `eval.ItemControl` gains `Kind`, `Label`, `Min/Max/Step`, `Options []string`; `Item.Control` becomes `[]ItemControl`.

- [ ] **Step 1: Write failing tests in `internal/ksl/eval/env_test.go`**
```go
func TestPanelBindings(t *testing.T) {
    st := &State{Panel: Panel{Open: true, Focus: 1, Dirty: false, Rows: []PanelRow{
        {Kind: "action", Label: "RUN"}, {Kind: "range", Label: "Volume"},
    }}}
    e := NewEnv(st, nil)
    for _, tc := range []struct{ expr string; want any }{
        {"$panel.open", true}, {"$panel.focus", 1},
        {"$panel.rows.len", 2}, {"$panel.dirty", false},
    } { /* eval and compare */ }
}
func TestOrderedControls(t *testing.T) {
    // $item.volume and $item.mute both resolve; order of item.Controls slice is preserved
}
```

- [ ] **Step 2: Run tests, verify they fail** (no `$panel` binding, `Controls` still map)

- [ ] **Step 3: Change `Item.Controls` to `[]ItemControl` and extend `ItemControl`** in `env.go`. Update `ItemVal`/`ControlVal` member access to find a control by ID over the slice.

- [ ] **Step 4: Add `Panel`, `PanelVal`, and `$panel.*` lookups** to `env.go`.

- [ ] **Step 5: Update `itemFromBackend` in `internal/render/scene.go`** to build the ordered `[]ItemControl` with metadata from `backend.Control`.

- [ ] **Step 6: Run the eval tests again**
```bash
go test ./internal/ksl/eval/
```

- [ ] **Step 7: Fix compile fallout** in `internal/render` (`cardpainter.go` `itemToBackend`, tests) until `go build ./...` and `go test ./internal/...` pass.

- [ ] **Step 8: Commit**
```bash
git add internal/ksl/eval internal/render
git commit -m "feat(ksl): ordered controls and \$panel bindings"
```

---

### Task 3: launcher — panel state and verbs

The launcher owns the panel: open/close, row assembly (action first, controls after, progress excluded), focus navigation, per-kind manipulation, and text editing.

**Files:**
- Modify: `internal/launcher/launcher.go`
- Modify: `internal/launcher/launcher_test.go`

**Interfaces:**
- Consumes: `backend.Control`, `backend.Action`, `backend.PanelRow` (Task 1), `backend.Change`, `k`/`k1` constants.
- Produces:
  - `type panelState struct { itemID backend.ResultID; rows []backend.PanelRow; focus int; edit string }`
  - `func (l *Launcher) PanelOpen() bool`
  - `func (l *Launcher) PanelRows() []backend.PanelRow`
  - `func (l *Launcher) PanelFocus() int`
  - `func (l *Launcher) PanelDirty() bool`
  - `func (l *Launcher) PanelNavigate(delta int)` — move focus, clamp.
  - `func (l *Launcher) PanelAdjust(delta int)` — `range` step by `Step` (coarse ×5 if `mod`), `choice` cycle `Options`; settle via `DebouncedControl`.
  - `func (l *Launcher) PanelActivate()` — action row → run action; `toggle` flip; `choice` commit; `text` toggle edit/commit.
  - `func (l *Launcher) PanelType(s string)` / `PanelBackspace()` — mutate `edit`.
  - Edge: `DebouncedControl` currently uses `l.Mode()` as source; keep (panel rows come from the current item's source).

- [ ] **Step 1: Write failing tests in `internal/launcher/launcher_test.go`** using `listBackend`.
```go
func TestEnterOpensPanelForControls(t *testing.T) {
    // item has Controls {volume range, mute toggle}, no actions
    // Enter() -> PanelOpen()==true, PanelRows() length 2
}
func TestEnterOpensPanelWithActionFirst(t *testing.T) {
    // item has Actions + Controls -> rows[0].Kind=="action"
}
func TestPanelNavigateClamps(t *testing.T) { /* focus stays in [0,len) */ }
func TestPanelAdjustStepsRange(t *testing.T) { /* DebouncedControl eventually calls backend.Control with stepped value */ }
func TestPanelActivateToggles(t *testing.T) { /* toggle flips + Control called */ }
func TestPanelActivateRunsAction(t *testing.T) { /* action row -> host action queued */ }
func TestEscapeClosesPanelFirst(t *testing.T) { /* panel open: Escape closes panel, does not dismiss */ }
```

- [ ] **Step 2: Run tests, verify they fail**

- [ ] **Step 3: Add `panelState` and the `Panel*` methods** to `launcher.go`.

- [ ] **Step 4: Modify `Enter()`** — if selected item has interactive controls or a primary action, assemble `panelState` (action row first if action exists, then non-progress controls), set `focus=0`, `changed()`; else existing behaviour. Note: `openFocusView` remains for `Expandable` items without controls.

- [ ] **Step 5: Modify `Escape()`/`Back()`** — if `panel != nil`, clear it and return before popping a level.

- [ ] **Step 6: Run tests**
```bash
go test ./internal/launcher/
go build ./...
```

- [ ] **Step 7: Commit**
```bash
git add internal/launcher
git commit -m "feat(launcher): panel state and control verbs"
```

---

### Task 4: `View` interface + renderer wiring

The renderer must read panel state from the launcher and place it into `eval.State.Panel`.

**Files:**
- Modify: `internal/render/render.go` (View interface)
- Modify: `internal/render/scene.go` (`state` builds `eval.State`)
- Modify: `internal/render/render_test.go` (`fakeView` implements new interface methods)

**Interfaces:**
- Consumes: launcher `PanelOpen/PanelRows/PanelFocus/PanelDirty`; `eval.Panel`.
- Produces: `View` gains `PanelOpen() bool`, `PanelRows() []backend.PanelRow`, `PanelFocus() int`, `PanelDirty() bool`.

- [ ] **Step 1: Add the four methods to the `View` interface** in `render.go`.

- [ ] **Step 2: Add stub methods to `fakeView`** in `render_test.go` so it still satisfies `View`.

- [ ] **Step 3: In `state()` (scene.go), populate `eval.State.Panel`** from `l.PanelOpen()`, `l.PanelFocus()`, `l.PanelDirty()`, mapping `l.PanelRows()` → `[]eval.PanelRow`.

- [ ] **Step 4: Write a render test** asserting a panel row paints accent pixels when `PanelOpen()` is true (extend `containsColor` checks for the open vs closed panel).

- [ ] **Step 5: Run tests**
```bash
go test ./internal/render/
go build ./...
```

- [ ] **Step 6: Commit**
```bash
git add internal/render
git commit -m "feat(render): expose panel state through View"
```

---

### Task 5: input — mode-aware routing

The keyboard adapter routes differently based on whether the panel is open.

**Files:**
- Modify: `internal/input/input.go`
- Add: `internal/input/input_test.go` (or extend existing)

**Interfaces:**
- Consumes: `launcher.PanelOpen()`, launcher panel verbs.
- Produces: when panel open — Up/Down → `PanelNavigate(∓1)`, Left/Right → `PanelAdjust(∓1)` (with `Modifiers` for coarse), Enter → `PanelActivate()`, typing → `PanelType`, Backspace → `PanelBackspace`, Escape → `Escape()` (closes panel). When closed, current routing.

- [ ] **Step 1: Write failing tests** — drive `onKeyPress`/`onTextInput` against the launcher with a panel open and assert the right launcher methods are reached (use a fake launcher or observe state).

- [ ] **Step 2: Implement the branch** in `onKeyPress` and `onTextInput` keyed on `a.l.PanelOpen()`.

- [ ] **Step 3: Run tests**
```bash
go test ./internal/input/ ./internal/launcher/
go build ./...
```

- [ ] **Step 4: Commit**
```bash
git add internal/input
git commit -m "feat(input): route keys to panel verbs when open"
```

---

### Task 6: default scene — the panel in KSL

Author the panel in `internal/render/default.ksl` using `def` components and the existing primitive kit; no new Go painters unless a widget can't be expressed.

**Files:**
- Modify: `internal/render/default.ksl`

**Interfaces:**
- Consumes: `$panel.open/rows/focus/dirty`, `$active`, `$item`, `$reveal.angle`, `$reveal.cardShear`.
- Produces: a panel `def` rendered under the selected card when `$panel.open && $active.id == $item.id`, sliding out along the reel's cross-axis (inherits `Group#content` rotation), each row a `when`-gated widget: action row = `Pill` labeled `$row.label`; range = label + `Meter` (ratio bound to `$row.value`); toggle = `Pill`/`Box` fill by `$row.on`; choice = `Text` + chevrons; text = `Box` + `Text` + `Cursor` (blink when `$panel.dirty`).

- [ ] **Step 1: Add row-binding support** — the `List` template must iterate `$panel.rows`; map each row to `eval.Item` schema in `itemFromBackend`/state so `$item` binds per row, OR expose `$row` via a scoped env. Choose the minimal path the interpreter supports; if `List` `items:` cannot yet bind to `$panel.rows`, extend `listItems` to accept an array value.

- [ ] **Step 2: Add `def` components** to `default.ksl` for each row kind.

- [ ] **Step 3: Add the panel to the card template** with the `when` guard and an open transition (translate/opacity).

- [ ] **Step 4: Verify the default scene parses and validates**
```bash
go test ./internal/ksl/ ./internal/render/
```

- [ ] **Step 5: Commit**
```bash
git add internal/render/default.ksl
git commit -m "feat(ksl): control panel in default scene"
```

---

### Task 7: integration verification

Confirm the whole path end-to-end and update any docs.

**Files:**
- Run: full suite.
- Modify: `docs/superpowers/specs/2026-10-22-control-display-interaction-design.md` if implementation diverged.

- [ ] **Step 1: Run the full test suite**
```bash
go test ./...
```

- [ ] **Step 2: Build**
```bash
go build ./...
```

- [ ] **Step 3: Manual smoke** — launch with the audio backend, select a sink, Enter opens the panel with action row (if any) + volume/mute, navigate and adjust, Escape returns to the reel.

- [ ] **Step 4: Commit any final adjustments**
```bash
git add -A
git commit -m "chore: control panel integration verification"
```
