# Keyshell Scene Language (KSL) Specification & Reference

**KSL** (Keyshell Scene Language) is a declarative styling and scene graph language designed for **keyshell**, a GPU-accelerated Wayland launcher overlay built with pure Go.

KSL unifies layout, reactive state binding, vector styling, transitions, and keyframe animations into a clean, CSS/QML-inspired declarative syntax. Scenes are parsed, validated, and evaluated against live host state on every frame, repainting seamlessly via `gogpu/gg` affine transforms.

---

## Table of Contents

1. [Architecture & Design Philosophy](#1-architecture--design-philosophy)
2. [Document Structure & Top-Level Declarations](#2-document-structure--top-level-declarations)
   - [Window Root Node](#window-root-node)
   - [Theme Blocks](#theme-blocks)
   - [Reusable Components (`def`)](#reusable-components-def)
   - [Includes (`include`)](#includes-include)
   - [States & Transitions](#states--transitions)
   - [Keyframes (`@keyframes`)](#keyframes-keyframes)
3. [Primitive Nodes Reference](#3-primitive-nodes-reference)
   - [Containers: Window, Box, Row, Column, Grid, Stack, Group, List](#containers)
   - [Visual Leaves: Text, Icon, Image, Pill, Meter, Cursor, Path, Card](#visual-leaves)
4. [Layout Model & Sizing](#4-layout-model--sizing)
   - [Length Units & Sizing Properties](#length-units--sizing-properties)
   - [Flexbox (`Row` & `Column`)](#flexbox-row--column)
   - [Grid Layout (`Grid`)](#grid-layout-grid)
   - [Stack Layout (`Stack`)](#stack-layout-stack)
   - [Transforms & Conditional Rendering (`Group`, `when`)](#transforms--conditional-rendering)
5. [Expressions, Operators & Built-in Functions](#5-expressions-operators--built-in-functions)
   - [Data Types & Literals](#data-types--literals)
   - [Operators & Precedence](#operators--precedence)
   - [Methods](#methods)
   - [Built-in Functions (`rand`, `jitter`, effects)](#built-in-functions)
6. [Reactive Data Bindings](#6-reactive-data-bindings)
   - [Global Launcher Bindings](#global-launcher-bindings)
   - [Reveal & Band Geometry (`$reveal`)](#reveal--band-geometry-reveal)
   - [Control Panel Bindings (`$panel`)](#control-panel-bindings-panel)
   - [Item Bindings inside Lists (`$item`)](#item-bindings-inside-lists-item)
7. [List Viewports & Virtualization](#7-list-viewports--virtualization)
8. [Case Study 1: The Default Slanted Band Scene](#8-case-study-1-the-default-slanted-band-scene)
9. [Case Study 2: The Modern Centered Dropdown Scene](#9-case-study-2-the-modern-centered-dropdown-scene)

---

## 1. Architecture & Design Philosophy

Traditional UI toolkits (like Qt or GTK) impose heavy runtime dependencies, complex event loops, and large memory footprints. Keyshell achieves sub-millisecond query response and high frame rates through:

- **Decoupled State Machine**: Host logic (backend queries, subprocess plugin management, ranking, frecency) lives in pure Go. KSL only describes how this state is visually mapped to screen rectangles and shapes.
- **Immediate Resolution & Layout**: The KSL AST resolves against a snapshot of host state each frame (`eval.Resolve`). Nodes compute their bounds via flexbox, grid, or stack constraints, clamp text with ellipsis, and stream directly into painters.
- **Affine Vector Pipeline**: The underlying renderer (`gogpu/gg`) executes affine transforms (`Rotate`, `Shear`, `Translate`, `Scale`, `ClipRect`) directly on vector geometry, enabling complex angled and animated visual designs without GPU shader boilerplate.
- **Hot Reloading**: During development (`keyshell run --scene my.ksl --watch`), file changes trigger instantaneous re-parsing and re-rendering without losing launcher state or active query text.

---

## 2. Document Structure & Top-Level Declarations

A KSL file (`.ksl`) consists of top-level declarations: exactly one root `Window` node, optional `theme` blocks, reusable component definitions (`def`), external file includes (`include`), state blocks (`state`), transitions (`transition`), and animation keyframes (`@keyframes`).

Comments start with `//` and extend to the end of the line.

```ksl
// Example document skeleton
include "tokens.ksl"

theme {
    tokens {
        ink: #F0F6FC
        muted: #8B949E
        accent: #58A6FF
        slab: #161B22EE
    }
}

@keyframes blink {
    0% { opacity: 1 }
    50% { opacity: 0 }
    100% { opacity: 1 }
}

def ActionButton {
    Pill {
        width: 120, height: 32, radius: 4
        text: $item.title, fill: $slab, color: $ink
        state selected { fill: $accent, color: #000000 }
    }
}

Window {
    Box {
        width: 600, height: 400
        fill: $slab
        // ... child nodes
    }
}
```

### Window Root Node

Every KSL scene must have **exactly one** root `Window` node. It represents the Wayland surface boundary.
- **Dimensions**: Sized automatically by the compositor / layer-shell configuration. Inside KSL, `$width` and `$height` expose the current window dimensions.
- **Special Properties**:
  - `mask: band`: Applies keyshell's trapezoidal alpha stencil mask to the window surface, enabling angled band presentations.

### Theme Blocks

Theme blocks declare design tokens (colours), typography aliases, and asset directories:

```ksl
theme {
    tokens {
        ink: #EEEEEE
        muted: #777777
        accent: #3388FF
        slab: #111116EE
        surface-alt: #22222B
    }
    fonts {
        display: "RedHatDisplay-BlackItalic.otf"
        body: "Inter-Regular.ttf"
    }
}
```

Any token declared in `tokens` is accessible as a binding everywhere in the file: `$ink`, `$accent`, `$surface-alt`.

### Reusable Components (`def`)

The `def` keyword defines reusable node trees that can be instantiated by name throughout the scene:

```ksl
def Badge {
    Pill {
        radius: 4
        text: $item.badge
        tone: accent
    }
}

// In your layout:
Row {
    Badge {}
}
```

Components may define child nodes, properties, and internal `state` blocks.

### Includes (`include`)

Scenes can be split across multiple files using `include`:

```ksl
include "palette.ksl"
include "components/buttons.ksl"
```

The path is resolved relative to the directory of the file containing the `include` directive. The file watcher recursively monitors all included files when `--watch` is active.

### States & Transitions

Pseudo-states allow elements to react to dynamic changes. Supported built-in states are:
- `open`: Overlay has finished its summon animation and is visible.
- `closed`: Overlay is hidden or dismissed.
- `focused`: Node has user input focus.
- `selected`: Item is currently highlighted or active in a list.
- `searching`: A search query is currently resolving asynchronously.

States can be scoped to a single node:
```ksl
Box {
    border: $muted
    state selected {
        border: $accent
        fill: $surface-alt
    }
}
```

Or declared at the scene level targeting specific selectors:
```ksl
state open {
    Group#content { opacity: 1 }
}

transition open {
    rotate: 250ms ease-out,
    opacity: 180ms
}
```

### Keyframes (`@keyframes`)

Animations declare timeline percentages with property stops:

```ksl
@keyframes spin {
    0%   { rotate: 0deg }
    100% { rotate: 360deg }
}

@keyframes blink {
    0%   { opacity: 1 }
    50%  { opacity: 0 }
    100% { opacity: 1 }
}
```

Apply keyframe animations to nodes using the `animation:` property shorthand:
```ksl
Cursor {
    width: 2, height: 20, color: $accent
    animation: blink 800ms linear infinite
}
```

Syntax: `animation: <name> <duration> [easing] [iteration-count] [direction]`
- **iteration-count**: numeric (e.g. `3`) or `infinite`.
- **direction**: `alternate`, `reverse`, or `alternate reverse`.

---

## 3. Primitive Nodes Reference

KSL includes 15 built-in node primitives, divided into layout containers and visual leaves.

### Containers

| Node | Purpose | Key Properties |
| --- | --- | --- |
| `Window` | Top-level surface boundary. | `mask` |
| `Box` | Styled rectangular container or divider. | `width`, `height`, `x`, `y`, `fill`, `border`, `border-width`, `radius`, `shadow`, `blur`, `opacity`, `clip`, `padding`, `z`, `when` |
| `Row` | Horizontal flex container. | `width`, `height`, `gap`, `align`, `justify`, `when` |
| `Column` | Vertical flex container. | `width`, `height`, `gap`, `align`, `justify`, `when` |
| `Grid` | Uniform multi-column grid container. | `cols`, `gap`, `width`, `height`, `when` |
| `Stack` | Layered overlay stack (z-index ordered). | `align`, `width`, `height`, `when` |
| `Group` | Affine transform group. | `x`, `y`, `width`, `height`, `rotate`, `scale`, `opacity`, `when` |
| `List` | Dynamic virtualized list of items. | `items`, `selected`, `orientation`, `gap`, `cap`, `scroll`, `overflow` |

### Visual Leaves

| Node | Purpose | Key Properties |
| --- | --- | --- |
| `Text` | Vector text rendered via FreeType/gg with ellipsis truncation. | `text`, `size`, `color`, `weight`, `opacity`, `uppercase`, `align`, `tracking`, `maxW` |
| `Icon` / `Image` | Renders desktop icons, custom images, or vector glyphs. | `icon`, `glyph`, `image`, `src`, `width`, `height`, `size`, `color`, `fit`, `radius`, `tint` |
| `Pill` | Styled label badge capsule. | `text`, `tone`, `size`, `color`, `fill`, `radius` |
| `Meter` | Linear bar or range meter gauge. | `ratio`, `value`, `min`, `max`, `step`, `variant`, `track`, `bar`, `height`, `radius` |
| `Cursor` | Input caret cursor with blink support. | `width`, `height`, `color`, `opacity`, `animation` |
| `Path` | SVG path vector polygon or bezier contour. | `d`, `fill`, `stroke`, `stroke-width`, `opacity`, `transform` |
| `Card` | Keyshell composite result card renderer. | Inherits result metadata and secondary controls |

---

## 4. Layout Model & Sizing

### Length Units & Sizing Properties

- **Pixels**: Numeric values are in device pixels (e.g. `width: 640`, `gap: 12`, `height: 44px`).
- **Percentages**: `%` units resolve relative to the Window dimensions (e.g. `width: 80%` resolves to `0.80 * $width`).
- **Angles**: `deg` units used for rotations (e.g. `rotate: 5deg`, `rotate: 360deg`).
- **Durations**: `ms` or `s` for animations (e.g. `800ms`, `0.25s`).
- **Constraints**:
  - `minW`, `maxW`: Minimum and maximum horizontal bounds.
  - `minH`, `maxH`: Minimum and maximum vertical bounds.
  - `aspect`: Enforces width-to-height aspect ratio (`height = width / aspect`).
- **Intrinsic Sizing**:
  - `Text`: Computes width and height from font metrics and string length. If constrained by `maxW` or the parent width, painter automatically truncates overflow with `...` (ellipsis).
  - `Pill`: Automatically sizes to label length + horizontal padding (16px).
  - `Cursor`: Defaults to 3px × 24px (scaled).

### Flexbox (`Row` & `Column`)

`Row` (horizontal) and `Column` (vertical) lay out children sequentially:

```ksl
Row {
    width: 600, height: 50
    gap: 12
    align: center      // cross-axis alignment: start | center | end
    justify: center    // main-axis distribution: start | center | end

    Icon { width: 24, height: 24, icon: "search" }
    Text { text: $query, size: 16 }
    Box  { grow: 1 }   // absorbs leftover space
    Text { text: $results.len, size: 12, color: $muted }
}
```

- **`grow: <number>`**: Proportional distribution of available space. Multiple children with `grow` share the leftover space based on their relative weights.
- **`gap: <number>`**: Spacing placed between adjacent children.

### Grid Layout (`Grid`)

`Grid` arranges children in a uniform row-major grid:

```ksl
Grid {
    cols: 3
    gap: 8
    width: 600, height: 300
    // Child nodes fill cells automatically
}
```

### Stack Layout (`Stack`)

`Stack` layers children on top of each other in declaration order or via the `z` property:

```ksl
Stack {
    width: 400, height: 200
    align: center      // center | end | start

    Box { width: 400, height: 200, fill: $slab, radius: 8 }
    Text { text: "Centered on Box", size: 16, z: 2 }
}
```

### Transforms & Conditional Rendering

- **`Group`**: Groups child elements under a combined affine transform:
  ```ksl
  Group#panel {
      x: 100, y: 200
      width: 360, height: 180
      rotate: $reveal.angle
      when: $panel.open
      // Child elements transform together
  }
  ```
- **`when: <boolean-expression>`**: Conditional rendering. When the expression evaluates to `false`, the node and all its children are skipped during resolution, taking no space and emitting no render calls.

---

## 5. Expressions, Operators & Built-in Functions

### Data Types & Literals

- **Numbers**: `42`, `3.14`, `-10`
- **Lengths / Angles / Times**: `12px`, `50%`, `12deg`, `250ms`, `1.5s`
- **Colours**: Hex literals `#RGB`, `#RGBA`, `#RRGGBB`, `#RRGGBBAA` (e.g. `#58A6FF`, `#161B22EE`)
- **Strings**: Double-quoted strings `"Search apps..."`, supporting escape characters
- **Booleans**: `true`, `false`
- **Keywords**: Identifiers like `center`, `start`, `end`, `accent`, `bar`
- **Arrays**: `[ "ghostty", "alacritty" ]`
- **Object Inline Blocks**: `{ height: 320px }`

### Operators & Precedence

1. **Unary**: `-` (negate), `!` (logical NOT)
2. **Multiplicative**: `*`, `/`
3. **Additive**: `+`, `-` (also string concatenation: `"Count: " + $results.len`)
4. **Relational**: `<`, `<=`, `>`, `>=`
5. **Equality**: `==`, `!=`
6. **Logical AND**: `&&`
7. **Logical OR**: `||`
8. **Ternary**: `condition ? thenValue : elseValue`

### Methods

- **Strings**:
  - `$text.toUpper()`: Uppercase string conversion
  - `$text.toLower()`: Lowercase string conversion
  - `$text.len()`: Character count
- **Numbers**:
  - `$num.round()`: Rounds float to nearest integer (e.g. `($item.ratio * 100).round()`)

### Built-in Functions

- **`rand(min, max)`**: Samples a random float in `[min, max]`. **Draws are cached and frozen per summon epoch**, meaning values stay stable during frame rendering and animations, changing only when the launcher is re-opened.
- **`jitter(key, min, max)`**: Hashes a unique string key into a stable float between `min` and `max`. Ideal for procedural variation across item lists.
- **Effects**:
  - `blur(radius)`: Gaussian blur effect (e.g. `blur: blur(8px)`)
  - `shadow(x, y, blur, color)`: Drop shadow effect (e.g. `shadow: shadow(0, 4, 12, #00000088)`)
  - `tilt(deg)`: Skew/tilt transform

---

## 6. Reactive Data Bindings

All variables prefixed with `$` bind to live keyshell state:

### Global Launcher Bindings

| Binding | Type | Description |
| --- | --- | --- |
| `$query` | `string` | The current text typed into the search bar. |
| `$mode` | `string` | Current mode (`"apps"`, `"files"`, `"windows"`, `"calc"`, `"run"`, or plugin name). |
| `$selected` | `number` | 0-indexed integer of the highlighted item in `$results`. |
| `$results` | `Results` | Matched results list. Access length with `$results.len`. |
| `$active` | `Item` or `false` | The currently selected `Item` object (or `false` if results are empty). |
| `$searching` | `boolean` | `true` while asynchronous backend/plugin queries are running. |
| `$typing` | `number` | Activity ratio (`0.0` to `1.0`), decays smoothly when typing pauses. |
| `$width` | `number` | Total window width in pixels. |
| `$height` | `number` | Total window height in pixels. |

### Reveal & Band Geometry (`$reveal`)

Keyshell's default aesthetic uses a procedurally tilted band across the display. `$reveal` exposes the geometric calculations:

| Binding | Description |
| --- | --- |
| `$reveal.frameX`, `$reveal.frameY` | Top-left coordinates of the rotated content frame. |
| `$reveal.cw`, `$reveal.ch` | Content frame width and height. |
| `$reveal.angle` | Tilt angle in radians / degrees (e.g. pass to `rotate: $reveal.angle`). |
| `$reveal.shear` | Band shear factor. |
| `$reveal.cardShear` | Shear factor applied to individual cards. |
| `$reveal.y`, `$reveal.h` | Vertical center and band height. |
| `$reveal.hLeft`, `$reveal.hRight` | Left and right heights of the trapezoid band. |
| `$reveal.d` | Precalculated SVG path data for the band trapezoid polygon. |

### Control Panel Bindings (`$panel`)

When an item provides secondary controls (e.g. an audio sink with volume, mute, and port selector), keyshell's control panel opens:

| Binding | Type | Description |
| --- | --- | --- |
| `$panel.open` | `boolean` | `true` when the control panel is active. |
| `$panel.focus` | `number` | 0-indexed row currently focused in the panel. |
| `$panel.dirty` | `boolean` | `true` if text is currently being edited in a text row. |
| `$panel.rows` | `Results` | List of control rows materialized as `Item` objects. |

### Item Bindings inside Lists (`$item`)

Inside a `List` template, `$item` represents each element:

- **Standard Search Result Fields**:
  - `$item.id`: Unique identifier
  - `$item.title`: Primary title string
  - `$item.subtitle`: Description or secondary path
  - `$item.badge`: Mode or shortcut badge text
  - `$item.status`: Status text (e.g. "Default", "Connected")
  - `$item.statusTone`: Tone keyword (`"accent"`, `"warn"`, `"error"`, `"neutral"`)
  - `$item.icon`: XDG desktop icon name or file path
  - `$item.glyph`: Semantic glyph keyword (`"command"`, `"power"`, `"media"`, `"file"`, `"window"`, `"tray"`) or Unicode character (`"▶"`, `"⚙"`)
  - `$item.image`: Image preview URL or path
- **Inline Control Row Fields** (when `$item.kind != ""`):
  - `$item.kind`: Row type: `"action"`, `"range"`, `"toggle"`, `"choice"`, `"text"`
  - `$item.ratio`: 0.0 to 1.0 progress ratio for range sliders
  - `$item.on`: Boolean toggle state
  - `$item.min`, `$item.max`, `$item.step`: Numeric bounds for range controls
  - `$item.text`: Current value text for choice and text controls
  - `$item.options`: Array of selectable choices

---

## 7. List Viewports & Virtualization

`List` is keyshell's high-performance scrolling container:

```ksl
List {
    width: 624, height: 395
    items: $panel.open ? $panel.rows : $results
    selected: $panel.open ? $panel.focus : $selected
    orientation: vertical      // vertical | horizontal
    scroll: reveal             // reveal | center
    overflow: clamp            // clamp | cycle
    gap: 2
    cap: 60                    // maximum results evaluated

    template {
        Box {
            width: 624, height: 44, radius: 6
            state selected { fill: $surface-alt }
            // Item content...
        }
    }

    empty {
        Text {
            text: $searching ? "SEARCHING..." : "NO RESULTS"
            size: 13, color: $muted, align: center
        }
    }

    overflowSummary {
        Text { text: "+ more items...", size: 11, color: $muted }
    }
}
```

### Scrolling & Overflow Policies

- **`orientation`**:
  - `vertical` (default): Rows laid out top-to-bottom.
  - `horizontal`: Cards laid out left-to-right.
- **`scroll`**:
  - `reveal`: Keeps selection in view. When navigating with arrow keys, the viewport scrolls smoothly only when selection reaches the edges.
  - `center`: Always centers the selected item in the middle of the viewport.
- **`overflow`**:
  - `clamp`: Normal bounded list with edges clipped at 0 and end.
  - `cycle`: Infinite cyclical reel. Scrolling past the end wraps seamlessly back to the beginning.
- **Virtualization**:
  - When `items` contains more than 20 items and uses `reveal` or `center`, KSL **automatically virtualizes the viewport**. Only visible rows are resolved into the AST, eliminating allocations and maintaining 60+ FPS even with hundreds of results.

---

## 8. Case Study 1: The Default Slanted Band Scene

Located at [`internal/render/default.ksl`](file:///home/darkliquid/Projects/keyshell/internal/render/default.ksl), keyshell's default theme showcases complex affine math, angled vector bands, animated spinners, and detached floating control panels.

```ksl
// Snippet from internal/render/default.ksl
Window {
    mask: band
    Path { d: $reveal.d, fill: $slab }

    Group#content {
        x: $reveal.frameX, y: $reveal.frameY
        width: $reveal.cw, height: $reveal.ch
        rotate: $reveal.angle

        // Top Search Bar
        Row {
            x: 0, y: 12
            width: $reveal.cw, height: 48
            justify: center
            Row {
                width: $reveal.cw * 0.8, maxW: 1200, height: 48
                gap: 12, align: center
                Pill { text: $mode, tone: accent }
                Row { gap: 2, align: center
                    Text { text: $query, size: 22, uppercase: true, color: $ink }
                    Cursor { width: 3, height: 24, color: $accent, animation: blink 800ms linear infinite }
                }
                Box { grow: 1 }
                Group#spinner { width: 20, height: 20, when: $searching,
                    animation: spin 900ms linear infinite
                    Box { width: 20, height: 20, radius: 10, border: $muted }
                    Box { width: 5, height: 5, radius: 2.5, x: 13, y: 2, fill: $accent }
                }
                Text { text: $results.len, size: 14, color: $muted, when: !$searching }
            }
        }

        // Horizontal Card Reel
        List {
            x: 0, y: 68
            width: $reveal.cw, height: $reveal.ch - 80
            items: $results, selected: $selected
            orientation: horizontal
            gap: 16
            scroll: center
            overflow: cycle
            template {
                Group#card { width: 360, height: $reveal.ch - 80, minH: 130
                    Card { }
                }
            }
            empty {
                Text { text: $searching ? "SEARCHING" : "NO RESULTS", size: 12, color: $muted,
                    width: $reveal.cw, align: center }
            }
        }
    }

    // Detached Control Panel Overlay
    Group#panel {
        x: $reveal.frameX + ($reveal.cw - 360) / 2
        y: $reveal.y + ($reveal.hLeft + $reveal.hRight) / 4 + 4
        width: 360
        height: $panel.rows.len * 46 - 6
        rotate: $reveal.angle
        when: $panel.open
        List {
            width: 360
            items: $panel.rows, selected: $panel.focus
            orientation: vertical
            gap: 6
            overflow: clamp
            template {
                Group#prow { width: 360, height: 40
                    RowAction   { when: $item.kind == "action" }
                    RowRange    { when: $item.kind == "range" }
                    RowToggle   { when: $item.kind == "toggle" }
                    RowChoice   { when: $item.kind == "choice" }
                    RowText     { when: $item.kind == "text" }
                }
            }
        }
    }
}
```

### Key Architectural Highlights in `default.ksl`
1. **Dynamic Trapezoid Stencil**: `mask: band` combined with `Path { d: $reveal.d, fill: $slab }` forms the iconic angled Wayland band across the screen.
2. **Rotated Coordinate Space**: `Group#content` rotates by `$reveal.angle` and translates to `$reveal.frameX, $reveal.frameY`. All children (search row, pills, cards) automatically align with the band's tilt without manual trigonometry.
3. **Spinner & Caret Animations**: `@keyframes spin` and `@keyframes blink` animate vector nodes smoothly.
4. **Cyclic Card Reel**: `orientation: horizontal, scroll: center, overflow: cycle` renders a continuous looping carousel centered on the active card.
5. **Floating Overlay Outside Mask**: `Group#panel` is positioned as a sibling of `Group#content` at the `Window` root level. This ensures it paints outside the band's alpha stencil mask, sliding smoothly out from underneath the band.

---

## 9. Case Study 2: The Modern Centered Dropdown Scene

Located at [`scenes/dropdown.ksl`](file:///home/darkliquid/Projects/keyshell/scenes/dropdown.ksl), this scene implements a classic centered Spotlight / Raycast modal launcher.

```ksl
// Dropdown theme: a classic centered popup launcher (Raycast / Spotlight style)
Window {
    Box {
        width: 640, height: 460
        x: ($width - 640) / 2, y: 120
        fill: $slab, radius: 8, border: $muted

        Column {
            width: 640, height: 460
            gap: 0, align: center

            // Search Header
            Row {
                width: 624, height: 52
                gap: 12, align: center

                Box { width: 4 }
                Pill { text: $panel.open ? "CONTROLS" : $mode, tone: accent }
                Row {
                    gap: 4, align: center
                    Text { text: $panel.open ? ($active ? $active.title : "Controls") : $query, size: 18, color: $ink, maxW: 420 }
                    Cursor { width: 2, height: 20, color: $accent, when: !$panel.open, animation: blink 800ms linear infinite }
                    Text { text: "(ESC to exit)", size: 12, color: $muted, when: $panel.open }
                }
                Box { grow: 1 }
                Text { text: $panel.open ? $panel.rows.len : $results.len, size: 12, color: $muted, when: !$searching }
                Box { width: 4 }
            }

            // Divider
            Box { width: 640, height: 1, fill: $surface-alt }

            // Top spacer
            Box { height: 6 }

            // Vertical Result List (inset with 8px margin from borders)
            List {
                width: 624, height: 395
                items: $panel.open ? $panel.rows : $results
                selected: $panel.open ? $panel.focus : $selected
                orientation: vertical
                scroll: reveal
                overflow: clamp
                gap: 2

                template {
                    Box {
                        width: 624, height: 44, radius: 6
                        state selected { fill: $surface-alt }

                        // Standard Search Item
                        Row {
                            when: $item.kind == ""
                            width: 624, height: 44, gap: 12, align: center

                            Box { width: 4 }
                            Icon { icon: $item.icon, glyph: $item.glyph, width: 24, height: 24 }
                            Column {
                                grow: 1, gap: 2, justify: center
                                Text { text: $item.title, size: 14, color: $ink }
                                Text { text: $item.subtitle, size: 11, color: $muted, when: $item.subtitle != "" }
                            }
                            Pill { text: $item.status, when: $item.status != "", tone: $item.statusTone }
                            Pill { text: $item.badge, when: $item.badge != "", tone: neutral }
                            Box { width: 4 }
                        }

                        // Inline Action Row
                        Row {
                            when: $item.kind == "action"
                            width: 624, height: 44, gap: 12, align: center
                            Box { width: 4 }
                            Icon { glyph: "▶", size: 14, color: $accent }
                            Text { text: $item.title, size: 14, color: $ink, grow: 1 }
                            Pill { text: "ACTION", tone: accent }
                            Box { width: 4 }
                        }

                        // Inline Range Row (Slider)
                        Row {
                            when: $item.kind == "range"
                            width: 624, height: 44, gap: 12, align: center
                            Box { width: 4 }
                            Box { width: 80, height: 24, radius: 4, fill: $surface-alt
                                Text { text: $item.title, size: 11, color: $muted, uppercase: true, align: center, maxW: 76 }
                            }
                            Meter { width: 430, height: 8, ratio: $item.ratio, variant: bar }
                            Box { grow: 1 }
                            Row { gap: 2, align: center
                                Text { text: ($item.ratio * 100).round(), size: 12, color: $muted }
                                Text { text: "%", size: 12, color: $muted }
                            }
                            Box { width: 4 }
                        }

                        // Inline Toggle Row
                        Row {
                            when: $item.kind == "toggle"
                            width: 624, height: 44, gap: 12, align: center
                            Box { width: 4 }
                            Box { width: 80, height: 24, radius: 4, fill: $surface-alt
                                Text { text: $item.title, size: 11, color: $muted, uppercase: true, align: center, maxW: 76 }
                            }
                            Box { width: 36, height: 18, radius: 9, fill: $item.on ? $accent : $surface-alt }
                            Text { text: $item.on ? "ON" : "OFF", size: 12, color: $item.on ? $accent : $muted }
                            Box { grow: 1 }
                            Pill { text: $item.on ? "ENABLED" : "DISABLED", tone: $item.on ? accent : neutral }
                            Box { width: 4 }
                        }

                        // Inline Choice Row
                        Row {
                            when: $item.kind == "choice"
                            width: 624, height: 44, gap: 12, align: center
                            Box { width: 4 }
                            Box { width: 80, height: 24, radius: 4, fill: $surface-alt
                                Text { text: $item.title, size: 11, color: $muted, uppercase: true, align: center, maxW: 76 }
                            }
                            Text { text: $item.text, size: 13, color: $ink, grow: 1 }
                            Pill { text: "CHOICE", tone: neutral }
                            Box { width: 4 }
                        }

                        // Inline Text Input Row
                        Row {
                            when: $item.kind == "text"
                            width: 624, height: 44, gap: 12, align: center
                            Box { width: 4 }
                            Box { width: 80, height: 24, radius: 4, fill: $surface-alt
                                Text { text: $item.title, size: 11, color: $muted, uppercase: true, align: center, maxW: 76 }
                            }
                            Box { width: 380, height: 28, radius: 4, fill: $surface-alt, border: $muted
                                Row { height: 28, gap: 2, align: center
                                    Box { width: 6 }
                                    Text { text: $item.text, size: 13, color: $ink, maxW: 340 }
                                    Cursor { width: 2, height: 18, color: $accent, opacity: $panel.dirty ? 1 : 0, animation: blink 800ms linear infinite }
                                }
                            }
                            Box { grow: 1 }
                            Box { width: 4 }
                        }
                    }
                }

                empty {
                    Box {
                        width: 624, height: 200
                        Text { text: $searching ? "SEARCHING..." : ($panel.open ? "NO CONTROLS" : "NO RESULTS"), size: 13, color: $muted, width: 624, height: 200, align: center }
                    }
                }
            }
        }
    }
}
```

### Key Architectural Highlights in `dropdown.ksl`
1. **Centered Modal Window**: Sized explicitly to `640x460` and centered horizontally using `x: ($width - 640) / 2, y: 120`.
2. **Unified Search & Control Transition**: Ternary bindings switch dynamically between normal search mode and active item controls:
   ```ksl
   items: $panel.open ? $panel.rows : $results
   selected: $panel.open ? $panel.focus : $selected
   ```
   When the user opens a control panel, the search field seamlessly displays the active card title with an `(ESC to exit)` prompt, and the vertical list swaps its contents from search results to interactive control rows.
3. **Pixel-Perfect Border Insets**: By sizing the inner `List` and rows to `624px` with `radius: 6` inside the `640px` window, selection backgrounds never collide with or overlap the modal border.
4. **Polymorphic Template Conditioning**: The single `template` block dispatches dynamically to standard search rows or specific control rows (`action`, `range`, `toggle`, `choice`, `text`) using `when: $item.kind == ...`.
5. **Robust Overflow Protection**: Automatic ellipsis text clipping (`Text.maxW` and container width limits) prevents lengthy titles, URLs, or command strings from breaking the layout.
