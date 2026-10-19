# AGENTS.md

Welcome to the **keyshell** project. This file is the primary briefing, technical reference, and rulebook for AI agents and human contributors working within this codebase.

---

## 1. Project Overview & Philosophy

**keyshell** is a keyboard-driven application launcher and system command overlay engineered with [Quickshell](https://quickshell.outfoxxed.me/) (Qt 6 / QML declarative shell framework) for Wayland desktop environments.

### Core Tenets
- **One Summoned Overlay, No Status Bar, No Daemon**: keyshell does not render a persistent desktop bar, dock, or widgets. It lives as a single summoned overlay that appears instantaneously on demand across active monitors and vanishes immediately upon dismissal or action execution.
- **Keyboard-First & Modal Fluency**: Every interaction—launching applications, switching active windows, browsing files, adjusting audio volume, managing media playback, interacting with tray applets, toggling Bluetooth, controlling system power, or connecting to Wi-Fi—is 100% controllable from the keyboard with rapid hotkeys and prefix sigils.
- **Instantaneous Kinetic Responsiveness**: Micro-animations are tight and punchy (<100ms transitions). There is zero perceptible lag or sluggishness; interactions must feel tactile, snappy, and mechanical.

---

## 2. Visual Style & Aesthetic DNA

keyshell's visual presentation is radically distinct from conventional minimalist or flat desktop shells. It draws direct inspiration from high-energy arcade and action video game user interfaces:

### Tripartite Visual Inspiration
1. **denshattack!** ([Reference](https://www.gameuidatabase.com/gameData.php?id=2567)):
   - **Arcade Speed & Industrial Signage**: High-contrast caution/racing aesthetics, slanted banners, segmented block meters, rapid mechanical readouts, and vibrant cyan/magenta highlights.
2. **Persona (Persona 3 Reload / Persona 5 / Tactica)** ([Reference](https://www.gameuidatabase.com/index.php?series=36&plat=0&sort=2)):
   - **Dynamic Diagonal Slashes & Paper Cutouts**: Asymmetrical geometry, bold diagonal perspective, stark high-contrast ink black vs. paper white silhouettes, punctuated by intense crimson red.
   - **Hard Drop Shadows**: Solid geometric offset shadows with **zero blur or feathering** (`Theme.shadowOffset`).
   - **Heavy Typography**: Extreme all-caps italic display faces, high visual impact, tightly spaced and confident.
3. **Hi-Fi RUSH** ([Reference](https://www.gameuidatabase.com/gameData.php?id=1678)):
   - **Comic Pop-Art Cel-Shading**: 3px thick comic outlines, Ben-Day/halftone energy, tactile segmented meters, bouncy cubic easing, expressive badges, and arcade-style confirmation states (e.g. hold-to-confirm power meters).

### Design Rules for Any New UI Code
- **Palette Hygiene (`Theme.qml`)**:
  - `Theme.slab` (`#0B0B0F`): Deepest ink background.
  - `Theme.surfaceAlt` (`#121218`): Raised container background.
  - `Theme.ink` (`#F2F2F7`): High-legibility crisp text and foreground elements.
  - `Theme.accent` (`#FF2E63`): Primary hot crimson / magenta pink (selection borders, active state, destructive holds).
  - `Theme.accentAlt` (`#08F7FE`): Electric cyan / neon teal (hard drop shadows, default badges, auxiliary accents).
  - `Theme.muted` (`#5A5A66`): Desaturated dark gray for subtitles and inactive key hints.
- **Transforms & Geometry**:
  - On every reveal, `Reveal.resample()` dynamically randomizes the central band tilt angle, shear factor, and left/right heights within bounds set in `Config.qml`.
  - Content containers and cards utilize affine shearing via `Matrix4x4` with `Reveal.shearFactor`.
  - Never introduce rounded circular pills or soft Gaussian blurs. Borders are thick (2–3px), shadows are solid offset rectangles.
- **Typography (`Theme.heavy(pixelSize)`)**:
  - Default face is `Red Hat Display` (or `Teko` / `Bebas Neue` if installed).
  - Configured with `Font.Black`, `italic: true`, `capitalization: Font.AllUppercase`, and `hintingPreference: Font.PreferNoHinting`.
  - Always set `renderType: Text.QtRendering` for crisp hardware rendering.
- **Animation Curves**:
  - Use `Easing.OutCubic` with short durations: `Config.expandDurationMs` (90ms) and `Config.scrollDurationMs` (70ms). Never use long, floaty mobile-style easing.

---

## 3. Window Manager & Compositor Integration

### Primary Target: niri
keyshell is authored primarily for [niri](https://github.com/YaLTeR/niri), a scrollable-tiling Wayland compositor.
- Summoned via niri keybinds executing `qs -c keyshell ipc call keyshell toggle` or mode-specific variants.
- Leverages niri's IPC (`niri msg -j windows`, `niri msg -j workspaces`, `niri msg action focus-window`) to query window metadata, active workspaces, and monitor output assignments.

### Mandate: Support Other Wayland Compositors Gracefully
While optimized for niri, keyshell **must maintain compatibility with other Wayland compositors** (Sway, Hyprland, River, Labwc, and generic wlroots environments):
1. **Generic Window Discovery via Wayland Protocols**:
   - `WindowBackend.qml` uses Quickshell's `ToplevelManager` (`wlr-foreign-toplevel-management-unstable-v1`) as the base source of truth for windows, previews (`tl.previewSource`), and focus activation (`tl.activate()`).
2. **Dynamic WM IPC Detection**:
   - In `WindowBackend.qml`, compositor inspection commands dynamically check socket and binary presence before querying:
     - If `$NIRI_SOCKET` exists and `niri` is available → parse `niri msg -j`.
     - Else if `$SWAYSOCK` exists and `swaymsg` is available → parse `swaymsg -t get_tree`.
     - Future WM additions (e.g. `$HYPRLAND_INSTANCE_SIGNATURE` with `hyprctl`) must follow this pattern and fail gracefully without crashing or hanging the shell.
3. **Session & Power Actions (`PowerBackend.qml`)**:
   - When triggering session logout, keyshell must check the active environment (e.g. check for niri, sway, hyprland) before running compositor-specific exit commands, with a standard fallback to `loginctl terminate-session ${XDG_SESSION_ID:-self}`.
4. **Wayland Layer-Shell Contract**:
   - Defined in `components/LauncherWindow.qml`:
     - `WlrLayershell.layer: WlrLayer.Overlay`
     - `WlrLayershell.exclusionMode: ExclusionMode.Ignore`
     - `WlrLayershell.namespace: "keyshell"`
     - `WlrLayershell.keyboardFocus: Launcher.open ? WlrKeyboardFocus.OnDemand : WlrKeyboardFocus.None`

---

## 4. Codebase Architecture

```
keyshell/
├── README.md                          # User installation and quickstart guide
├── AGENTS.md                          # This document
├── docs/
│   └── INSTALL.md                     # Session-shell installation guide
├── contrib/
│   └── keyshell.service               # systemd user unit for the session shell
├── tests/
│   └── run.js                         # Node harness for the rank.js scoring math
└── quickshell/
    └── keyshell/
        ├── shell.qml                  # ShellRoot, multi-screen Variants, IpcHandler
        ├── Config.qml                 # Singleton: all numeric constants, timeouts, tunables
        ├── Theme.qml                  # Singleton: palette, typography generator, shadow metrics
        ├── Launcher.qml               # Singleton: core application state, results, selection
        ├── BackendManager.qml         # Singleton: mode registry, cycling, prefix dispatch
        ├── Reveal.qml                 # Singleton: geometry resampling (tilt, shear, bounds)
        ├── Frecency.qml               # Singleton: persisted usage scores (XDG state)
        ├── SearchAggregator.qml       # Singleton: merged fan-out, ranking, settle timer
        ├── rank.js                    # Plain JS: match tiers, frecency decay, merge/sort
        ├── calc.js                    # Plain JS: calculator expression parser & formatter
        ├── plugin-proto.js            # Plain JS: plugin trigger, line and manifest parsing
        ├── clipboard-watch.py         # Clipboard capture helper, one process per change
        ├── PluginRegistry.qml         # Singleton: plugin discovery, dynamic backend list
        ├── PluginBackend.qml          # Component (not a singleton): one instance per plugin
        ├── Terminal.qml               # Singleton: resolves and launches the terminal
        │
        ├── AppBackend.qml             # Applications backend (DesktopEntries)
        ├── WindowBackend.qml          # Open windows backend (ToplevelManager + WM IPC)
        ├── FileBackend.qml            # File search backend (dsearch subprocess)
        ├── AudioBackend.qml           # Pipewire audio sink/source management
        ├── MediaBackend.qml           # MPRIS media player controller
        ├── TrayBackend.qml            # SNI StatusNotifierItem system tray & DBusMenu
        ├── BluetoothBackend.qml       # BlueZ Bluetooth adapter & device management
        ├── PowerBackend.qml           # UPower battery & systemd power/session management
        ├── NetworkBackend.qml         # NetworkManager Wi-Fi/Ethernet management
        ├── CalculatorBackend.qml      # Calculator backend (calc.js evaluation, copies result)
        ├── RunBackend.qml             # Run-command backend (launches the command in a terminal)
        │
        └── components/
            ├── LauncherWindow.qml     # PanelWindow overlay, keyboard routing, reveal animations
            ├── Band.qml               # Tilted trapezoid band (Slab Shape) container
            ├── Card.qml               # Sheared card frame with drop shadow & delegate loader
            ├── Reel.qml               # Horizontal cyclic ListView carousel
            ├── SearchField.qml        # Mode badge, search text input, status hint readout
            │
            ├── AppCardDelegate.qml       # Card presentation for application entries
            ├── WindowCardDelegate.qml    # Card presentation for window entries + live thumbnail
            ├── FileCardDelegate.qml      # Card presentation for file hits + file type badges
            ├── AudioCardDelegate.qml     # Card presentation for audio devices + level meter
            ├── MediaCardDelegate.qml     # Card presentation for MPRIS players + track info
            ├── TrayCardDelegate.qml       # Card presentation for tray applets + status
            ├── BluetoothCardDelegate.qml  # Card presentation for Bluetooth devices + signal
            ├── PowerCardDelegate.qml      # Card presentation for power profiles & power actions
            ├── NetworkCardDelegate.qml    # Card presentation for Wi-Fi networks + RSSI
            ├── ClipboardCardDelegate.qml  # Card presentation for clipboard entries + age
            ├── CalcCardDelegate.qml       # Card presentation for calculator results
            ├── RunCardDelegate.qml        # Card presentation for run-command entries
            │
            ├── VolumeSliderView.qml      # Full console: discrete block audio slider
            ├── MediaConsoleView.qml      # Full console: playback controls & scrub bar
            ├── TrayMenuView.qml          # Full console: numbered interactive DBus menu list
            ├── BluetoothConsoleView.qml  # Full console: device pairing, trust, connect actions
            ├── PowerConsoleView.qml      # Full console: power profile selection & hold-meter
            ├── NetworkConsoleView.qml    # Full console: network actions & details
            └── NetworkPasswordView.qml   # Full console: inline WPA/WPA2 PSK passphrase entry
```

---

## 5. Interaction Modes & Sigil Prefixes

Users can navigate backends via `Tab` / `Shift+Tab`, dedicated compositor keybinds, or by typing a single trigger prefix into an empty search field:

| Mode | Badge | Prefix | Primary Shortcut | Primary Enter Action |
| --- | --- | --- | --- | --- |
| **Applications** | `APPS` | *(none)* | `Mod+Space` | Launch desktop application |
| **Windows** | `WINDOWS` | `@` | `Mod+W` / `Alt+Tab` | Focus window via compositor |
| **Files** | `FILES` | `/` | `Mod+F` | Open with `xdg-open` via `dsearch` |
| **Audio** | `AUDIO` | `!` | `Mod+A` | Open `VolumeSliderView` (`Space` opens slider, `M` mutes, `D` sets default) |
| **Media** | `MEDIA` | `>` | `Mod+M` | Open `MediaConsoleView` (`Space` play/pause, `N` next, `P` prev, `R` raise) |
| **Tray** | `TRAY` | `^` | `Mod+T` | Open `TrayMenuView` (`1-9` select, `Backspace` up) |
| **Bluetooth** | `BT` | `*` | `Mod+B` | Open `BluetoothConsoleView` (`E` toggle power, `S` scan) |
| **Power** | `POWER` | `%` | `Mod+P` | Open `PowerConsoleView` (destructive actions require 1s hold or double-tap) |
| **Network** | `NET` | `&` | `Mod+N` | Open `NetworkConsoleView` (`W` toggle Wi-Fi, `S` scan) |
| **Calculator** | `CALC` | `=` | *(none)* | Copy the result to the clipboard via `wl-copy` |
| **Run command** | `RUN` | `:` | *(none)* | Run the query in a terminal (`$TERMINAL`, else `ghostty`, else `alacritty`) |

The `CALC` and `RUN` backends declare `searchable: false` and `trackable: false`, so they stay out of the merged default query and never contribute to frecency. They are reached only by their prefix or by `Tab` cycling.

### Global Keyboard Map (Inside Overlay)
- `printable characters`: Appends to query filter (or triggers prefix mode switch if query is empty).
- `Backspace`: Removes last query character; exits submenus or consoles if query is empty.
- `Tab` / `Shift+Tab`: Cycles through available backends in order (skipping empty media/tray when appropriate).
- `Left` / `Right`: Moves selection across the horizontal reel.
- `Enter` / `Return`: Executes the primary action of the selected item, or commits modal input. An item carrying `isDestructive` is gated instead: it needs a hold of `Config.powerHoldDurationMs` or a double-tap, in any mode, not only `POWER`.
- `Escape`: Closes active console/submenu; dismisses keyshell if on root reel.
- `Space`: Quick-action toggle in supported modes (play/pause media, open audio slider, open console).

---

## 6. Developing, Running & Testing

### Runtime Prerequisites
- **Quickshell 0.3.2+** with Qt 6 Wayland and Services support.
- **Pipewire** (for `Quickshell.Services.Pipewire`).
- **NetworkManager** (for `Quickshell.Networking`).
- **BlueZ** (for `Quickshell.Bluetooth`).
- **UPower** (for `Quickshell.Services.UPower`).
- **dsearch** (`/usr/bin/dsearch` installed and indexed for file search).
- **wl-clipboard** (`wl-paste`, `wl-copy`) for clipboard capture and result copying.
- **python3** to run `clipboard-watch.py`, the clipboard capture helper.
- **xdg-open** to open file hits and plugin `open-url` actions.
- A terminal in `Config.terminalCandidates` (`ghostty`, `alacritty`) or `$TERMINAL` for run-command.
- A running Wayland session (`niri`, `sway`, etc.).

### Installing as a Session Shell
Installation is the one deployment mode that matters to users: the configuration
is symlinked in as `~/.config/quickshell/keyshell`, and the shell is started once
per session, either by `contrib/keyshell.service` (a systemd user unit bound to
`graphical-session.target`) or by the compositor's autostart. Never mix the two,
and always pass `--no-duplicate`, because two instances of the configuration
both answer the same IPC target and both draw an overlay.

`docs/INSTALL.md` is the user-facing write-up: prerequisites, both startup
methods, the mode ids and IPC function names for keybinds, verification, update,
removal and troubleshooting. Keep it, the README's Install section and
`contrib/keyshell.service` in agreement when any of them changes.

### Launching in Development
Run the configuration directly from the repository root:
```bash
qs -p quickshell/keyshell
```

To run with verbose debug logs:
```bash
qs -vv -p quickshell/keyshell
```

### Testing IPC Calls
keyshell exposes an extensive IPC API via `shell.qml` under target `keyshell`:
```bash
# Toggle visibility
qs ipc call keyshell toggle

# Open directly into a specific mode
qs ipc call keyshell openMode "windows"
qs ipc call keyshell openMode "audio"
qs ipc call keyshell openMode "network"

# Audio controls (with auto-HUD popup)
qs ipc call keyshell volumeUp 0.05
qs ipc call keyshell volumeDown 0.05
qs ipc call keyshell toggleMute

# Media controls
qs ipc call keyshell playPause
qs ipc call keyshell nextTrack
qs ipc call keyshell previousTrack

# System controls
qs ipc call keyshell toggleBluetoothPower
qs ipc call keyshell toggleWifi
qs ipc call keyshell lockSession
```

---

## 7. Developer Rules & Best Practices for Agents

When implementing features, fixing bugs, or refactoring in keyshell, adhere strictly to these rules:

### 1. Architectural Guardrails
- **No Hardcoded Magic Numbers**: Every dimension, animation timing, tick count, and timeout belongs in `Config.qml`.
- **No Inline Palette Hacking**: Never use arbitrary hex colors in delegates or views. Always refer to `Theme.slab`, `Theme.accent`, etc.
- **Singletons**: All global managers and backends must declare `pragma Singleton` at line 1 and instantiate with `Singleton { id: ... }`.
- **Resource Cleanup**:
  - Always stop timers and processes (`Process.running = false`) when views or the launcher are dismissed (`Launcher.dismiss()`).
  - Never allow asynchronous subprocesses (e.g. `dsearch`, `niri msg`) to race; track request generations (`generation` integer pattern) to discard stale asynchronous results.

### 2. Maintaining Visual Fidelity
- **Keep Outlines & Shadows Crisp**:
  - Outlines must be 2–3px wide with solid borders.
  - Drop shadows must use `Theme.shadowOffset` without blurs.
  - Apply `layer.enabled: true`, `layer.smooth: true`, and `layer.samples: 4` to sheared shapes (`Shape` and transformed `Rectangle`) to eliminate aliasing along diagonal edges.
- **Shearing Discipline**:
  - All items rendered within `Band.qml` that follow the card aesthetic must apply `Matrix4x4` shearing with `Reveal.shearFactor` and offset by `-Reveal.shearFactor * height * 0.5`.

### 3. Adding a New Backend Mode
To add a new backend (e.g. `ClipboardBackend` or `CalculatorBackend`):
1. Create `quickshell/keyshell/MyBackend.qml` as a `Singleton`:
   - Declare properties: `id`, `name`, `badge`, `prefix`, `cardDelegate`.
   - Declare `readonly property bool searchable: true` to include it in the merged default query, and `readonly property bool trackable: true` to let activations feed frecency. Both default to `false`, so opting in is explicit.
   - Implement `search(query, limit)` and `refresh()` methods.
   - Emit `signal resultsReady(string query, var items)`.
2. Register the backend by adding it to the `backends` array in `BackendManager.qml`. Result collection is registry-driven, so no per-backend `Connections` block is needed.
3. Create `components/MyCardDelegate.qml` using `Theme.qml` and `Theme.heavy()` typography.
4. Add IPC trigger endpoints to `shell.qml` if external hotkey launching is desired.
5. If the backend requires a dedicated console view (e.g. slider or details sheet), add `MyConsoleView.qml` and wire open/close signals in `Launcher.qml` and `Band.qml`.

### 4. Writing a Plugin
A plugin is an external executable plus a manifest, so it needs no QML and can be written in any language.
1. Create `$XDG_CONFIG_HOME/keyshell/plugins/<name>/plugin.json`:
   ```json
   {
     "name": "Docker",
     "badge": "DOCKER",
     "trigger": ";docker",
     "command": ["/usr/bin/docker-plugin"],
     "glyph": "🐳",
     "searchable": false,
     "trackable": false
   }
   ```
   `trigger` must be at least `Config.minTriggerLength` (2) characters, so it can never collide with a single-character sigil such as `=` or `@`. `searchable` opts the plugin into the merged default query; `trackable` lets its activations feed frecency. Both default to `false`.
2. The shell spawns `command` and writes one JSON line per request to its stdin: `{"query": "ps", "limit": 60, "merged": false}`. `merged` is `true` for the merged default query and `false` for the plugin's own mode.
3. Answer with one JSON object per line on stdout, then `{"done": true}` to end the response. The terminator is what makes the end of a response deterministic. Each item needs an `id`, a `title` and at least one valid action:
   ```json
   {"id": "web-1", "title": "web-1", "subtitle": "running", "glyph": "🐳", "badge": "UP", "destructive": true,
    "actions": [{"type": "exec", "argv": ["docker", "rm", "-f", "web-1"]}]}
   ```
   `destructive` is optional and defaults to `false`. Set it on any item whose action is dangerous: the shell then refuses to run it on a plain Enter and requires the power actions' confirmation instead, a hold of `Config.powerHoldDurationMs` or a double-tap, drawn as a hold meter on the card. `PluginBackend` maps it to the shell's existing `isDestructive` item field, which is the same field `PowerBackend` sets, so the confirmation path is shared.
4. Actions are declarative and only these five types are executed: `copy` (`text`), `open-url` (`url`), `exec` (`argv`), `exec-terminal` (`argv`), `set-query` (`query`). Unknown types, missing fields and malformed lines are dropped, so a broken plugin degrades to no results.
5. A plugin process is started when its mode is entered and stopped when the mode changes or the launcher is dismissed. `PluginRegistry.stopAll()` runs on dismiss.

A plugin never runs code inside the shell process. The shell only ever executes a declared action, which is the same trust level as running the plugin executable you installed yourself.

`plugins/processes` in this repository is the reference implementation: a Python plugin that lists processes and terminates one. Copy its shape, and note the choices it had to make deliberately. Terminating is destructive, so its kill item sets `destructive` and the plugin returns nothing for an empty query in either mode, which leaves no way to reach the launcher with an arbitrary process selected; it sends `SIGTERM` and never `SIGKILL`. A searchable plugin is asked both its own mode's query and the merged default query, and the `merged` flag says which, which matters for a plugin that should list entries when opened but stay silent in the default query; this one behaves the same in both, so it ignores the flag. And its item ids are pids, which are not stable across runs, which is why `trackable` is `false` there: frecency needs an id that survives a restart.

`plugins/emoji` is the second reference implementation, and the one to copy when a plugin is harmless. It copies rather than runs, so it is the only example of the `copy` action. It is both `searchable` and `trackable`, and it is the case the `merged` flag exists for: an empty merged query returns nothing while opening its own mode returns a starter set. Its ids are codepoints and stay stable across runs, so `trackable` is meaningful there and the shell orders the picker by your usage.

The shell asks a plugin one question at a time, holding a newer query until the outstanding response finishes and keeping only the newest. A plugin never has to correlate responses with requests, and `PluginBackend` relies on that: it has a single request in flight and no request ids.

### 5. Preserving Wayland Compositor Flexibility
- Never write shell commands assuming `niri` is the only compositor in existence.
- Always check for environment variables (`$NIRI_SOCKET`, `$SWAYSOCK`, `$HYPRLAND_INSTANCE_SIGNATURE`) before dispatching compositor-specific CLI tools.
- Provide graceful degradation: if compositor-specific metadata (like workspace names) cannot be retrieved, fallback to generic protocol data (`ToplevelManager`) or clean placeholders without throwing errors.

### 6. The Clipboard Watcher and Its Limits
- `ClipboardBackend.qml` runs `wl-paste --watch` for the whole shell lifetime. It is the **only** process that must outlive the launcher, so never add it to `Launcher.dismiss()` or to the `setMode()` cleanup that stops plugin processes. The pause key is the only thing that stops it.
- Capture is one helper process per change, `clipboard-watch.py`, which sits beside `shell.qml` and is located through `Quickshell.shellDir`. It owns validation, dedupe, the entry cap and the TTL, so those rules live in one testable place rather than in QML. The store is `${XDG_STATE_HOME:-~/.local/state}/keyshell/clipboard.json`, written atomically, and the shell passes the caps to the helper as environment variables.
- The policy is **explicit controls only**: bounded retention, a pause key that stops capture, a clear key, and a per-entry delete. There is deliberately no secret detection and no password-manager skip list, because the usual ways a password gets copied (a terminal running `pass -c`, or a browser's own manager) are exactly what such a list misses. Do not add heuristics here, and do not write documentation that implies a guarantee this feature cannot make.
