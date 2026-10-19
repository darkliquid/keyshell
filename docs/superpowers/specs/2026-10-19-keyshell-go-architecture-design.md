# keyshell Go rewrite — core architecture

Date: 2026-10-19
Branch: `go-rewrite` (in the `keyshell-go` worktree; fresh history, no parent commits)
Status: design, approved section by section; not yet implemented

## Scope

The core architecture of the Go rewrite of keyshell: the modules, the one seam
every result source crosses, the plugin protocol and lifecycle, the result and
control schema, the render and input pipeline, configuration, and how it will be
tested. It is the thing implementation is planned against.

Not in scope: the visual design itself (it already exists and is being
preserved), the QML implementation (it stays on `main` as the behavioural and
visual reference until parity), and any specific backend's protocol details.

## Why Go

Decided by the research in `~/.local/share/crush/research/keyshell-go-reimplementation`:

- `gogpu/gg` already has the affine transform API the design is built on
  (`Shear`, `ShearAbout`, `Rotate`, `Transform`, `TextPath`) over a GPU backend,
  where iced has no transform API at all.
- The stack is pure Go, cgo-free: one binary, no Qt, no GTK, no wgpu-native.
- The single blocker — layer-shell surfaces — is solved by a ~600-line patch to
  gogpu (branch `darkliquid/gogpu@feat/wayland-layer-shell`, PR drafted),
  carried as a `replace` until it lands upstream.

## Verified foundations

Each of these was measured on this machine (niri, Wayland, Vulkan, RTX 3060)
against the layer-shell fork of gogpu, and each constrains the design:

| Finding | Evidence | Constraint it imposes |
| --- | --- | --- |
| A layer surface is created, configured and carries Vulkan frames | `~/Projects/gogpu-layershell-spike/FINDINGS.md` | The overlay is a layer surface; no window manager involvement |
| On-demand focus is granted on show without a click; text, launcher keys and logical-pointer events arrive; clicking outside releases focus while the surface stays mapped | `spikes/layershell-input`, research note `spike-s2-layer-input.md` | Dismiss-on-focus-loss is implementable, and the surface can stay alive between summons |
| `gg` renders the display face (Red Hat Display Black Italic) indistinguishably from Qt at display sizes, including hard offset shadows, shear, text-as-path and strokes; 11px is the one soft spot | `spikes/text`, note `spike-s3-typography.md` | The visual identity is portable; the UI draws with gg |
| Layer surfaces composite per-pixel alpha, but `NewTextureFromRGBA` applies alpha twice to premultiplied data | `spikes/transparency`, note `spike-s4-transparency.md` | Unpremultiply before upload (or use a premultiplied-aware path); scrims and translucent cards are available |

## Decisions

1. **Go-only plugin interface**, typed, with **multiple instances** per plugin
   binary. Non-Go tools are wrapped by an adapter plugin, one instance per
   wrapped program. Rejected: an any-language wire protocol as the primary
   interface (it was the fallback the user explicitly declined in favour of
   Go-only plus adapters).
2. **Transport is a line-delimited JSON protocol over stdio** with a typed Go
   SDK on top, not hashicorp/go-plugin. Rationale: the project's premise is one
   pure-Go binary; the adapter-plugin pattern is only cheap when the wire format
   is the plugin's own language; the existing protocol and its reference plugins
   are already line-delimited JSON; and instance supervision (load modes, idle
   teardown, restart backoff) has to be written host-side either way. Rejected:
   go-plugin + gRPC (typed streaming and mTLS for free, at the cost of protobuf,
   gRPC and codegen in every plugin, plus a wire format that cannot be inspected
   with `cat`).
3. **Plugins own their domain; the host owns environment effects.** The host
   exposes a capability API (launch app, open URL, copy text, run in terminal,
   focus window, notify) and never learns a domain operation. Rejected: the host
   executing everything (couples the host to every domain) and full access
   behind per-plugin permission manifests (largest trust surface, most machinery
   before the first plugin works).
4. **Warmth is config-driven**: `hot` / `lazy` / `auto` per instance, with
   `auto` resolving to hot when the instance answers the merged query and lazy
   when it is reachable only by trigger or Tab. Instances are long-lived with
   `init`/`ready`/`teardown` hooks.
