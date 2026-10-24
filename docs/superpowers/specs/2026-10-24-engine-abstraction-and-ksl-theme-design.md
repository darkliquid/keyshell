# Specification: Engine Abstraction & KSL Theme Refactor

**Status:** Approved  
**Date:** 2026-10-24  
**Authors:** Google DeepMind pair programming assistant & Darkliquid  

---

## 1. Overview & Motivation

Keyshell was originally designed with several tightly-coupled architectural assumptions:
1. **Hardcoded Plugin & Backend Branching:** The host binary (`internal/cli/host.go` and `internal/cli/run.go`) contains hardcoded `switch` statements and static slices for individual backend IDs (`apps`, `files`, `windows`, `calc`, `run`, `tray`, `clipboard`, `media`, `network`, `power`). The launcher and CLI treat in-process and subprocess plugins with disparate initialization pathways.
2. **Hardcoded Band Geometry & Reveal Calculations:** The Go engine (`internal/render/reveal.go`) and host configuration (`internal/config/config.go`) define 14 fixed layout fields (`min_tilt_deg`, `band_height_px`, `card_width_px`, etc.) tailored solely to the legacy tilted band layout. Furthermore, `internal/render/render.go` contains hardcoded hit-testing coordinates (`640x460` at `y=120`) to support non-band scenes like `dropdown.ksl`.
3. **Restricted Theme Tokens without User Overrides:** KSL `theme { ... }` blocks only support color hex tokens and unconditionally overwrite host theme tokens, preventing users from overriding scene theme tokens from `config.toml`.

This refactor decouples Keyshell into three clean architectural seams:
- **Unified Plugin Registry:** All backends (in-process or subprocess) implement a common interface and register dynamically without hardcoded branches.
- **Scene-Driven Hit-Testing & KSL Band Helper:** Hit-testing is computed dynamically against the resolved `eval.Frame`, and band trapezoid math is moved into a KSL-accessible geometry helper `band(...)`.
- **KSL Theme Variables with Config Cascading:** KSL `theme { ... }` blocks declare arbitrary typed variables with scene defaults, while `config.toml [theme]` provides user overrides.

---

## 2. Detailed Architecture & Design

### 2.1 Unified Plugin & Backend Architecture

