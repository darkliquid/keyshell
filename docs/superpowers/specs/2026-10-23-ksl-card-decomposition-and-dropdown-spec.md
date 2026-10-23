# Specification: KSL Card Decomposition & Dropdown UI (Option A)

**Status:** Approved for Implementation  
**Date:** 2026-10-23  
**Authors:** Google DeepMind pair programming assistant & Darkliquid  

---

## 1. Problem Statement & Motivation

Keyshell's layout language (KSL) currently delegates list item rendering to an opaque native Go primitive, `Card {}`. In [`default.ksl`](file:///home/darkliquid/Projects/keyshell/internal/render/default.ksl), a list of results is rendered as:
```ksl
List {
    items: $results, selected: $selected
    template {
        Group#card { width: 360, height: $reveal.ch - 80
            Card { }
        }
    }
}
```

While `Card {}` produces the slanted cyberpunk parallelogram cards, it prevents KSL from expressing any other visual presentation. Specifically, a user or theme author cannot:
1. Break down an item into custom, compact rows (e.g. 40px height with an icon, title, subtitle, and right-aligned badge).
2. Render theme-resolved icons via `Image { source: $item.icon }`.
3. Build a traditional vertical dropdown launcher (Spotlight / Raycast / Alfred / Rofi style).
4. Vertically scroll lists reliably because `List` does not clip its viewport, and `scroll: reveal` is currently an unhandled no-op in the evaluator.

---

## 2. Design Goals