5. **Controls are optimistic with a settle window.** The widget echoes locally,
   the host coalesces changes and sends the value once it stops moving for
   `controlSettleMs` (default 80ms); plugin state reconciles last-write-wins.
   Rejected:
   plugin-authoritative values (laggy on drag) and pure optimism without
   coalescing (one call per animation frame).
6. **Matching is ours**: port the tier model and frecency (from `rank.js`, with
   its fixtures), behind a `Matcher` seam so a stronger scorer (fzf's algorithm)
   can replace the fuzzy tier later without touching ranking. Ranking applies to
   root levels; scoped levels keep the plugin's order (see Navigation).
7. **Input is gogpu's, behind an `Input` seam** (key, text, preedit, focus,
   pointer). IME is deferred to a future backend; gogpu issue #331 is the known
   blocker.
8. **Items may contain items.** The launcher navigates a tree of scoped queries
   rather than a flat result list, so menus, device lists and nested options are
   data (a sub-level) instead of one widget per backend; ranking applies at the
   root, and scoped levels keep the plugin's order. Rejected: per-backend
   console widgets (the QML status quo) and a fixed two-level model.
9. **Interfaces first, then the process spine** (see Delivery order): the
   protocol commitment comes after the interface has been exercised by real
   backends, so the seam is designed against use rather than speculation.

## Modules and the seam

```
cmd/keyshell                  the binary; also runs in plugin mode: `keyshell plugin <id>`
internal/launcher             state machine: navigation stack, query, mode, selection, dispatch
internal/ui                   the visual layer: band, reel, cards, control widgets (gg drawing)
internal/render               per-output layer surfaces, texture upload, present, hide/show
internal/input                Input seam: key, text, preedit, focus, pointer (+ gogpu adapter)
internal/capability           host environment effects: launch, open-url, copy, terminal, focus, notify
internal/rank                 tier model + frecency behind a Matcher seam
internal/calc                 expression parser and formatter (ported from calc.js)
internal/icon                 icon-theme lookup (freedesktop search order, cached)
internal/state                persisted stores (frecency, clipboard) with atomic writes
internal/plugin               the seam: Backend interface, supervision, protocol, adapters
internal/plugin/sdk           what plugin authors import (Info, Item, Control, Serve, stream helpers)
internal/config               host config + XDG paths
internal/ipc                  control socket and verbs
```

The interface everything programs against:

```go
type Backend interface {
    Info() Info                                        // id, name, badge, trigger, control kinds, hot hint, stable ids
    Query(ctx context.Context, q Query) (Stream, error) // q = {Text, Limit, Merged}
    Control(ctx context.Context, c Change) error         // coalesced value change
    Close() error                                       // teardown hook
}

type Stream interface {
    Next() (Item, bool, error) // item, more, error
    Close() error
}
```

Design notes:

- **Two adapters at the seam**: `inproc` (a Go implementation linked in) and
  `proc` (a plugin process speaking the protocol). That is what makes the seam
  real rather than hypothetical, and it lets tests and low-latency first-party
  backends skip process boundaries without the launcher knowing.
- **The deletion test**: deleting `internal/plugin` would force every backend to
  spawn, frame, supervise and negotiate for itself, so the module earns its
  keep. `internal/rank`, `internal/calc` and `internal/icon` are pure and deep;
  `internal/capability` is deliberately shallow (a short, auditable list).
- **The interface is the test surface**: a backend is tested with a fake
  `Stream`; the protocol is tested with a fake plugin process; the UI is tested
  by rendering and comparing PNGs.
- `internal/plugin/sdk` exists so plugin authors depend on a small, stable
  package rather than the host's internals.

## Plugin protocol

Wire: one JSON object per line over stdio, UTF-8, 1 MiB line cap. Malformed or
oversized lines are counted and dropped; they are never fatal. This is the
successor of the existing keyshell plugin protocol, so the current Python
reference plugins remain a useful comparison.

Handshake:

```
host  → KEYSHELL_PLUGIN=1, KEYSHELL_PROTOCOL=1, KEYSHELL_INSTANCE=<id> in the environment
plugin→ {"hello":{"protocol":1,"id":"audio","name":"Audio","badge":"AUDIO","trigger":"!",
                  "controls":["range","toggle"],"hot":true,"stable_ids":true}}
host  → {"init":{"instance":"sink-1","config":{...}}}
plugin→ {"ready":{}}                      // services are up; the mode stops showing "loading"
```

Queries (one in flight per instance, newest wins):

