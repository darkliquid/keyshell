# KSL Card Decomposition & Dropdown UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable KSL to compose arbitrary item layouts (including classic compact vertical dropdown rows) with icon theme resolution, viewport clipping, and vertical list scrolling.

**Architecture:**
- Expose global surface dimensions (`$width`, `$height`) in the KSL evaluation environment.
- Add viewport clipping (`dc.Clip()`) to `List` in [`painters.go`](file:///home/darkliquid/Projects/keyshell/internal/render/painters.go).
- Implement vertical `scroll: reveal` offset calculations in [`resolve.go`](file:///home/darkliquid/Projects/keyshell/internal/ksl/eval/resolve.go).
- Wire XDG theme icon resolution and monogram fallback to `paintImage` in [`effects.go`](file:///home/darkliquid/Projects/keyshell/internal/render/effects.go).
- Build and verify a complete `scenes/dropdown.ksl` theme.

**Tech Stack:** Go 1.23+, `gg` (2D software canvas), `internal/ksl`, `internal/render`.

---

## File Structure

- Modify: `internal/ksl/eval/env.go` (Expose `$width` and `$height` to root scope)
- Test: `internal/ksl/eval/env_test.go`
- Modify: `internal/ksl/eval/resolve.go` (Implement vertical `scroll: reveal` in `resolveList`)
- Test: `internal/ksl/eval/resolve_test.go`
- Modify: `internal/render/painters.go` (Add `paintList` with viewport clipping)
- Test: `internal/render/painters_test.go`
- Modify: `internal/render/effects.go` (Add icon resolution and fallback to `paintImage`)
- Test: `internal/render/effects_test.go`
- Create: `scenes/dropdown.ksl` (Dropdown theme)
- Test: `internal/render/scene_test.go` (Dropdown scene integration test)

---

### Task 1: Expose `$width` and `$height` in KSL Environment

**Files:**
- Modify: `internal/ksl/eval/env.go:170-185`
- Test: `internal/ksl/eval/env_test.go`

- [ ] **Step 1: Write failing test in `env_test.go`**

Verify that `$width` and `$height` evaluate to the state's surface dimensions.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/ksl/eval -run TestEnvExposesWidthAndHeight`
Expected: FAIL

- [ ] **Step 3: Implement minimal change in `env.go`**

In `lookupRef(name string)`:
```go
case "width":
    return Num(e.st.Width), true
case "height":
    return Num(e.st.Height), true
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/ksl/eval -run TestEnvExposesWidthAndHeight`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/eval/env.go internal/ksl/eval/env_test.go
git commit -m "feat(ksl): expose \$width and \$height in evaluation environment"
```

---

### Task 2: Implement Viewport Clipping for `List` Container

**Files:**
- Modify: `internal/render/painters.go:28-50`
- Test: `internal/render/painters_test.go`

- [ ] **Step 1: Write failing test in `painters_test.go`**

Add `TestPaintListClipsChildrenToViewport`: paint a `List` with bounds `(10, 10, 100, 100)` containing a child `Box` spanning `(0, 0, 200, 200)` and verify no pixels are painted outside `(10, 10, 100, 100)`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/render -run TestPaintListClipsChildrenToViewport`
Expected: FAIL (pixels leak outside viewport)

- [ ] **Step 3: Implement `paintList` in `painters.go`**

```go
func paintList(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
    dc.Push()
    dc.DrawRectangle(n.Rect.X, n.Rect.Y, n.Rect.W, n.Rect.H)
    dc.Clip()
    paintChildren(dc, th, n, deps)
    dc.Pop()
}
```
Register `"List": paintList` in `painterFor`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/render -run TestPaintListClipsChildrenToViewport`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render/painters.go internal/render/painters_test.go
git commit -m "feat(render): clip List children to container viewport"
```

---

### Task 3: Implement Vertical `scroll: reveal` in `List` Evaluator

**Files:**
- Modify: `internal/ksl/eval/resolve.go:375-430`
- Test: `internal/ksl/eval/resolve_test.go`

- [ ] **Step 1: Write failing test in `resolve_test.go`**

Add `TestListScrollRevealVertical`: create a vertical list of 10 items with item height 40px (total 400px) in a viewport of 120px. Select item 0, item 5, item 9. Verify that in all cases the selected item rect is fully within the viewport bounds.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/ksl/eval -run TestListScrollRevealVertical`
Expected: FAIL (unhandled scroll mode)

- [ ] **Step 3: Implement vertical scrolling in `resolveList`**

In [`resolve.go`](file:///home/darkliquid/Projects/keyshell/internal/ksl/eval/resolve.go):
Support `scroll: reveal` (and default vertical scrolling):
Calculate `selNode := rn.Children[sel]` offset:
- If `selNode.Rect.Y < rn.Rect.Y`, shift content down by `rn.Rect.Y - selNode.Rect.Y`.
- If `selNode.Rect.Y + selNode.Rect.H > rn.Rect.Y + rn.Rect.H`, shift content up by `(rn.Rect.Y + rn.Rect.H) - (selNode.Rect.Y + selNode.Rect.H)`.
- Clamp shift between `0` and `-(contentH - viewportH)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/ksl/eval -run TestListScrollRevealVertical`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ksl/eval/resolve.go internal/ksl/eval/resolve_test.go
git commit -m "feat(ksl): implement vertical scroll: reveal in List evaluator"
```

---

### Task 4: Icon Theme Resolution & Monogram Fallback in `paintImage`

**Files:**
- Modify: `internal/render/effects.go:11-45`
- Test: `internal/render/effects_test.go`

- [ ] **Step 1: Write failing test in `effects_test.go`**

Test `paintImage` with a named icon (e.g. `"terminal"`) using `PainterDeps` containing a mock or real `icons.Resolver`. Verify image or monogram is rendered without leaving the slot blank.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/render -run TestPaintImageResolvesNamedIcons`
Expected: FAIL

- [ ] **Step 3: Implement icon lookup in `paintImage`**

In [`internal/render/effects.go`](file:///home/darkliquid/Projects/keyshell/internal/render/effects.go):
If `src` is an icon name, resolve through `deps.Icons.LookupAsync`. If ready, render with `deps.Images.Get`. If not ready or missing, draw monogram fallback with `dc.DrawStringAnchored`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/render -run TestPaintImageResolvesNamedIcons`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render/effects.go internal/render/effects_test.go
git commit -m "feat(render): resolve named icons and fallback monograms in paintImage"
```

---

### Task 5: Dropdown Scene Definition & Integration Test

**Files:**
- Create: `scenes/dropdown.ksl`
- Test: `internal/render/scene_test.go`

- [ ] **Step 1: Write integration test in `scene_test.go`**

Verify `scenes/dropdown.ksl` parses, validates, and renders a frame containing search query, mode pill, and decomposed item rows (icon, title, subtitle, status, badge).

- [ ] **Step 2: Create `scenes/dropdown.ksl`**

Write the complete classic dropdown theme with top search bar, divider, vertical scrollable list, and row highlight.

- [ ] **Step 3: Run integration test to verify it passes**

Run: `go test -v ./internal/render -run TestDropdownSceneRenders`
Expected: PASS

- [ ] **Step 4: Run full test suite**

Run: `go test -count=1 ./...`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add scenes/dropdown.ksl internal/render/scene_test.go
git commit -m "feat(render): add classic vertical dropdown theme and verify rendering"
```
