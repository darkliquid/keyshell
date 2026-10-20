# Keyshell UI language — a declarative scene language for custom overlays

Date: 2026-10-20
Status: design exploration, no implementation
Owner: keyshell

## Summary

The keyshell overlay is hard-coded: `internal/render` paints a fixed
band + search field + reel with pixel constants, and `internal/card`
only *selects* which of three hard-coded card bodies to draw. This spec
proposes a small **declarative scene language** ("KSL", Keyshell Scene
Language) that replaces the hard-coded scene with a user-authored
description of the entire overlay: primitives, layout, styling, states,
animation, and data bindings. The current UI becomes the *default scene*
shipped with keyshell; users can replace it wholesale.

Design goals, in priority order:

1. A user can describe a UI that looks nothing like the current overlay.
2. Authoring feels like writing a simplified CSS + a scene tree, not XML.
3. The language is small enough to learn in minutes and read at a glance.
4. Animation and state transitions are first-class and declarative.
5. It slots into the existing gg renderer without a GPU rewrite.
6. Themes are not limited to vector art: raster imagery, effects, and
   cross-tree reuse are first-class.

## Where this sits today

- `internal/render/render.go` draws the band (tilted / sheared
  trapezoid), then paints content in the band's rotated frame.
- `internal/render/card.go` draws a `card.Plan` inside each sheared
  quad: chips, visual, title/subtitle/detail, meters, ticks, footer.
- `internal/card` composes a `Plan` from item data (pure, no drawing).
- `internal/theme` holds the palette (`slab`, `ink`, `accent`,
  `accent-alt`, `muted`, `surface-alt`) and one font face.
- The renderer reads a `View` interface: `IsOpen`, `Query`, `Results`,
  `Selected`, `ModeBadge`, `Poll`.
- The reveal geometry (`Reveal`) randomises band tilt/shear per summon.

KSL generalises all of these: `theme` becomes a stylable concept, the
hard-coded draw calls become scene nodes, and the `View` interface
becomes the data-binding surface.

## Design space and prior art

| System | Form | Strengths | Weaknesses for this task |
| --- | --- | --- | --- |
| **SVG** | XML, vector scene graph | Arbitrary shapes, transforms, animation (SMIL/CSS) | XML verbosity; no tidy layout; user rejects XML |
| **CSS** | selector + declaration blocks | Familiar property syntax, cascading, transitions, keyframes | Needs a DOM/tree to hang off; not a scene language alone |
| **QML** | declarative object tree + JS | Property bindings, anchors, states, transitions; keyshell *was* ported from QML | Heavy; JS dependency; loading machinery |
| **SwiftUI / Compose** | code-builder DSL | Reactive, type-safe, animateable | Lives in a language; not user-editable config |
| **HTML/CSS** | markup + stylesheet | Huge ecosystem, flexbox | Markup is verbose; needs a full layout engine |
| **Godot .tscn** | text scene format | Tree with resources, transforms | Property table style; no styling cascade |

KSL takes the parts that fit a *config-file* use case: the **block
tree** of QML/Godot, the **property/declaration syntax and cascade** of
CSS, and **state + transition animation** of QML/CSS, with a layout
model reduced to Row/Column/Stack/Grid plus explicit anchors. No JS, no
XML, no external layout engine.

## Language overview

A KSL file is one scene. A scene is a tree of primitive nodes. Each
node is a block:

```ksl
Primitive {
    key: value
    Child {
        key: value
    }
}
```

- Node names are primitives (or user-defined components). A block body
  holds properties and child nodes.
- Properties are `key: value`, CSS-style. Values are numbers, sizes,
  colours, strings, keywords, theme references (`$accent`), data
  bindings (`$query`), cross-node references (`#id.prop`), or functions
  (`tilt(12deg)`, `blur(8px)`).
- Blocks may carry a `#id` for selectors, state targeting, and
  cross-node references.
- A scene may contain non-node declarations: `@keyframes`, `state`,
  `transition`, `theme`, `include`, and `def` components.

### Minimal example (full custom UI)

