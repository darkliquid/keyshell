# Keyshell Configuration (`config.toml`) Reference

This document provides a comprehensive guide to configuring **keyshell**, covering file locations, default behaviors, environment variables, CLI flags, and an in-depth reference for every configuration option in `config.toml`.

---

## Table of Contents

1. [Configuration File Locations & Loading Order](#1-configuration-file-locations--loading-order)
2. [Configuration Philosophy: Layered Defaults](#2-configuration-philosophy-layered-defaults)
3. [Section-by-Section Reference](#3-section-by-section-reference)
   - [`[ui]` — Theming & Scenes](#ui--theming--scenes)
   - [`[reveal]` — Slanted Band Geometry & Procedural Randomization](#reveal--slanted-band-geometry--procedural-randomization)
   - [`[anim]` — Animation Durations](#anim--animation-durations)
   - [`[overlay]` — Search Limits & Process Timeouts](#overlay--search-limits--process-timeouts)
   - [`[drinks]` — Preferred Terminal Emulators](#drinks--preferred-terminal-emulators)
   - [`[commands]` — Environment Action Overrides & Portability](#commands--environment-action-overrides--portability)
   - [`[frecency]` — Search Ranking & Historical Scoring](#frecency--search-ranking--historical-scoring)
   - [`[[instance]]` — Subprocess Plugin Declarations](#instance--subprocess-plugin-declarations)
4. [Complete Annotated Example `config.toml`](#4-complete-annotated-example-configtoml)
5. [CLI Flags & Environment Overrides](#5-cli-flags--environment-overrides)

---

## 1. Configuration File Locations & Loading Order

Keyshell looks for its configuration file in the following order:

1. **Explicit CLI Flag**: `--config <path>` (e.g. `keyshell run --config /custom/path/config.toml`)
2. **Environment Variable**: `KEYSHELL_CONFIG` (e.g. `export KEYSHELL_CONFIG="$HOME/.config/keyshell/custom.toml"`)
3. **XDG Standard Path**: `$XDG_CONFIG_HOME/keyshell/config.toml`
4. **Fallback Default**: `$HOME/.config/keyshell/config.toml`

### Missing Config File Behavior
If no configuration file is found at the target path, **keyshell runs without error**, applying its full set of internal defaults. You only need to create a `config.toml` when you wish to override specific settings.

---

## 2. Configuration Philosophy: Layered Defaults

Configuration in keyshell is completely **layered and partial**:
- Every setting has a built-in default compiled into the binary.
- Any setting defined in your TOML file overrides the default.
- Any omitted table or key retains its built-in default value.
- Syntax errors or malformed TOML will cause keyshell to report an error and refuse to start, preventing silent misconfigurations.

---

## 3. Section-by-Section Reference

### `[ui]` — Theming & Scenes

Controls the visual theme loaded by the graphical host.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `scene` | `string` | `""` | Path to a `.ksl` (Keyshell Scene Language) file. An empty string loads keyshell's built-in embedded default theme (the classic tilted band). Relative paths resolve from keyshell's working directory. |

#### Example:
```toml
[ui]
# Use the built-in dropdown theme
scene = "scenes/dropdown.ksl"

# Or point to a custom user theme
# scene = "/home/user/.config/keyshell/my-theme.ksl"
```

> [!TIP]
> You can temporarily override `[ui] scene` on the command line using `--scene <path>`. Combine with `--watch` for instant live-reload on save:
> ```bash
> keyshell run --scene scenes/dropdown.ksl --watch
> ```

---

### `[reveal]` — Slanted Band Geometry & Procedural Randomization

Governs the procedural math and randomization ranges used by the default slanted band overlay (`internal/render/default.ksl`) upon each summon.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `min_tilt_deg` | `float` | `5.0` | Minimum absolute tilt angle in degrees for the band. Ensures the band never renders completely flat. |
| `center_spread_factor` | `float` | `0.10` | Fractional vertical range around the screen center within which the band center is randomly placed (e.g., `0.10` = ±10% screen height). |
| `taper_max_factor` | `float` | `0.15` | Maximum allowable height taper between the left and right ends of the band. |
| `shear_coupling` | `float` | `1.0` | Coupling factor between the band's tilt angle and the card skew angle. |
| `shear_jitter_deg` | `float` | `2.0` | Maximum random angular jitter in degrees applied to card shear on each summon. |
| `shear_min_deg` | `float` | `6.0` | Minimum clamped shear angle in degrees. |
| `shear_max_deg` | `float` | `14.0` | Maximum clamped shear angle in degrees. |
| `band_height_px` | `float` | `290.0` | Nominal height of the center band in pixels. |
| `band_center_factor` | `float` | `0.5` | Target vertical center of the band as a fraction of display height (`0.5` is the exact center of the screen). |
| `min_end_height_px` | `float` | `200.0` | Minimum height in pixels that either side of the band can shrink to when tapered. |
| `card_width_px` | `float` | `320.0` | Nominal width of an unselected result card in pixels. |
| `card_width_max_px` | `float` | `360.0` | Maximum width of the active/selected result card in pixels. |
| `search_height_px` | `float` | `48.0` | Height in pixels of the search input field. |
| `content_padding_px` | `float` | `12.0` | Inner padding around search and card contents in pixels. |

---

### `[anim]` — Animation Durations

Tuning knobs for keyshell's snappy, mechanical animation feel.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `expand_ms` | `int` | `90` | Duration in milliseconds for the overlay summon and expand animation. |
| `scroll_ms` | `int` | `70` | Duration in milliseconds for selection movement and card reel scroll transitions. |

#### Example:
```toml
[anim]
expand_ms = 110
scroll_ms = 60
```

---

### `[overlay]` — Search Limits & Process Timeouts

Controls result limits, settle windows, and process lifecycle timeouts.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `max_results` | `int` | `60` | Maximum total number of ranked results passed to the UI for rendering. |
| `settle_ms` | `int` | `80` | Settle debounce window in milliseconds for rapid control changes (volume sliders, brightness, etc.) before committing changes. |
| `idle_ms` | `int` | `300000` | Idle timeout in milliseconds (default is 5 minutes) before idle lazy backend or plugin subprocesses are reaped. |

#### Example:
```toml
[overlay]
max_results = 50
settle_ms = 100
idle_ms = 180000  # 3 minutes
```

---

### `[drinks]` — Preferred Terminal Emulators

Defines candidate terminal emulators used when running commands with `exec-terminal` (such as in `run` mode or terminal-based desktop applications).

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `terminal` | `array of strings` | `["ghostty", "alacritty"]` | List of terminal emulator binary names. Keyshell searches `$PATH` in declaration order and uses the first binary found, executing it with `-e <command>`. |

#### Example:
```toml
[drinks]
terminal = ["foot", "kitty", "ghostty", "alacritty", "xterm"]
```

---

### `[commands]` — Environment Action Overrides & Portability

Allows overriding keyshell's host execution commands. This enables seamless portability across different Wayland compositors (Niri, Sway, Hyprland, River, Wayfire) and different operating systems (Linux, macOS, BSD, Windows).

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `copy` | `array of strings` | `["wl-copy"]` | Command executed to copy text to the clipboard. The text to copy is appended as the last argument. |
| `open_url` | `array of strings` | `["xdg-open"]` | Command executed to open a URL or launch a file association. The URL/path is appended as the last argument. |
| `focus_window` | `array of strings` | Auto-detected | Command executed to focus an existing window by its ID. Supports an `{id}` token anywhere in the arguments. If `{id}` is omitted, the ID is appended. |

#### Built-in `focus_window` Auto-Detection:
If `[commands] focus_window` is not explicitly set in `config.toml`, keyshell checks environment variables:
- If `NIRI_SOCKET` is present: `["niri", "msg", "action", "focus-window", "{id}"]`
- If `SWAYSOCK` is present: `["swaymsg", "[con_id={id}]", "focus"]`
- Otherwise: Reports an error prompting the user to configure `[commands] focus_window`.

#### Compositor Examples for `focus_window`:

**Hyprland**:
```toml
[commands]
focus_window = ["hyprctl", "dispatch", "focuswindow", "address:{id}"]
```

**Sway / i3**:
```toml
[commands]
focus_window = ["swaymsg", "[con_id={id}]", "focus"]
```

**Niri**:
```toml
[commands]
focus_window = ["niri", "msg", "action", "focus-window", "{id}"]
```

**macOS (via yabai / pbcopy / open)**:
```toml
[commands]
copy = ["pbcopy"]
open_url = ["open"]
focus_window = ["yabai", "-m", "window", "--focus", "{id}"]
```

---

### `[frecency]` — Search Ranking & Historical Scoring

Keyshell ranks search results using fuzzy string matching augmented by **frecency** (a hybrid score factoring both frequency and recency of use). The parameters below tune the scoring formula and persistence.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `count_cap` | `int` | `10` | Maximum count multiplier applied to an item's score, preventing single frequently used items from permanently monopolizing top rankings. |
| `max_age_days` | `int` | `90` | Maximum age in days before historical usage entries are purged from disk. |
| `flush_debounce_ms` | `int` | `2000` | Debounce duration in milliseconds before writing updated frecency stats to disk (`~/.local/share/keyshell/frecency.json`). |
| `buckets` | `array of tables` | *(See below)* | Age brackets and their assigned weight multipliers. |

#### Default Frecency Buckets:
```toml
[[frecency.buckets]]
max_age_ms = 3600000    # Used within the last 1 hour
weight = 4

[[frecency.buckets]]
max_age_ms = 86400000   # Used within the last 24 hours
weight = 3

[[frecency.buckets]]
max_age_ms = 604800000  # Used within the last 7 days
weight = 2

[[frecency.buckets]]
max_age_ms = 2592000000 # Used within the last 30 days
weight = 1
```

---

### `[[instance]]` — Subprocess Plugin Declarations

Keyshell runs plugins as isolated subprocesses over a JSON-RPC / line-delimited IPC protocol. First-party plugins (`audio`, `media`, `network`, `power`, `clipboard`, `tray`) and third-party custom plugins are declared as `[[instance]]` array tables.

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `id` | `string` | *(Required)* | Unique identifier for the plugin. Also defines the search mode name (e.g. `keyshell openMode audio`). |
| `command` | `array of strings` | `["keyshell", "plugin", "<id>"]` | Command line used to spawn the subprocess. If omitted or empty, keyshell automatically invokes itself as `<self> plugin <id>`. |
| `mode` | `string` | `"hot"` | Lifecycle management policy: `"hot"`, `"lazy"`, or `"auto"`. |
| `noreap` | `bool` | `false` | When `true`, keyshell will never kill or reap this process when idle or closing. Essential for clipboard monitors or long-running daemons. |

#### Lifecycle Modes:
- **`hot`**: Spawned during keyshell startup and kept warm in memory for instantaneous queries. Recommended for frequently used plugins (`audio`, `power`, `clipboard`).
- **`lazy`**: Spawned on-demand when the user searches or navigates into the plugin's mode. Reaped after `[overlay] idle_ms` to conserve system memory.
- **`auto`**: Managed automatically based on user invocation frequency.

#### Default Instances:
If no `[[instance]]` entries are defined in `config.toml`, keyshell automatically runs the following:
```toml
[[instance]]
id = "audio"
mode = "hot"

[[instance]]
id = "media"
mode = "hot"

[[instance]]
id = "network"
mode = "hot"

[[instance]]
id = "power"
mode = "hot"

[[instance]]
id = "clipboard"
mode = "hot"
noreap = true

[[instance]]
id = "tray"
mode = "hot"
```

#### Custom Third-Party Plugin Example:
You can write external plugins in Python, Rust, Bash, or Go and register them directly:
```toml
[[instance]]
id = "weather"
command = ["/usr/local/bin/keyshell-plugin-weather", "--units", "metric"]
mode = "lazy"
```

---

## 4. Complete Annotated Example `config.toml`

Below is a complete, production-ready `config.toml` demonstrating all available configuration sections:

```toml
# ==============================================================================
# Keyshell Master Configuration (~/.config/keyshell/config.toml)
# ==============================================================================

# ---- User Interface & Theming ------------------------------------------------
[ui]
# Select the scene file. Omit or leave empty for the default slanted band.
scene = "scenes/dropdown.ksl"

# ---- Visual Reveal Geometry (Default Band Theme) -----------------------------
[reveal]
min_tilt_deg         = 5.0    # Minimum tilt angle in degrees
center_spread_factor = 0.10   # Vertical random placement spread (±10%)
taper_max_factor     = 0.15   # Max taper difference between band ends
shear_coupling      = 1.0    # Coupling factor between tilt and card shear
shear_jitter_deg     = 2.0    # Jitter applied to shear angle
shear_min_deg        = 6.0    # Minimum shear angle in degrees
shear_max_deg        = 14.0   # Maximum shear angle in degrees
band_height_px       = 290.0  # Center band height in pixels
band_center_factor   = 0.5    # Nominal vertical center (0.5 = 50% screen height)
min_end_height_px     = 200.0  # Minimum clamped end height
card_width_px        = 320.0  # Base card width
card_width_max_px     = 360.0  # Active card max width
search_height_px     = 48.0   # Search field height
content_padding_px   = 12.0   # Inner content padding

# ---- Animation Feel & Durations ----------------------------------------------
[anim]
expand_ms = 90   # Overlay summon expand duration (ms)
scroll_ms = 70   # Card reel / list scroll animation (ms)

# ---- Result Overlay Limits ---------------------------------------------------
[overlay]
max_results = 60          # Max ranked results shown
settle_ms   = 80          # Optimistic control update settle debounce (ms)
idle_ms     = 300000      # Lazy plugin idle reap timeout (5 minutes)

# ---- Terminal Candidates -----------------------------------------------------
[drinks]
# Checked in order against $PATH when launching terminal commands
terminal = ["ghostty", "foot", "kitty", "alacritty", "xterm"]

# ---- Host Execution & Portability Overrides ----------------------------------
[commands]
# Custom clipboard copy command (defaults to wl-copy)
copy = ["wl-copy"]

# Custom URL / association opener (defaults to xdg-open)
open_url = ["xdg-open"]

# Window focus command for your compositor.
# Use {id} as a placeholder, or the window ID will be appended.
# Examples:
#   Hyprland: ["hyprctl", "dispatch", "focuswindow", "address:{id}"]
#   Niri:     ["niri", "msg", "action", "focus-window", "{id}"]
#   Sway:     ["swaymsg", "[con_id={id}]", "focus"]
focus_window = ["niri", "msg", "action", "focus-window", "{id}"]

# ---- Frecency Usage Ranking --------------------------------------------------
[frecency]
count_cap          = 10     # Max count multiplier
max_age_days       = 90     # Retention period before pruning entries
flush_debounce_ms  = 2000   # Atomic disk save debounce interval

# Age decay tiers for search ranking
[[frecency.buckets]]
max_age_ms = 3600000     # 1 hour
weight = 4

[[frecency.buckets]]
max_age_ms = 86400000    # 24 hours
weight = 3

[[frecency.buckets]]
max_age_ms = 604800000   # 7 days
weight = 2

[[frecency.buckets]]
max_age_ms = 2592000000  # 30 days
weight = 1

# ---- Plugin Subprocess Instances ---------------------------------------------
# First-party plugins (empty command resolves to `<self> plugin <id>`)
[[instance]]
id = "audio"
mode = "hot"

[[instance]]
id = "media"
mode = "hot"

[[instance]]
id = "network"
mode = "hot"

[[instance]]
id = "power"
mode = "hot"

[[instance]]
id = "clipboard"
mode = "hot"
noreap = true   # Clipboard monitor must remain running

[[instance]]
id = "tray"
mode = "hot"

# Custom user plugin example:
# [[instance]]
# id = "github"
# command = ["/home/user/bin/keyshell-github-plugin"]
# mode = "lazy"
```

---

## 5. CLI Flags & Environment Overrides

Values defined in `config.toml` can be selectively overridden using command-line flags when starting `keyshell run` or `keyshell headless`:

| CLI Flag | Config Setting Equivalent | Description |
| --- | --- | --- |
| `--config <path>` | *(Root config path)* | Specify an explicit configuration file path. |
| `--scene <path>` | `[ui] scene` | Override the scene file at runtime. |
| `--watch` | *(N/A)* | Live-reload the scene and all included `.ksl` files whenever they change on disk. |
| `--plugins <a,b>` / `--plugins none` | `[[instance]]` | Override which plugins are activated. |
| `--log-level <debug\|info\|warn\|error>` | *(N/A)* | Set logger output verbosity. |
| `--debug-addr <host:port>` | *(N/A)* | Expose Go `pprof` and `expvar` endpoints (e.g. `127.0.0.1:6060`). |
| `--otel <URL>` / `--otel-stdout` | *(N/A)* | Stream OpenTelemetry traces and metrics to an OTLP/HTTP collector or stdout. |

### Environment Variables
- `KEYSHELL_CONFIG`: Specifies the config path when `--config` is not passed.
- `XDG_CONFIG_HOME`: Standard configuration directory (defaults to `$HOME/.config`).
- `NIRI_SOCKET` / `SWAYSOCK`: Auto-detected by keyshell to infer compositor window focus commands when `[commands] focus_window` is not explicitly set.