```
host  → {"query":{"id":17,"text":"vol","limit":60,"merged":false}}   // scope optional, see Navigation
plugin→ {"item":{...}}
plugin→ {"item":{...}}
plugin→ {"done":{"id":17}}                 // or {"error":{"id":17,"message":"..."}}
host  → {"cancel":{"id":17}}               // when a newer query supersedes it
```

Controls and state:

```
host  → {"control":{"item":"sink-0","control":"volume","value":0.42}}   // after the settle window
plugin→ {"state":{"item":"sink-0","controls":{"volume":0.42,"mute":false}}}
```

Actions:

- Items carry a declarative action list. Capability actions (`open-url`,
  `copy`, `exec`, `exec-terminal`, `set-query`) are executed by the host.
- Domain actions (`{"type":"plugin","action":"connect"}`) are forwarded to the
  producing plugin as `{"invoke":{"item":"...","action":"connect"}}`.
- The destructive gate (hold for `powerHoldDurationMs`, or double-tap) is
  applied by the host on the primary action in both paths, so a plugin cannot
  bypass it.

Lifecycle:

- `hot` instances start with the session; `lazy` on first use; `auto` resolves
  by the rule in Decision 4.
- Idle teardown after `pluginIdleTimeoutMs` (default 5 minutes; `0` disables
  it), except for instances marked `noreap` — see the clipboard note below.
- A crashed instance restarts with backoff (1s, 2s, 4s, 8s; at most 5 attempts
  in 60s); past that the mode reports itself unavailable instead of hanging.
- Health: `{"ping":{}}` / `{"pong":{"ready":true}}` every `pluginPingIntervalMs`
  (default 10s); three unanswered pings restart the instance.
- Host exit sends `SIGTERM`, then `SIGKILL` after `pluginExitGraceMs`
  (default 2s).
- Internal behaviour: a slow `init` never blocks a summon — the summon shows the
  mode as loading and results stream in when ready.

Versioning: `protocol` is a single int, currently `1`; the host refuses
anything else and surfaces that in the mode's status line rather than failing
mysteriously. The SDK pins the same constant, so a plugin built against a newer
protocol fails loudly. Richer negotiation is deliberately postponed until a
second protocol version actually exists.

First-party plugins: the same binary in plugin mode (`keyshell plugin audio`),
so shipping stays one artifact while the architected boundary stays a process.
Third-party plugins import `internal/plugin/sdk` and call `sdk.Serve(...)`.

One instance is architecturally special: **clipboard capture must run
continuously**, so the clipboard plugin is `hot` and marked `noreap` (exempt from
idle teardown). That is the same reason the current implementation keeps
`wl-paste --watch` alive for the shell's whole lifetime: history cannot be
reconstructed after the fact. Its pause and clear controls are ordinary
`Toggle`/domain actions in the schema.

## Result and control schema

```go
type Item struct {
    ID          string
    Title       string
    Subtitle    string
    Glyph       string     // emoji or icon name
    Badge       string
    Destructive bool
    Actions     []Action   // ordered; first is primary (may be empty)
    Controls    []Control  // editable values, shown on the card and in the focus view
    HasChildren bool       // Enter descends into a scoped level
    Expandable  bool       // Enter opens the focus view for this item's controls
    Preview     any        // reserved; deferred
}

type Control struct {
    ID     string
    Kind   Kind            // Range | Toggle | Choice | Text
    Label  string
    Value  Value           // float64 | bool | string
    Min, Max, Step float64 // Range only
    Options []Option       // Choice only
}
```

- `Kind` is semantic, never stylistic: `Range` renders as the segmented block
  meter, `Toggle` as a chunky switch, `Choice` as a row of sigil chips, `Text`
  as an inline field. An item may carry several controls, and may also contain
  items (see Navigation and sub-queries), so today's eight hand-written console
  views collapse into four widgets plus two generic renderings — a child reel
  and a focus view. A new backend adds no UI code.
- Control changes are never destructive-gated; only actions are.
- `Info.StableIDs` decides whether an instance's items feed frecency (the
  documented lesson: pids are not stable, codepoints are).

## Navigation and sub-queries

An item may contain items: a tray applet contains its menu, a Bluetooth adapter
contains its devices, a power card contains its profiles, an equaliser contains
its bands. The launcher therefore navigates a **tree**: any item, **at any
depth**, may contain items, and every level is produced by the same query
mechanism — there is no second API for menus, and no depth at which behaviour
changes.