```ksl
// A wholly different overlay: a centred search ring.
Window {
    theme {
        ink: #EAEAEA
        ring: #FF2E63
    }

    Column { gap: 24, align: center
      Text { text: $query, size: 48, color: $ink }
      Ring { size: 120, stroke: $ring, progress: $typing }
      Text { text: "ENTER TO OPEN", size: 12, color: $muted }
    }

    state open {
        Ring { opacity: 1, scale: 1 }
    }
    transition open { opacity: 200ms ease-out }

    @keyframes breathe {
        0%  { stroke: $ring, opacity: 0.4 }
        100% { stroke: $accent-alt, opacity: 1 }
    }
    Ring { animation: breathe 2s ease-in-out infinite alternate }
}
```

This is deliberately unlike the shipped UI, to show the language is not
tied to the current look.

## Core concepts

### 1. Primitives

A complete vector + raster scene kit. Everything the current UI draws
maps onto these, and they cover arbitrary new UIs.

| Primitive | Role | Key properties |
| --- | --- | --- |
| `Window` | scene root; fills the surface, transparent outside | `width`, `height`, `scale` |
| `Box` | rectangle | `fill`, `border`, `radius`, `x`, `y`, `width`, `height`, `shear`, `opacity` |
| `Text` | a text run | `text`, `font`, `size`, `weight`, `color`, `align`, `wrap`, `elide`, `uppercase` |
| `Icon` | glyph or icon-theme art (no raster) | `glyph`, `icon`, `size`, `color` |
| `Image` | decoded raster bitmap over any rectangle | `source`, `fit`, `tint`, `radius`, `opacity` |
| `Pill` | text on a tinted plate (badge/status/chip) | `text`, `tone`, `size`, `border`, `radius` |
| `Meter` | ratio readout, bar or ticks | `ratio`, `ticks`, `variant`, `label`, `value`, `tone` |
| `Row` | horizontal flex container | `gap`, `align`, `justify`, `grow` |
| `Column` | vertical flex container | `gap`, `align`, `justify`, `grow` |
| `Grid` | tiling container (fixed columns) | `cols`, `gap`, `align`, `justify` |
| `Stack` | overlay container | `align`, `z` |
| `List` | data-bound repetition; a clipped viewport | `items`, `selected`, `template`, `orientation`, `viewport`, `scroll`, `overflow`, `cap`, `empty` |
| `Path` | low-level vector drawing (escape hatch) | `d` (move/line/quad/cubic/close) |
| `Group` | transform + layer container | `translate`, `rotate`, `shear`, `scale`, `opacity`, `z` |

Every node also accepts the universal properties of §3: `z`, visual
effects, and `when`.

Notes:

- `Icon` is glyph (`glyph`, literal text/emoji) or icon-theme (`icon`).
  Raster images belong to `Image`, not `Icon`.
- `Image` is the full raster surface; see §8.
- `Pill` is sugar over `Box` + `Text` with a tone-mapped fill, matching
  `drawChip`.
- `Meter` `variant: ticks` with `ticks: 10` reproduces the hold/volume/
  battery gauge; `variant: bar` reproduces the solid bar.
- `Path` is the guarantee that *any* shape is expressible, not a
  curated list. It is why a user can build a UI that looks nothing like
  the shipped one.
- `Grid` provides the tiling an icon browser needs; see §2.

### 2. Layout model

Layout is deliberately small.

- `Row` lays children left-to-right; `Column` top-to-bottom. Both are
  flex: children may be fixed-size or `grow: 1` to share leftover space.
  `gap` spaces them, `align`/`justify` position them on the cross/main
  axis.
- `Stack` overlays children: each child may pin via `align`, or use
  explicit `x`/`y`. Siblings may carry a `z` (see §3).
- `Grid` tiles children into `cols` columns, filling row by row, with a
  uniform `gap`. This is how an icon/thumbnail browser is laid out
  without hand-positioning.
- Any node can take explicit `x y width height` to opt out of flow.
- `Group` applies a geometric transform to everything inside: the band's
  `rotate` + card `shear` are just `Group { rotate: … }` nodes.

Constraints, applied after layout:

- `minW`, `maxW`, `minH`, `maxH` clamp a node's resolved size.
- `aspect` fixes a width/height ratio when one axis is derived.

Units and responsive scaling:

- Lengths accept `px` or `%` (of the `Window`). Percentages are the
  responsive lever: `width: 40%` tracks the surface.
- A per-theme `scale` factor on `Window` multiplies every geometric
  length and font size, so an entire scene constrains proportionally on
  smaller surfaces. It inherits through the tree.

### 3. Universal node properties

Three properties apply to every node.

