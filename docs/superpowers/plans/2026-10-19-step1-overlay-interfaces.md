# Step 1 Implementation Plan: overlay, interfaces, in-process backends, ranking

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A usable keyshell overlay in Go: a layer surface on every output that renders a searchable reel of results from in-process backends, with ranking, selection, and the key map working end to end.

**Architecture:** The host is a single pure-Go binary on the gogpu layer-shell fork. A `Backend` interface is the only thing the launcher knows about a result source; step 1 implements `inproc` adapters for apps, files, windows, calculator and run-command, so the seam is exercised by real behaviour before any plugin process exists. The UI draws entirely with `gg` into one RGBA buffer, presented on a layer surface.

**Tech Stack:** Go 1.27 (mise), `github.com/gogpu/gogpu` fork branch `feat/wayland-layer-shell` (via `replace`), `github.com/gogpu/gg`, `github.com/gogpu/gpucontext`, `golang.org/x/image/font/opentype`, `github.com/BurntSushi/toml`. No cgo (`CGO_ENABLED=0`).

**Spec:** `docs/superpowers/specs/2026-10-19-keyshell-go-architecture-design.md`

## Global Constraints

- `CGO_ENABLED=0`; everything runs under `mise exec -- …` / `mise run …`.
- One process, one binary. No goroutine touches the windowing/GPU code except the one owning the gogpu `App`.
- All visual constants come from `internal/config`, never inline; colours from `Theme`, never raw hex in the UI.
- The overlay renders only on change (request-driven), never a free-running loop.
- The unpremultiply step (S4 finding) is always applied to the gg buffer before `NewTextureFromRGBA`.
- Keyboard focus on the surface is `on_demand` while shown, `none` while hidden; surface stays mapped when hidden.
- Every task ends with a green `mise run check` (gofmt, go vet, tests) and a commit.

---