Model:

- The launcher keeps a **navigation stack** per summon, one entry per level:
  `{scope, items, selection, text, status}`. The root level has an empty scope.
- `Item.HasChildren` says a level can be entered from this item; `Item.Expandable`
  says the item has controls worth showing large (today's console views).

Protocol: a scoped query is the ordinary query with an explicit scope, so
"list the children" and "filter the children" are one code path in the plugin:

```
host  → {"query":{"id":19,"text":"","limit":60,"merged":false,
                  "scope":{"item":"applet:nm","path":["applet:nm"]}}}
plugin→ {"item":{...}} … {"done":{"id":19}}
```

- `path` is the scope chain from the root, so a plugin resolves a deep node from
  the request alone rather than relying on host-tracked handles.
- A scope the plugin no longer recognises (a menu that has since closed) answers
  `{"error":{"id":19,"message":"stale scope"}}`; the host pops that level and
  shows the message on the parent card.
- Ordering differs by level, deliberately: **root levels are ranked by the host**
  (tiers + frecency), **scoped levels preserve the plugin's order** and are only
  filtered when the user types. An applet's menu order is meaning, not a ranking.
- Depth is capped by `maxNavDepth` (default 8); a scoped level that returns
  nothing shows its status rather than becoming a dead end.

Keys — this extends the existing map rather than adding bindings:

| Key | Effect |
| --- | --- |
| `Enter` | The item's primary action if it has one; otherwise the focus view if `Expandable`; otherwise descend if `HasChildren` |
| `Right` / `Left` | Descend / pop explicitly, for an item that has both an action and children |
| `Backspace` | Pop a level when the query is empty (as it already exits submenus today) |
| `Escape` | Pop a level, or dismiss at the root |
| typing | Filters the current level; at the root it is the merged search |

The destructive gate still applies to the primary action wherever it is invoked,
at any depth.

## Render and input pipeline

- One layer surface per output, overlay layer, exclusive zone `-1`. Shown:
  keyboard `on_demand`. Hidden: keyboard `none`, empty input region, fully
  transparent — the surface stays mapped so summoning is instant rather than a
  surface-plus-swapchain rebuild. Fallback if a compositor misbehaves:
  destroy/recreate.
- gg draws the entire composition into one RGBA buffer per change; the host
  unpremultiplies it before `NewTextureFromRGBA` (the S4 defect) and presents
  with `DrawTexture`. Rendering is demand-driven, matching gogpu's
  request-driven draw callback and preserving zero-CPU-when-idle.
- Animation: `Reveal` resampling ports directly; a small easing/tween helper;
  the existing sub-100ms durations become config values.
- Input: `Input` seam events drive the launcher exactly as today — printable
  characters into the query or prefix switch, Tab cycling, arrows across the
  reel, Enter gated when destructive and routing through the navigation rules
  above, Escape dismiss, Space quick-action. Text arrives through the text
  event, not key codes.
- External control: `$XDG_RUNTIME_DIR/keyshell.sock` with line-delimited verbs.
  The CLI is the same binary, so compositor keybinds are `keyshell toggle`.
  Step 1 ships `toggle` and `openMode`; the remaining verbs from today's 27
  IPC functions are carried over as the backends they belong to arrive (step 4),
  and the verb list is generated from the launcher's exposed capabilities so it
  cannot drift.

## Config, discovery, trust

- Plugins are **self-describing**: identity, badge, trigger, control kinds, hot
  hint and stable-id flag come from the `hello` handshake.
- **Instances are declared in config**:
  `$XDG_CONFIG_HOME/keyshell/config.toml`, with `[[instance]]` tables (`id`,
  `command`, `args`, `env`, `mode`, `trigger` override, `enabled`, `noreap`)
  plus host tunables. Existing `plugin.json` manifests import as single-instance
  entries. **TOML is the format** (one small dependency, and comments matter in a
  file users edit by hand).
- **Trust**: a plugin is a program the user installed and runs with their
  privileges. Host-executed capabilities keep the item path auditable. No
  sandbox is offered and the documentation must not imply one.

## Testing

- **Pure unit tests**: rank tiers (ported fixtures from `rank.js`), calculator
  (ported from `calc.js`), icon lookup, config parsing, frecency decay.
- **Protocol tests** against fake plugins: well-behaved, slow to init, flooding,
  dying mid-stream, wrong protocol version, oversized line, never answering a
  ping.
- **Control-loop tests**: settle coalescing, last-write-wins reconciliation,
  destructive gating on both action paths.
- **Navigation tests**: descend/pop across several levels, a stale scope popping
  the level with a status, the depth cap, ordering rules (ranked at the root,
  plugin order when scoped), and a filter applied inside a level without
  reordering it.
- **Golden-image tests** for the UI vocabulary: render each widget state to PNG
  and compare, so the visual identity is regression-tested.
- **Composited-surface verifier**: the S4 screenshot checker (`spikes/transparency`)
  extended as an opt-in mise task that runs a real session check (alpha, focus,
  surface mapping).

## Delivery order

1. Interfaces, in-process first-party backends (apps, files, windows, calc, run),
   UI shell, ranking and frecency → a working overlay.
2. Plugin host, protocol and SDK, with one backend (audio) moved to a process;
   the in-process adapter stays for tests and latency-sensitive backends.
3. Remaining backends ported one at a time.
4. IPC verbs and CLI, clipboard history, tray, and the rest.

Each step is usable on its own, and step 1 exercises every interface the later
steps depend on.

**Each step is its own implementation plan.** This document is the architecture
they share, not a single plan: the next artefact is a plan for step 1 only
(interfaces, in-process backends, UI shell, ranking), and steps 2-4 get their own
once step 1 has exercised the interfaces against real behaviour.

## Open items

- Hide/show focus semantics: does flipping `keyboard_interactivity` on a mapped
  surface release focus on niri, and does a hidden-but-mapped surface stay cheap?
  (Small spike before this is locked in.)
- Multi-output surface lifecycle and per-output scale.
- Whether `hot` backends stay in-process permanently rather than spawning.
- The exact descend/pop key map: `Enter` routing as specified, with `Right`/`Left`
  as explicit descend/pop. Worth confirming against muscle memory once the
  overlay renders, since the existing map uses `Left`/`Right` to move along the
  reel (per-level, so it still works, but it deserves a second look).
- Whether gogpu gains a premultiplied-aware upload or `ggcanvas` already handles
  it (then the unpremultiply step disappears).

## Appendix: rejected alternatives

- **hashicorp/go-plugin + gRPC**: typed streaming, health checks, mTLS and
  crash/restart for free, at the cost of protobuf + gRPC + generated stubs in
  every plugin, a wire format that cannot be inspected directly, and a large
  dependency tree in a project whose premise is one pure-Go binary. Revisit only
  if plugin authors ask for codegen or the protocol grows to need schema tooling.
- **Host executes every side effect**: uniform and auditable, but the host must
  then learn every domain operation (set volume, connect wifi, pair bluetooth) —
  exactly the coupling the plugin split exists to avoid.
- **Per-plugin permission manifests**: maximum flexibility, but the trust model,
  the enforcement machinery and the documentation burden all land before the
  first plugin can work.
- **fzf's algorithm as the fuzzy scorer in v1**: a good scorer behind an
  unstable internal API; the tier model is product behaviour we already own and
  test, so it goes behind a seam and the scorer can follow.

## Appendix: QML → Go mapping

| Today (QML/JS/Python) | Go |
| --- | --- |
| `rank.js` + `Frecency.qml` | `internal/rank` + `internal/state` |
| `calc.js` | `internal/calc` (used by the calc plugin) |
| `PluginRegistry.qml`, `PluginBackend.qml`, `plugin-proto.js` | `internal/plugin` + `internal/plugin/sdk` |
| `BackendManager.qml`, `SearchAggregator.qml`, `Launcher.qml` | `internal/launcher` |
| `Reveal.qml`, `Theme.qml`, `Config.qml` | `internal/ui` + `internal/config` |
| `components/*` (band, card, reel, field, delegates, consoles) | `internal/ui` (four schema-driven widgets, plus the child reel and focus view, replace eight consoles) |
| `LauncherWindow.qml` | `internal/render` + `internal/input` |
| `shell.qml` `IpcHandler` | `internal/ipc` |
| twelve `*Backend.qml` files | first-party plugins |
| `IconImage` / icon-theme lookup | `internal/icon` |
| `clipboard-watch.py` | `internal/state` + a clipboard watcher (helper process initially), always-warm instance |