**Z-index (`z`).** Overlap order normally follows declaration order. An
explicit `z` (number, default 0) overrides it: within a sibling group,
nodes paint in ascending `z`, stable on ties (declaration order). A
full-screen background is an `Image`/`Box` at the scene root declared
first, or at lower `z` than the chrome.

**Visual effects.** A node may declare effects applied after its
fill/content:

- `blur(radius)` — Gaussian blur of the node's own painted content.
- `shadow(dy, radius, color)` — drop shadow behind the node.
- `brightness(factor)`, `saturate(factor)` — colour adjustments.
- `linear-gradient(from, to[, stops…])`, `radial-gradient(…)` — paint a
  gradient instead of a flat fill.

Effects run on the GPU via the shader path and degrade gracefully
(identity) where unsupported. They are animateable like any geometry
property (e.g. a blur radius that eases on selection change).

**Conditional presence (`when`).** A boolean expression; when false the
node (and its subtree) is skipped. `when: $mode == "media"`, `when:
$results.len > 0`. This is the general data-driven show/hide mechanism,
wider than named states.

### 4. Styling and theming

- **Theme tokens** are named values referenced as `$token`. A `theme`
  block overrides them. The shipped theme exposes `$slab`, `$ink`,
  `$accent`, `$accent-alt`, `$muted`, `$surface-alt` (identical to
  today's palette, so the default scene reproduces the current look).
- **Font registry.** A `theme` block may define named families:

```ksl
theme {
    ink: #EAEAEA
    font {
        display: "RedHatDisplay"
        ui: "Inter"
    }
}
```

  `font: display` selects a family; `size` and `weight` refine it.
  Missing families fall back to the default face.
- **Assets.** A `theme` block may list bundled paths (`assets: [ "bg.png", "texture.jpg" ]`) so loaders pre-decode them off the frame
  thread into a cache/atlas (see §8).
- **Cascading styles** follow a CSS-lite selector. A declaration block
  with a selector applies to matching nodes:

```ksl
Box.selected { fill: $accent }
Text.muted   { color: $muted }
```

  Selectors match node type, `#id`, pseudo-state (`:selected`, `:open`,
  `:focused`), and attribute (`[mode="media"]`). Cascade order: node-local
  property > id selector > type/pseudo > theme default.

### 5. States and selectors

States are boolean conditions on the scene and drive both styling and
animation.

- **Built-in states** mirror the renderer's `View`:
  `open` (IsOpen), `focused` (query focused), `selected` (current list
  item), plus `mode` as an attribute.
- **Per-node states** apply a property block when the state holds:

```ksl
Box#card {
    fill: $slab
    state selected {
        fill: $accent
    }
}
```

- **Scene-level states** can restructure the scene (show/hide groups).

```ksl
state open {
    SearchField { opacity: 1 }
}
state closed {
    SearchField { opacity: 0 }
}
```

### 6. Animation

Two declarative mechanisms cover the current UI's animations and more.

- **Transitions** animate property changes between states:
  `transition open { opacity: 200ms ease-out }` (per-property duration
  and easing). Used for open/close, selection changes, hover.
- **Keyframes** drive time-based loops and one-shot sequences:

```ksl
@keyframes blink {
    0%  { opacity: 1 }
    50% { opacity: 0 }
    100%{ opacity: 1 }
}
Cursor { animation: blink 800ms linear infinite }
```

Properties eligible for animation: `opacity`, `translate`, `rotate`,
`shear`, `scale`, `fill`, `color`, `stroke`, `ratio`, `x`/`y`/
`width`/`height`, and the effect parameters (`blur` radius, `shadow`
offset/radius, `brightness`, `saturate`). Easing is a keyword
(`linear`, `ease`, `ease-in`, `ease-out`, `ease-in-out`) or a
cubic-bezier function.

The reveal-on-open animation is a scene-level transition on the band
`Group`'s `rotate`/`translate` from a closed to an open state, replacing
`Reveal`'s hard-coded geometry with a declarative one.

### 7. Data binding and reactivity

The scene reads the launcher's state through bound variables, items flow
into `List` templates, and any node can reach the selection.

- **Scene variables**: `$open`, `$focused`, `$query`, `$results`,
  `$selected`, `$mode`, `$typing` (placeholder for a typing/activity
  signal). A binding re-evaluates when its source changes and schedules
  a repaint (the renderer already redraws on `OnChange`).
- **List items**: inside a `List`, `$item` exposes the item's fields
  (`title`, `subtitle`, `badge`, `status`, `visual`, `glyph`, `icon`,
  `detail`, `hint`, and its controls). A `List` template renders one
  sub-tree per item; `selected` highlights the current index.
- **$active**: a top-level binding to the currently selected item,
  available anywhere in the tree. This is the hoist mechanism — a
  full-screen background can do `background { image: $active.image }`
  without living inside the `List` template.
- **Cross-node references**: `prop: #someId.prop` resolves a value
  already computed elsewhere, so nodes can reuse loaded assets or styled
  values without duplicating them.
- **Expressions**: a minimal expression language for computed values —
  arithmetic, string concatenation, and a ternary — deliberately *not*
  general scripting. `text: $item.title.toUpper()`, `width: $results.len * 12`.

Plugins/backends may publish extra read-only variables and functions
through the existing plugin host, so custom UIs can bind to plugin data
without the language growing a scripting engine.

### 8. Imagery and raster

`Image` is the raster surface.

- **Source resolution**: `source` is a local path, resolved against the
  theme bundle directory, then the config directory, then absolute.
  Remote URLs are **not** supported in v1 (see Out of scope).
- **Formats**: PNG, JPEG, WebP (static). Animated formats are deferred.
- **Loading**: decoded asynchronously off the frame thread, cached keyed
  by path (and target size). Until loaded, the node paints a theme
  placeholder so nothing flashes.
- **Global cache**: images are cached scene-wide, so a background that
  references the same path as a card reuses the card's decoded texture —
  no double decode. Cross-node references (`#id`) let a node reuse the
  same resolved image explicitly.
- **Fit and tint**: `fit` (`cover` | `contain` | `fill` | `tile`)
  controls how the bitmap fills the rectangle; `tint` multiplies its
  colour (useful for duotone/overlay effects); `radius` clips corners.
- **Additive use**: an `Image` can be the scene root's background, a
  card's visual, or a layer inside a `Stack` — it is a plain node.

### 9. Viewports, scrolling, and list dynamics

Scrolling is a *capability of containers*, not a separate primitive. Any
container may clip its children to its bounds and scroll its content;
`List` is the primary consumer. Two concerns stay separate: **what is
visible** (the viewport) and **which items exist** (content and
capacity).

#### Clipping

- `clip: true|false` on any container. When true, children outside the
  container's rectangle are hidden, using the same row-span masking the
  band already uses. Default is `false` for `Row`/`Column`/`Grid`/`Stack` and
  `true` for `List` (a list is inherently a viewport).

Every scrollable viewport is a clipped container with a scroll offset.

#### The List viewport

```ksl
List {
    items: $results
    selected: $selected
    orientation: vertical          // vertical | horizontal
    clip: true                     // implied for List
    viewport: { height: 320px }    // or a fraction of the Window
    cap: 8                         // max items materialised (optional)
    scroll: reveal                 // reveal | center | none
    overflow: clamp                // clamp | cycle | page
    template { … }
}
```

- `orientation` picks the flow: `vertical` stacks items downward
  (dropdown style); `horizontal` lays them in a row (the current reel).
- `viewport` bounds the visible area: a height (vertical) or width
  (horizontal), in pixels or as a fraction of the `Window`. This is the
  constraint that stops a long list from running off the screen.

#### Item limits

- `cap` bounds how many items are materialised into the scene tree. It
  is a *content* bound, independent of the viewport, so a large result
  set does not build thousands of nodes.
- When `cap` hides items, an optional `overflowSummary` sub-template may
  render a "+N" tail (mirroring the existing card overflow marker);
  without it the capped list simply ends.

#### Selection tracking

When `selected` moves, the viewport keeps it visible according to
`scroll`:

- `reveal` - minimal scroll: if the selection moves above the viewport,
  it scrolls up to show it; below, it scrolls down just enough. This is
  classic dropdown/list behavior.
- `center` - keep the selected item centred in the viewport (today's
  reel feel).
- `none` - never auto-scroll; offset changes only via explicit `offset`
  bindings or input.

#### Boundary behavior (`overflow`)

What happens when navigation tries to pass the first or last item:

- `clamp` - selection stops at the ends (a bounded list; the dropdown
  default).
- `cycle` - selection wraps last to first and vice versa (the reel
  metaphor).
- `page` - multi-step navigation (PageUp/PageDown or repeated arrows)
  jumps by a viewport-sized page; edges still clamp.

An explicit `offset` binding wins over auto-scroll and can drive a
scrollbar or slider.

#### Dropdown example

A traditional dropdown launcher is a vertical, clipped, scrolling list:

```ksl
Column { gap: 8, align: center
    Box { width: 70%, height: 44px, fill: $surface-alt, radius: 4px
        Text { text: $query, size: 16, color: $ink }
    }
    List {
        items: $results, selected: $selected
        orientation: vertical
        viewport: { height: 300px }
        cap: 20
        scroll: reveal
        overflow: clamp
        template {
            Box { width: 70%, height: 44px, fill: $slab, border: $muted, radius: 4px
                state selected { fill: $accent, border: $accent }
                Text { text: $item.title, size: 14, color: $ink }
            }
        }
        empty {
            Text { text: "NO RESULTS", size: 12, color: $muted }
        }
    }
}
```

The same story with `orientation: horizontal`, `scroll: center`, and
`overflow: cycle` reproduces today's reel inside the band.

### 10. Extensibility and escape hatches

- `Path` covers arbitrary geometry.
- A plugin can contribute custom primitive *drawers* via the host
  (registered under a name usable as a node type), for visuals the core
  kit cannot express.
- A plugin may supply custom image loaders/decoders for bespoke formats.
- `def Component { … }` defines a reusable node (see §11); `Window`/
  `Group` layers mean a custom UI can be a full replacement, not an
  overlay on the default one.

### 11. Composition and reuse

- `include "file.ksl"` splices another scene fragment at that point,
  so shared chrome or card templates live in one place.
- `def Name { … }` defines a reusable component; instances appear as
  `Name { … }` nodes anywhere. This is how a card template is defined
  once and shared across `List` templates and states without copy-paste.

```ksl
def Card {
    Box { width: 100%, height: 44px, fill: $slab, radius: 4px
        state selected { fill: $accent }
        Text { text: $item.title, size: 14, color: $ink }
    }
}
```

## Reference: current UI written in KSL

A faithful, compact rendering of today's overlay shows the language is
expressive enough to be a drop-in description of the shipped look.

```ksl
Window {
    // The slab band: a Group with the reveal tilt.
    Group id=band {
        x: 0, y: $reveal.y, width: 100%, height: $reveal.h
        rotate: $reveal.angle
        shear: $reveal.shear
        fill: $slab
        // clipped to the band quad via the path mask
        Stack { align: center
            Column { gap: 8, align: center
                Row { gap: 12, align: center
                    Pill { text: $mode, tone: accent }
                    Text { text: $query, size: 22, uppercase: true, color: $ink }
                    Cursor { color: $accent, animation: blink 800ms linear infinite }
                    Text { text: $results.len, size: 14, color: $muted }
                }
                List { items: $results, selected: $selected, width: 60%, height: 40%
                    // card template drawn per item; selected pill inverts
                    Group {
                        shear: $reveal.cardShear
                        Box { fill: $slab, border: $muted }
                        state selected { fill: $accent, border: $accent }
                    }
                }
            }
        }
    }
    state open { Group#band { opacity: 1, rotate: 0deg } }
    transition open { rotate: 250ms ease-out, opacity: 180ms }
}
```

The `$reveal` variables replace `Reveal`'s per-summon geometry without
changing any behaviour.

## Grammar (informal EBNF)

```ebnf
scene     := ws ( node | decl | include | compdef )*
decl      := themeblock | stateblock | transitiondecl | keyframes
include   := 'include' string ws
compdef   := 'def' IDENT '{' ws ( prop | node | decl )* '}' ws
node      := IDENT ('#' IDENT)? '{' ws ( prop | node | decl )* '}' ws
prop      := IDENT ':' value ws
themedef  := 'theme' '{' ws ( IDENT ':' value ws | fontfamily | assets )* '}' ws
fontfamily := 'font' '{' ws ( IDENT ':' string ws )* '}' ws
assets    := 'assets' ':' arr ws
stateblock:= 'state' IDENT '{' ws ( prop | node )* '}' ws
transdecl  := 'transition' IDENT '{' ws ( IDENT ':' timing ws )* '}' ws
keyframes := '@keyframes' IDENT '{' ws ( frame )* '}' ws
frame     := ( '0%' | '100%' | INT '%' ) '{' ws ( prop )* '}' ws
value     := scalar | color | string | ref | refpath | func | effect | arr
ref       := '$' IDENT ( '.' IDENT )*
refpath   := '#' IDENT '.' IDENT
effect    := 'blur(' scalar ')' | 'shadow(' scalar ',' scalar ',' color ')' |
             'brightness(' scalar ')' | 'saturate(' scalar ')' |
             'linear-gradient(' color ',' color ')' |
             'radial-gradient(' color ',' color ')'
func      := IDENT '(' ( value ( ',' value )* )? ')'
scalar    := number ( 'px' | 'deg' | '%' )?
color     := '#' hex hex hex | '#' hex hex hex hex hex hex | IDENT
string    := '"' ... '"'
arr       := '[' value ( ',' value )* ']'
timing    := scalar ('ms' | 's') IDENT?          // duration + easing
```

## Integration with keyshell

- **Default scene**: ships `ui/default.ksl` reproducing today's look;
  the hard-coded draw calls in `internal/render` become the interpreter
  for that file. With no user config, behaviour is unchanged.
- **Config**: `~/.config/keyshell/ui.ksl` (or a path in the existing
  config) replaces the default scene.
- **Parser**: a small tokenizer + recursive-descent parser producing a
  scene AST. No new dependencies beyond the standard library.
- **Interpreter**: walks the AST each redraw against the gg context,
  mirroring today's draw calls. `card.Plan` composition remains; a
  `List` template for cards invokes the same `drawCard` logic.
- **Data**: the renderer's `View` interface becomes the source of
  `$query/$results/$selected/$open/$mode`; `Reveal` becomes `$reveal`
  bindings; theme tokens load from the current `theme.Theme` defaults.
- **Imagery**: an async image loader and a global texture cache sit
  behind `Image`; effects map onto the GPU shader path with a software
  fallback.
- **Animation clock**: a per-frame elapsed/eased evaluation, run on the
  existing frame thread.

## Key decisions (resolved)

1. **Syntax**: brace-blocks (`Primitive { … }`) over indentation-only.
   Braces survive reformatting, allow single-line nodes, and read like
   CSS/JS. Indentation is optional sugar.
2. **Wind**: no scripting engine. Bindings + expressions only. Complex
   logic belongs in a plugin (custom primitive or published variable/
   function).
3. **Path primitive**: include a low-level `Path` for arbitrary shapes.
4. **List/`$item` surface**: mirror the current item schema and grow it
   with the card-composition work, not a parallel one.
5. **Reveal**: fold `Reveal` geometry into `$reveal` scene variables and
   make open/close a transition.
6. **Scrolling**: declarative container policies (`clip`/`viewport`/
   `scroll`/`overflow`/`cap`). The reel is `horizontal`/`center`/
   `cycle`; a dropdown is `vertical`/`reveal`/`clamp`.
7. **Raster imagery**: split `Image` from `Icon`, with `fit`, async local
   loading, and no remote URLs in v1.
8. **Effects pipeline**: blur/shadow/gradients/brightness/saturate as
   animateable per-node effects, GPU-backed with graceful fallback.
9. **Z-index**: explicit `z` per node, stable sort within siblings.
10. **$active hoist**: expose the selected item outside List templates
    for cross-tree bindings, plus `#id.prop` cross-node references.
11. **Grid**: a `Grid` primitive for tiled/icon-browser layouts.
12. **Typography**: a themed font family registry with fallback.
13. **Responsive**: percentages of the Window plus a per-theme `scale`.
14. **Constraints**: `minW/maxW/minH/maxH/aspect`.
15. **Reuse**: `include` and `def` components.
16. **Conditional presence**: a boolean `when:` on any node.

## Out of scope (future)

- Full CSS grid/floating layout (the tiling `Grid` is included; named
  grid areas and auto-flow rules are not).
- A visual editor / live preview tooling.
- Hand-authored shaders; themed effects are a curated, declarative
  subset.
- Remote imagery URLs and animated image formats (PNG/JPEG/WebP static
  only in v1).
- Interactivity beyond keyboard navigation/selection (no mouse events
  beyond the click-hit-test that already exists) until an input need
  appears.
- Multiple monitors/windows; the overlay owns one surface.