1. **Card Decomposition**: Allow KSL templates to compose items directly from granular primitives (`Image`/`Icon`, `Text`, `Pill`, `Meter`, `Box`, `Row`, `Column`) using `$item` properties (`$item.title`, `$item.subtitle`, `$item.icon`, `$item.badge`, `$item.status`, etc.).
2. **Icon Theme Resolution in KSL**: Enable `Image` (and `Icon`) to accept icon names (e.g., `"firefox"`, `"network-wired"`) and resolve them through the theme icon resolver with async decoding and fallback monograms.
3. **Viewport Clipping**: Implement container clipping (`dc.Clip()`) in [`internal/render/painters.go`](file:///home/darkliquid/Projects/keyshell/internal/render/painters.go) so `List` items that overflow the visible bounds do not paint outside the list container.
4. **Vertical List Scrolling**: Complete the `scroll: reveal` (and vertical scroll clamping) implementation in [`internal/ksl/eval/resolve.go`](file:///home/darkliquid/Projects/keyshell/internal/ksl/eval/resolve.go) so navigating up/down in a vertical list smoothly keeps the selected item in the visible viewport.
5. **Global Dimension Variables**: Expose `$width` and `$height` in [`internal/ksl/eval/env.go`](file:///home/darkliquid/Projects/keyshell/internal/ksl/eval/env.go) so elements can easily center themselves (e.g. `x: ($width - 640) / 2`).
6. **Classic Dropdown Theme**: Provide a complete, fully functioning `dropdown.ksl` theme demonstrating a classic centered popup launcher with a top search bar, vertical scrollable item list, selection highlight, and category badges.

---

## 3. Detailed Architecture & Component Changes

### 3.1. Icon Name Resolution in `paintImage`

Currently, `paintImage` in [`internal/render/effects.go`](file:///home/darkliquid/Projects/keyshell/internal/render/effects.go) assumes `source` is an absolute file path. We will update `paintImage` (and register `Icon` as a known primitive):
- If `source` is not an absolute path (`!strings.HasPrefix(src, "/")`), route it through `deps.Icons.LookupAsync(src, size)`.
- If ready and found, load the image through `deps.Images.Get(resolvedPath, w, h)`.
- If not ready or missing, draw a monogram fallback with `th.Face(...)` so the visual slot is never blank.

### 3.2. Viewport Clipping in `paintList`

In [`internal/render/painters.go`](file:///home/darkliquid/Projects/keyshell/internal/render/painters.go):
- Add `paintList(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps)`:
  ```go
  func paintList(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
      dc.Push()
      dc.DrawRectangle(n.Rect.X, n.Rect.Y, n.Rect.W, n.Rect.H)
      dc.Clip()
      paintChildren(dc, th, n, deps)
      dc.Pop()
  }
  ```
- Register `paintList` in `painterFor("List")`.

### 3.3. `scroll: reveal` & Vertical List Positioning

In [`internal/ksl/eval/resolve.go`](file:///home/darkliquid/Projects/keyshell/internal/ksl/eval/resolve.go):
- In `resolveList`:
  - When `scroll: reveal` or vertical orientation is used without `center`:
  - Check if the selected item `rn.Children[sel]` extends above or below the viewport `[rn.Rect.Y, rn.Rect.Y + rn.Rect.H]`.
  - Shift `rn.Children` vertically by `shift` so that the selected row is completely visible within the viewport.
  - Clamp `shift` so content never overscrolls beyond `0` (top) and `-(contentH - viewportH)` (bottom).

### 3.4. Environment Additions

In [`internal/ksl/eval/env.go`](file:///home/darkliquid/Projects/keyshell/internal/ksl/eval/env.go):
- Add `case "width": return Num(e.st.Width), true`
- Add `case "height": return Num(e.st.Height), true`
- Add `Icon` to `knownNodes` in [`internal/ksl/validate.go`](file:///home/darkliquid/Projects/keyshell/internal/ksl/validate.go).

### 3.5. Dropdown Scene Implementation

Create a fully functional `scenes/dropdown.ksl`:
```ksl
Window {
    Box {
        width: 640, height: 460
        x: ($width - 640) / 2, y: 120
        fill: $slab, radius: 8, border: $muted

        Column {
            width: 640, height: 460
            gap: 0

            // Search Header
            Row {
                width: 640, height: 52
                gap: 12, align: center
                x: 16

                Pill { text: $mode, tone: accent }
                Text { text: $query, size: 18, color: $ink }
                Cursor { width: 2, height: 20, color: $accent, animation: blink 800ms linear infinite }
                Box { grow: 1 }
                Text { text: $results.len, size: 12, color: $muted, when: !$searching }
            }

            // Divider
            Box { width: 640, height: 1, fill: $surface-alt }

            // Vertical Result List
            List {
                width: 640, height: 407
                items: $results, selected: $selected
                orientation: vertical
                scroll: reveal
                overflow: clamp
                gap: 2

                template {
                    Box {
                        width: 640, height: 44, radius: 4
                        state selected { fill: $surface-alt }

                        Row {
                            width: 608, height: 44, gap: 12, align: center
                            x: 16

                            Image { source: $item.icon, width: 24, height: 24 }
                            Column {
                                gap: 2
                                Text { text: $item.title, size: 14, color: $ink }
                                Text { text: $item.subtitle, size: 11, color: $muted, when: $item.subtitle != "" }
                            }
                            Box { grow: 1 }
                            Pill { text: $item.status, when: $item.status != "", tone: $item.statusTone }
                            Pill { text: $item.badge, when: $item.badge != "", tone: neutral }
                        }
                    }
                }

                empty {
                    Box {
                        width: 640, height: 200, align: center
                        Text { text: $searching ? "SEARCHING..." : "NO RESULTS", size: 13, color: $muted }
                    }
                }
            }
        }
    }
}
```

---

## 4. Verification & Testing Strategy

1. **Unit Tests**:
   - `eval/env_test.go`: Verify `$width` and `$height` resolve correctly.
   - `eval/resolve_test.go`: Verify `scroll: reveal` shifts items correctly when selecting items past the bottom and top of the viewport.
   - `render/painters_test.go`: Verify `paintList` applies a clipping rectangle matching the node bounds.
   - `render/effects_test.go`: Verify `paintImage` resolves icon names asynchronously and paints fallback monograms.
2. **Integration Test**:
   - Parse, validate, and render `dropdown.ksl` in headless/test runner, verifying that the output pixmap contains the rendered text, pills, and icons.