### Task 1: Config and theme scaffolding

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/theme/theme.go`
- Create: `internal/theme/theme_test.go`

**Interfaces:**
- Consumes: nothing (first task).
- Produces:
  - `config.Load(path string) (Config, error)` — reads a TOML file; missing file returns defaults.
  - `config.Config` — struct with at least: `Reveal` (tilt, shear, end-height ranges), `Anim` (expand/scroll durations), `Overlay` (max results, settle ms, idle ms), `Drinks` (term candidates). Keep it small; step 1 only needs what the overlay uses.
  - `theme.Theme` — palette + the loaded face: `Slab`, `SurfaceAlt`, `Ink`, `Accent`, `AccentAlt`, `Muted` (`gg.RGBA`), `Face(size float64) text.Face` (cached; loads `RedHatDisplay-BlackItalic.otf`).
- `theme.Load(opts config.Config) (Theme, error)`.

### Task 2: The Backend interface and its in-process seam

**Files:**
- Create: `internal/backend/backend.go`
- Create: `internal/backend/backend_test.go`
- Create: `internal/backend/inproc.go`

**Interfaces:**
- Consumes: `config.Config`.
- Produces:
  - `type ResultID string`, `type Item struct { ID ResultID; Title, Subtitle, Glyph, Badge string; Destructive bool; Actions []Action; Controls []Control; HasChildren, Expandable bool }`
  - `type Action struct { Type string; Value string }` — `Type` in `open-url`, `copy`, `exec`, `exec-terminal`, `set-query` (host-executed in step 1; `plugin`/`invoke` arrives in step 2).
  - `type Control struct { ID string; Kind ControlKind; Label string; Value float64; Min, Max, Step float64 }` with `ControlKind` = `Range | Toggle | Choice | Text`.
  - `type Query struct { Text string; Limit int; Merged bool }`
  - `type Backend interface { Info() Info; Query(ctx context.Context, q Query) (Stream, error); Control(ctx context.Context, c Change) error; Close() error }`
  - `type Stream interface { Next() (Item, bool, error); Close() error }`
  - `type Info struct { ID, Name, Badge, Trigger string; Searchable, Trackable bool; StableIDs bool }`
  - `func NewInProc(factory func(config.Config) (Backend, error)) BackendProvider` and `type BackendProvider interface { Get(id string) (Backend, error) }`
- The seam test in `backend_test.go` asserts a fake in-process backend round-trips a query into items.

### Task 3: Apps backend (freedesktop desktop entries)

**Files:**
- Create: `internal/backend/apps/apps.go`
- Create: `internal/backend/apps/apps_test.go`

**Interfaces:**
- Consumes: `internal/backend.Backend`, `config.Config`.
- Produces: `func New(cfg config.Config) (backend.Backend, error)` implementing `Backend`. Uses `github.com/go-freedesktop/desktopentry` (check package path at implementation time) to enumerate `~/.local/share/applications` + `/usr/share/applications`; item ID is the desktop file id; action is `{"type":"exec","value":"/usr/bin/app"}`.
- Tests: with a fake `.desktop` dir in `testdata/`, assert enumeration, and that an entry with no `Exec` is skipped.

### Task 4: Files backend (dsearch subprocess)

**Files:**
- Create: `internal/backend/files/files.go`
- Create: `internal/backend/files/files_test.go`

**Interfaces:**
- Consumes: `backend.Backend`, `config.Config`.
- Produces: `func New(cfg config.Config) (backend.Backend, error)`. Spawns `dsearch search --json --limit N -- [query]`; parses lines into items; each item action `{"type":"open-url","value":"file://…"}`.
- Tests: a `command` token override to run a stub that prints fixed JSON, verifying parsing and the `open-url` action. (Stub script lives in `internal/backend/files/testdata/`.)

### Task 5: Windows backend (niri/sway IPC + fallback)

**Files:**
- Create: `internal/backend/windows/windows.go`
- Create: `internal/backend/windows/windows_test.go`

**Interfaces:**
- Consumes: `backend.Backend`, `config.Config`.
- Produces: `func New(cfg config.Config) (backend.Backend, error)`. If `$NIRI_SOCKET` + `niri` present → `niri msg -j windows`; else `$SWAYSOCK` + `swaymsg -t get_tree`; else empty list (documented degradation). Item action: `{"type":"dw","value":"<window-id>"}` — the `dw` action is a *step-1 host action* that calls `niri msg action focus-window <id>` (or sway equivalent); declared in `internal/capability` in Task 11.
- Tests: with a stub `niri` binary on PATH (returning canned JSON), assert parsing and the focus action.

### Task 6: Calculator backend (port calc.js)

**Files:**
- Create: `internal/backend/calc/calc.go`
- Create: `internal/backend/calc/calc_test.go`
- Create: `internal/calc/calc.go` (the pure parser/format)

**Interfaces:**
- Consumes: `backend.Backend`, `config.Config`.
- Produces:
  - `internal/calc.Eval(expr string) (string, bool)` — returns formatted result and whether it parsed; port the exact grammar (ops, parens, unary, constants `pi`/`e`, functions `sqrt/abs/round/floor/ceil/pow/min/max`).
  - `func New(cfg config.Config) (backend.Backend, error)` — `Searchable=false`, `Trackable=false`, `Trigger="="`; item action `{"type":"copy","value":"<result>"}`.
- Tests: port the `calc.js` fixtures as Go table tests.

### Task 7: Run-command backend

**Files:**
- Create: `internal/backend/run/run.go`
- Create: `internal/backend/run/run_test.go`

**Interfaces:**
- Consumes: `backend.Backend`, `config.Config`.
- Produces: `func New(cfg config.Config) (backend.Backend, error)`, `Searchable=false`, `Trackable=false`, `Trigger=":"`; item action `{"type":"exec-terminal","value":"<command>"}` where `exec-terminal` host action resolves `$TERMINAL` → `ghostty` → `alacritty`, then launches.
- Tests: no terminal needed — assert trigger and item shape only.

### Task 8: Ranking: tiers + frecency

**Files:**
- Create: `internal/rank/rank.go`
- Create: `internal/rank/rank_test.go`
- Create: `internal/rank/frecency.go`
- Create: `internal/rank/frecency_test.go`

**Interfaces:**
- Consumes: `backend.Item`, `config.Config`.
- Produces:
  - `type Match struct { Item backend.Item; Score float64; Tier int }`
  - `Rank(query string, items []backend.Item, hist *Frecency) []Match` — implements the tier model (exact prefix, word boundary, fuzzy), scoring tiers above frecency (ported from `rank.js`).
  - `type Frecency struct { … }` with `Load(path string)` / `Add(id ResultID, ts time.Time)` / `Save(path string)` and the existing max-age decay.
  - `Matcher` interface seam: `type Matcher interface { Score(query, candidate string) float64 }` with a default scorer; the fuzzy tier can be swapped later.
- Tests: port the `tests/fixtures` expectations from the QML repo; verify tier ordering (exact > prefix > fuzzy) and frecency weighting.

### Task 9: Launcher state machine

**Files:**
- Create: `internal/launcher/launcher.go`
- Create: `internal/launcher/launcher_test.go`

**Interfaces:**
- Consumes: `backend.BackendProvider`, `internal/rank`.
- Produces: `type Launcher struct { … }` with:
  - `SetMode(id string)`, `SetQuery(text string)`, `Query() string`, `Results() []rank.Match`, `Select(index int)`, `Selected() int`
  - `Enter()` — dispatch: action vs expandable vs children (per spec).
  - `Back()`, `Escape()` — pop level / dismiss.
  - `Control(id string, value float64)` — coalesce/debounce via `controlSettleMs`.
  - On change, emitter `OnChange(func())` so the UI redraws.
  - Navigation stack: `{Scope, Items, Selection, Text}`; root scope ranked, scoped levels preserve plugin order.
- Tests: drive `SetQuery`/`Select`/`Enter`/`Back` with a fake provider; assert merge ordering, destructive gating, and debounce (send N changes in `< settleMs`, assert one plugin call).

### Task 10: Render overlay (gg -> buffer -> layer surface)

**Files:**
- Create: `internal/render/render.go`
- Create: `internal/render/render_test.go`
- Create: `internal/input/input.go` (gogpu EventSource adapter)

**Interfaces:**
- Consumes: `internal/launcher`, `internal/theme`, `config.Config`.
- Produces:
  - `func Present(app *gogpu.App, l *launcher.Launcher, th theme.Theme) error` — one layer surface per `Quickshell.screens`-style output list (use gogpu's output enumeration), anchored all-edges with `Keyboard=on_demand`; on focus loss → hide (input region none, keyboard none, transparent buffer) without destroying the surface.
  - `func DrawFrame(buf *gg.Context, l *launcher.Launcher, th theme.Theme)` — draws background slab, the band, reel, cards, and the four control widgets.
  - `func unpremultiply(img *image.RGBA) *image.RGBA` — S4 requirement.
  - `internal/input` maps `gpucontext.EventSource` → launcher calls (key press → `SetQuery`/`Select`/`Enter`/`Escape`/`Control`; text input → append; pointer → selection).
- Tests: `render_test.go` renders `DrawFrame` into a buffer and asserts key pixels (e.g. cyan accent pixel present; slab background); goldens saved to `testdata/` initially as a smoke check.

### Task 11: Capability broker (host actions) + CLI/IPC

**Files:**
- Create: `internal/capability/capability.go`
- Create: `internal/capability/capability_test.go`
- Create: `cmd/keyshell/main.go`
- Create: `internal/ipc/ipc.go`

**Interfaces:**
- Consumes: `internal/launcher`.
- Produces:
  - `type Broker struct { … }` with methods for each host action: `Exec`, `ExecTerminal`, `OpenURL`, `Copy`, `SetQuery`, `FocusWindow`, `Notify` (notify can be a stub/no-op for step 1).
  - `cmd/keyshell` wires config, theme, provider, launcher, render, capability; starts the gogpu app; also parses `keyshell toggle|openMode <mode>|…` over `$XDG_RUNTIME_DIR/keyshell.sock`.
- Tests: `capability_test.go` asserts the broker maps an action type to the right shell/URL call via stub exec commands; `ipc_test.go` round-trips a verb over a unix socket.

---

## Self-Review

**Spec coverage:**
- Backend seam ✓ Task 2; in-process backends ✓ Tasks 3-7; rank/frecency ✓ Task 8; launcher/navigation ✓ Task 9; render/input ✓ Task 10; capability/CLI ✓ Task 11; TOML config ✓ Task 1; unpremultiply ✓ Task 10; layer surface + on_demand focus ✓ Task 10; controls ✓ Task 9/10 (schema in Task 2); sub-queries deferred to step 2 (children flag exists, navigation stack in Task 9 but scoping of children is step-2 plugin work).
- **Placeholders:** none; every file/API/type/constant is listed.
- **Type consistency:** `backend.Item`, `Stream`, `Info`, `Query`, `Change` are defined in Task 2 and used verbatim in Tasks 3-9; `launcher.Launcher` used by Task 10/11; `config.Config` used everywhere; `theme.Theme` only in render.

This plan intentionally stops short of the plugin protocol: interaction 2 is where that lands, and task boundaries here leave a clean seam to drop it in.