#### 2.1.1 Common Plugin Contract
All backends (built-in and third-party) adhere to the [`sdk.Plugin`](file:///home/darkliquid/Projects/keyshell/internal/plugin/sdk/sdk.go) interface:
```go
type Plugin interface {
    Info() BackendInfo
    Query(ctx context.Context, q Query) (Stream, error)
    Control(ctx context.Context, c Change) error
    Invoke(ctx context.Context, i Invoke) error
}
```

#### 2.1.2 Adapters and Registry
```
                               ┌────────────────────────────────┐
                               │       launcher.Launcher        │
                               └───────────────┬────────────────┘
                                               │ queries
                               ┌───────────────▼────────────────┐
                               │    backend.ProviderRegistry    │
                               └───────┬────────────────┬───────┘
                                       │                │
                    ┌──────────────────▼─┐            ┌─▼──────────────────┐
                    │  InProcAdapter     │            │ SubprocessAdapter  │
                    │  (apps, calc, ...) │            │ (tray, net, ...)   │
                    └──────────┬─────────┘            └──────────┬─────────┘
                               │                                 │
                    ┌──────────▼─────────┐            ┌──────────▼─────────┐
                    │  sdk.Plugin (Go)   │            │ stdio JSON-RPC     │
                    └────────────────────┘            └────────────────────┘
```

- **`InProcAdapter`:** Wraps pure Go backends in-memory, eliminating process spawn overhead for high-frequency queries (`apps`, `calc`).
- **`SubprocessAdapter`:** Manages persistent JSON-RPC child processes (`keyshell plugin <name>` or third-party binaries discovered in `~/.config/keyshell/plugins/`).
- **`ProviderRegistry`:**
  - Manages active providers.
  - Dynamically builds:
    - Root searchable list: all providers where `Info().Searchable == true`.
    - Mode tab cycle: all providers declaring a non-empty `ID`.
    - Triggers: all providers declaring a non-empty `Trigger`.
  - Removes all hardcoded backend IDs from `cli/run.go`, `cli/host.go`, and `launcher/launcher.go`.

---

### 2.2 Scene-Driven Hit-Testing & KSL Band Helper

#### 2.2.1 Scene-Frame Hit-Testing (`Frame.Contains(x, y)`)
- Eliminates `BandContains(x, y)` and hardcoded coordinates in `internal/render/render.go`.
- During frame evaluation in `internal/ksl/eval`, `Resolve()` constructs the layout tree in `eval.Frame`.
- `Frame.Contains(x, y)` evaluates whether `(x, y)` intersects any rendered top-level window, box, or path:
  - **Inside:** Click hits launcher UI -> requests keyboard focus grab.
  - **Outside:** Click falls on clear transparent background -> calls `launcher.Close()` to dismiss.
- Supports any scene layout (band, dropdown, modal, sidebar, dock) without Go code changes.

#### 2.2.2 KSL Band Geometry Helper (`band(...)`)
- The Go engine core ceases to own `Reveal` and `Resample`.
- Instead, KSL provides a built-in function `band(...)` available in expressions:
  ```ksl
  theme {
      band_height: 290
      min_tilt: 5.0
      center_spread: 0.10
      taper_max: 0.15
      shear_min: 6
      shear_max: 14
  }

  let reveal = band($band_height, $min_tilt, $center_spread, $taper_max, $shear_min, $shear_max)

  Window {
      Path { d: $reveal.d, fill: $slab }
      Group#content {
          x: $reveal.frameX, y: $reveal.frameY
          width: $reveal.cw, height: $reveal.ch
          rotate: $reveal.angle
          ...
      }
  }
  ```
- Scenes that do not use a slanted band (such as `dropdown.ksl`) omit `band(...)` completely.

---

### 2.3 KSL Theme Variables & `config.toml` Cascading

#### 2.3.1 3-Tier Variable Cascading
```
 ┌──────────────────────────────────────────────────────────────┐
 │ Tier 1: Built-in Engine Fallbacks                            │
 │ (Default palette: slab, ink, accent, muted, display font)    │
 └──────────────────────────────┬───────────────────────────────┘
                                │ overridden by
 ┌──────────────────────────────▼───────────────────────────────┐
 │ Tier 2: KSL Scene Declarations (`theme { ... }`)             │
 │ (Scene defaults: colors, dimensions, layout knobs)           │
 └──────────────────────────────┬───────────────────────────────┘
                                │ overridden by
 ┌──────────────────────────────▼───────────────────────────────┐
 │ Tier 3: User Configuration (`config.toml [theme]`)           │
 │ (User custom colors, overridden dimensions, layout tweaks)   │
 └──────────────────────────────────────────────────────────────┘
```

1. **KSL `theme { ... }` block:**
   Allows declaring variables of any value type:
   - Colors: `accent: #FF2E63`
   - Numbers & dimensions: `band_height: 290`, `card_width: 360`, `min_tilt: 5.0`
   - Strings & bools
   Referenced anywhere in the scene as `$varname`.

2. **User `config.toml [theme]` Table:**
   Replaces the legacy `[reveal]` table in `config.Config`:
   ```toml
   [theme]
   accent = "#08F7FE"
   slab = "#000000"
   band_height = 320
   min_tilt = 8.0
   card_width = 400
   ```
   Keys in `[theme]` override KSL scene variables of the same name. Unused keys are ignored gracefully.

---

## 3. Data Structures & Schema Updates

### 3.1 `internal/config/config.go`
- **Removed:** `type Reveal struct` with its 14 hardcoded fields.
- **Added:** `Theme map[string]any` on `type Config struct`.

### 3.2 `internal/ksl/eval/eval.go`
- `eval.State` receives `Vars map[string]any` representing the merged Tier 1 + Tier 2 + Tier 3 variable map.
- `eval.Frame` implements `Contains(x, y float64) bool`.

---

## 4. Verification & Testing Strategy

1. **Unit Tests:**
   - Provider registry: dynamic mode generation, searchability filtering, zero hardcoded backend IDs.
   - KSL theme variables: evaluation of numbers, dimensions, colors, and `config.toml` override precedence.
   - Scene-frame hit-testing: verification of `Frame.Contains(x, y)` for both quads/paths and simple boxes.
   - Built-in `band(...)` helper: geometry calculation parity with previous `Resample`.
2. **Integration Verification:**
   - Launching with `default.ksl` (band) vs `dropdown.ksl` (centered window) without Go code changes.
   - Applying `[theme]` overrides in `config.toml` and verifying visual layout changes.
   - Testing all in-process and subprocess plugins seamlessly through the unified registry.
