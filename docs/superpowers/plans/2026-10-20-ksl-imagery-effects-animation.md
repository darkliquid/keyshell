# KSL Imagery, Effects and Animation Clock Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the scene language's §3 (effects), §6 (animation) and §8 (imagery) real: an async image loader with a scene-wide cache, the curated effect set painted with graceful software fallbacks, and a frame clock driving transitions and keyframes — so the default scene's cursor blink and reveal/selection transitions run through the language instead of hard-coded time math.

**Architecture:** Three pieces, each independently testable. `internal/anim` is a pure package: easings (keywords + cubic-bezier), a tween interpolator, and a keyframe sampler, all driven by an injected `time.Time` so tests are deterministic. `internal/render/images.go` owns async decoding and the texture cache, populated off the frame thread and read by the `Image` painter with a placeholder until ready. Effect and animation evaluation happen inside the painters/resolve path: `FuncVal` effects from eval get painted by `internal/render/painters.go`, and `SceneRenderer` tracks state edges to run transitions.

**Tech Stack:** Go 1.27.2, stdlib + one new module `golang.org/x/image` (for `webp` static decode; PNG/JPEG come from stdlib `image`). No GPU shaders in v1: effects are painted software-side where cheap (`shadow`, `brightness`, `saturate`, gradients) and degrade to identity where they are not (`blur`).

**Spec:** `docs/superpowers/specs/2026-10-20-keyshell-ui-language-design.md` (§3 visual effects, §6 animation, §8 imagery); consumes the interpreter plan's `FuncVal`/`AnimVal`/`AnimStep` AST and the renderer plan's `SceneRenderer`/`PainterDeps`.

**Scope check:** This is the last plan of the KSL v1 stack. Out of scope: hand-authored shaders, animated image formats, remote URLs (spec §Out of scope).

## Global Constraints

- Pure Go, `CGO_ENABLED=0`. One new dependency is allowed: `golang.org/x/image` (WebP static decode only). No others.
- The frame budget is 60 Hz; image decoding and gradient rasterisation happen off the frame thread.
- Effects degrade gracefully (identity) where unsupported, per spec §3.
- Transitions animate the resolved property set; they never mutate the AST.
- Tests: `go test ./internal/anim/... ./internal/render/...`; suite `mise run test`; gate `mise run check`.
- Commits: `feat(anim):`, `feat(render):`, `test(...):`, first line under 72 chars.
- The cursor blink (`cursorAlpha`) is replaced by the `blink` keyframes in the default scene; the visual parity is pinned by a test.

## File Structure

| File | Responsibility |
| --- | --- |
| `internal/anim/easing.go` | Easing keywords + cubic-bezier solver |
| `internal/anim/tween.go` | Per-property interpolation of resolved values |
| `internal/anim/keyframe.go` | Keyframe sampling over time |
| `internal/anim/clock.go` | The frame clock: state edges and elapsed time |
| `internal/render/images.go` | Async image loader, texture cache, placeholder |
| `internal/render/effects.go` | Gradient/shadow/brightness/saturate painters; blur fallback |
| `internal/render/scene.go` | (modified) state-edge detection feeding transitions; animation sampling in painters |

Tasks are ordered so each commits a compiling, tested increment.

---

### Task 1: Easings — keywords and cubic-bezier

**Files:**
- Create: `internal/anim/easing.go`
- Test: `internal/anim/easing_test.go`

**Interfaces:**
- Consumes: nothing (stdlib `math`).
- Produces: `type Easing func(t float64) float64`, `func ParseEasing(k eval.Value) (Easing, error)` mapping `linear|ease|ease-in|ease-out|ease-in-out` and `cubic-bezier(x1,y1,x2,y2)` FuncVals to functions.

- [ ] **Step 1: Write the failing test**

```go
package anim

import "testing"

func TestEasingEndpoints(t *testing.T) {
	for _, name := range []string{"linear", "ease", "ease-in", "ease-out", "ease-in-out"} {
		e, err := ParseEasing(keyword(name))
		if err != nil { t.Fatalf("%s: %v", name, err) }
		if e(0) != 0 || e(1) != 1 {
			t.Errorf("%s endpoints = %v, %v", name, e(0), e(1))
		}
	}
	if got := must(ParseEasing(keyword("ease-in")))(0.5); got >= 0.5 {
		t.Errorf("ease-in(0.5) = %v, want < 0.5 (slow start)", got)
	}
	if got := must(ParseEasing(keyword("ease-out")))(0.5); got <= 0.5 {
		t.Errorf("ease-out(0.5) = %v, want > 0.5 (fast start)", got)
	}
}

func TestCubicBezier(t *testing.T) {
	// the standard ease-in-out curve
	e := must(ParseEasing(cubicBezier(0.42, 0, 0.58, 1)))
	if got := e(0.5); got < 0.45 || got > 0.55 {
		t.Errorf("cubic-bezier(0.42,0,0.58,1)(0.5) = %v, want ~0.5", got)
	}
	// monotonicity: easing never goes backwards on [0,1]
	prev := 0.0
	for i := 1; i <= 100; i++ {
		if v := e(float64(i) / 100); v < prev-1e-6 {
			t.Fatalf("easing went backwards at %d: %v < %v", i, v, prev)
		}
		prev = e(float64(i) / 100)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/anim/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write the implementation**

```go
// Package anim evaluates the KSL animation model: easings, tweens,
// keyframes and the frame clock. It is pure and clock-injected.
package anim

import (
	"fmt"
	"math"

	"github.com/darkliquid/keyshell/internal/ksl/eval"
)

// Easing maps t in [0,1] to an eased fraction.
type Easing func(t float64) float64

// ParseEasing maps an eval value (Keyword or cubic-bezier FuncCall) to
// an easing function.
func ParseEasing(v eval.Value) (Easing, error) {
	switch x := v.(type) {
	case eval.Kw:
		switch x {
		case "linear":
			return linear, nil
		case "ease":
			return cubicBezierEasing(0.25, 0.1, 0.25, 1), nil
		case "ease-in":
			return cubicBezierEasing(0.42, 0, 1, 1), nil
		case "ease-out":
			return cubicBezierEasing(0, 0, 0.58, 1), nil
		case "ease-in-out":
			return cubicBezierEasing(0.42, 0, 0.58, 1), nil
		}
		return nil, fmt.Errorf("unknown easing %q", x)
	case eval.FuncVal:
		if x.Name != "cubic-bezier" {
			return nil, fmt.Errorf("unsupported easing %q", x.Name)
		}
		if len(x.Args) != 4 {
			return nil, fmt.Errorf("cubic-bezier wants 4 numbers")
		}
		pts := make([]float64, 4)
		for i, a := range x.Args {
			n, ok := a.(eval.Num)
			if !ok {
				return nil, fmt.Errorf("cubic-bezier arg %d is %T, want number", i, a)
			}
			pts[i] = float64(n)
		}
		return cubicBezierEasing(pts[0], pts[1], pts[2], pts[3]), nil
	}
	return nil, fmt.Errorf("unsupported easing %T", v)
}

func linear(t float64) float64 { return t }

// cubicBezierEasing solves the cubic bezier by bisection on x(t)=t.
func cubicBezierEasing(x1, y1, x2, y2 float64) Easing {
	return func(t float64) float64 {
		if t <= 0 {
			return 0
		}
		if t >= 1 {
			return 1
		}
		lo, hi := 0.0, 1.0
		for i := 0; i < 32; i++ {
			mid := (lo + hi) / 2
			if bezX(mid, x1, x2) < t {
				lo = mid
			} else {
				hi = mid
			}
		}
		s := (lo + hi) / 2
		return bezY(s, y1, y2)
	}
}

func bezX(t, x1, x2 float64) float64 { return bez(t, 0, x1, x2, 1) }
func bezY(t, y1, y2 float64) float64 { return bez(t, 0, y1, y2, 1) }

func bez(t, a, b, c, d float64) float64 {
	u := 1 - t
	return u*u*u*a + 3*u*u*t*b + 3*u*t*t*c + t*t*t*d
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/anim/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/anim/easing.go internal/anim/easing_test.go
git commit -m "feat(anim): easing keywords and the cubic-bezier solver"
```

---

### Task 2: Tweens — per-property interpolation

**Files:**
- Create: `internal/anim/tween.go`
- Test: `internal/anim/tween_test.go`

**Interfaces:**
- Consumes: `Easing` (Task 1), `eval.Value` variants.
- Produces: `func Lerp(a, b eval.Value, t float64) (eval.Value, bool)` — interpolates where the type supports it (Num, Length with equal units, Color channel-wise, Bool/Str step at t<0.5), reports `false` for non-interpolatable pairs.

- [ ] **Step 1: Write the failing test**

```go
package anim

import "testing"

func TestLerp(t *testing.T) {
	tests := []struct {
		a, b  eval.Value
		t     float64
		want  eval.Value
		ok    bool
	}{
		{eval.Num(0), eval.Num(10), 0.5, eval.Num(5), true},
		{eval.Length{V: 0, Unit: "px"}, eval.Length{V: 100, Unit: "px"}, 0.25, eval.Length{V: 25, Unit: "px"}, true},
		{eval.Color{0, 0, 0, 0xFF}, eval.Color{0xFF, 0xFF, 0xFF, 0xFF}, 0.5, eval.Color{0x80, 0x80, 0x80, 0xFF}, true},
		{eval.Str("a"), eval.Str("b"), 0.4, eval.Str("a"), true},
		{eval.Str("a"), eval.Str("b"), 0.6, eval.Str("b"), true},
		{eval.Bool(false), eval.Bool(true), 0.9, eval.Bool(true), true},
		{eval.Num(1), eval.Str("x"), 0.5, nil, false},
		{eval.Length{V: 1, Unit: "px"}, eval.Length{V: 2, Unit: "deg"}, 0.5, nil, false},
	}
	for _, tt := range tests {
		got, ok := Lerp(tt.a, tt.b, tt.t)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("Lerp(%v, %v, %v) = %v, %v; want %v, %v", tt.a, tt.b, tt.t, got, ok, tt.want, tt.ok)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/anim/ -run TestLerp -v`
Expected: FAIL — `Lerp` undefined.

- [ ] **Step 3: Write the implementation**

```go
// Lerp interpolates two resolved values at t in [0,1]; it reports false
// for pairs that cannot mix.
func Lerp(a, b eval.Value, t float64) (eval.Value, bool) {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	switch av := a.(type) {
	case eval.Num:
		if bv, ok := b.(eval.Num); ok {
			return eval.Num(float64(av) + (float64(bv)-float64(av))*t), true
		}
	case eval.Length:
		if bv, ok := b.(eval.Length); ok && av.Unit == bv.Unit {
			return eval.Length{V: av.V + (bv.V-av.V)*t, Unit: av.Unit}, true
		}
	case eval.Color:
		if bv, ok := b.(eval.Color); ok {
			return eval.Color{
				R: uint8(float64(av.R) + (float64(bv.R)-float64(av.R))*t),
				G: uint8(float64(av.G) + (float64(bv.G)-float64(av.G))*t),
				B: uint8(float64(av.B) + (float64(bv.B)-float64(av.B))*t),
				A: uint8(float64(av.A) + (float64(bv.A)-float64(av.A))*t),
			}, true
		}
	case eval.Str:
		if bv, ok := b.(eval.Str); ok {
			if t < 0.5 {
				return av, true
			}
			return bv, true
		}
	case eval.Bool:
		if bv, ok := b.(eval.Bool); ok {
			if t < 0.5 {
				return av, true
			}
			return bv, true
		}
	}
	return nil, false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/anim/ -run TestLerp -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/anim/tween.go internal/anim/tween_test.go
git commit -m "feat(anim): value interpolation for tweens"
```

---

### Task 3: Keyframe sampler

**Files:**
- Create: `internal/anim/keyframe.go`
- Test: `internal/anim/keyframe_test.go`

**Interfaces:**
- Consumes: `Easing`, `Lerp`, `ksl.Keyframes`/`ksl.Frame`, `ksl.AnimStep` (parser), `eval.Value`.
- Produces: `func Sample(kf *ksl.Keyframes, step ksl.AnimStep, at time.Duration) (map[string]eval.Value, error)` — the property set at a point in the animation, honouring duration, iteration (`infinite` or a count) and direction (`alternate`/`reverse`).

- [ ] **Step 1: Write the failing test**

```go
func TestSampleBlink(t *testing.T) {
	kf := mustKeyframes(t, `@keyframes blink { 0% { opacity: 1 } 50% { opacity: 0 } 100% { opacity: 1 } }`)
	step := ksl.AnimStep{Name: "blink", Dur: &ksl.Scalar{N: 800, Unit: "ms"}, Easing: keyword("linear"), Iteration: keyword("infinite")}
	// at t=0 opacity is 1, at 400ms opacity is 0, at 800ms it wraps to 1
	if got := sampleOpacity(t, kf, step, 0); got != 1 {
		t.Errorf("t=0 opacity = %v", got)
	}
	if got := sampleOpacity(t, kf, step, 400*time.Millisecond); got != 0 {
		t.Errorf("t=400ms opacity = %v", got)
	}
	if got := sampleOpacity(t, kf, step, 1200*time.Millisecond); got != 0 {
		t.Errorf("t=1200ms opacity = %v, want wrapped half cycle 0", got)
	}
}

func TestSampleDirection(t *testing.T) {
	kf := mustKeyframes(t, `@keyframes grow { 0% { width: 0 } 100% { width: 100 } }`)
	rev := ksl.AnimStep{Name: "grow", Dur: &ksl.Scalar{N: 1000, Unit: "ms"}, Direction: "reverse"}
	if got := sampleWidth(t, kf, rev, 0); got != 100 {
		t.Errorf("reverse t=0 width = %v, want 100", got)
	}
	alt := ksl.AnimStep{Name: "grow", Dur: &ksl.Scalar{N: 1000, Unit: "ms"}, Iteration: keyword("infinite"), Direction: "alternate"}
	if got := sampleWidth(t, kf, alt, 1500*time.Millisecond); got != 50 {
		t.Errorf("alternate t=1.5 cycles width = %v, want 50 (descending half)", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/anim/ -run TestSample -v`
Expected: FAIL — `Sample` undefined.

- [ ] **Step 3: Write the implementation**

The sampler resolves elapsed time into a frame position (0..1 of the keyframe track), then lerps the matching property values between the two surrounding frames:

```go
// Sample evaluates a keyframe animation at a point in time.
func Sample(kf *ksl.Keyframes, step ksl.AnimStep, at time.Duration) (map[string]eval.Value, error) {
	if len(kf.Frames) == 0 {
		return nil, fmt.Errorf("keyframes %s has no frames", kf.Name)
	}
	dur, err := durationOf(step)
	if err != nil {
		return nil, err
	}
	if dur <= 0 {
		return frameProps(kf.Frames[len(kf.Frames)-1]), nil
	}
	cycle := float64(at) / float64(dur)
	pos := math.Mod(cycle, 1)
	if step.Direction == "reverse" || (step.Direction == "alternate" && int(cycle)%2 == 1) {
		pos = 1 - pos
	}
	// find the two surrounding frames
	pct := pos * 100
	var lo, hi *ksl.Frame
	for _, f := range kf.Frames {
		if f.Percent <= pct {
			lo = f
		}
		if f.Percent >= pct && hi == nil {
			hi = f
		}
	}
	if lo == nil {
		lo = kf.Frames[0]
	}
	if hi == nil {
		hi = kf.Frames[len(kf.Frames)-1]
	}
	easing, err := ParseEasing(easingOf(step))
	if err != nil {
		return nil, err
	}
	t := 0.0
	if hi.Percent != lo.Percent {
		t = easing((pct - lo.Percent) / (hi.Percent - lo.Percent))
	}
	out := map[string]eval.Value{}
	for k, lv := range propsMap(lo.Props) {
		if hv, ok := propsMap(hi.Props)[k]; ok {
			if v, ok2 := Lerp(lv, hv, t); ok2 {
				out[k] = v
			} else {
				out[k] = lv
			}
		} else {
			out[k] = lv
		}
	}
	return out, nil
}
```

`propsMap` evaluates each frame's props once (they are literals in practice), `durationOf`/`easingOf` unwrap the step. Iteration `infinite` means `math.Mod` forever; a finite count clamps at the last cycle.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/anim/ -run TestSample -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/anim/keyframe.go internal/anim/keyframe_test.go
git commit -m "feat(anim): keyframe sampling with direction and iteration"
```

---

### Task 4: The frame clock and state edges

**Files:**
- Create: `internal/anim/clock.go`
- Modify: `internal/render/scene.go`
- Test: `internal/anim/clock_test.go`

**Interfaces:**
- Consumes: `Easing`, `Lerp`, `ksl.Transition`, `eval.State`.
- Produces: `type Clock struct` with `func (c *Clock) Tick(now time.Time)`, `func (c *Clock) Edge(state string) (start time.Time, ok bool)` and `func (c *Clock) Transition(tr *ksl.Transition, from, to map[string]eval.Value, now time.Time) map[string]eval.Value`.

- [ ] **Step 1: Write the failing test**

```go
func TestClockTracksEdges(t *testing.T) {
	c := NewClock(time.Unix(0, 0))
	c.Tick(time.Unix(0, 0), map[string]bool{"open": false})
	t1 := time.Unix(0, 100*int64(time.Millisecond))
	c.Tick(t1, map[string]bool{"open": true})
	start, ok := c.Edge("open")
	if !ok || !start.Equal(t1) {
		t.Errorf("edge = %v, %v", start, ok)
	}
	// no repeat edge on a steady state
	c.Tick(t1.Add(50*time.Millisecond), map[string]bool{"open": true})
	if _, ok := c.Edge("open"); !ok {
		t.Errorf("steady state must not re-arm the edge")
	}
}

func TestTransitionTweens(t *testing.T) {
	tr := mustTransition(t, `transition open { opacity: 200ms ease-out, rotate: 250ms ease-out }`)
	c := NewClock(time.Unix(0, 0))
	from := map[string]eval.Value{"opacity": eval.Num(0), "rotate": eval.Length{V: 2, Unit: "deg"}}
	to := map[string]eval.Value{"opacity": eval.Num(1), "rotate": eval.Length{V: 0, Unit: "deg"}}
	mid := c.Transition(tr, from, to, time.Unix(0, 100*int64(time.Millisecond)))
	op := mid["opacity"].(eval.Num)
	if op <= 0 || op >= 1 {
		t.Errorf("opacity at t=100ms/200ms ease-out = %v, want mid-flight", op)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/anim/ -run 'TestClock|TestTransition' -v`
Expected: FAIL — `Clock` undefined.

- [ ] **Step 3: Write the implementation**

`clock.go`:

```go
// Clock records when each built-in state last flipped, so transitions
// measure elapsed time from the edge.
type Clock struct {
	now    time.Time
	edges  map[string]time.Time
	active map[string]bool
}

func NewClock(now time.Time) *Clock {
	return &Clock{now: now, edges: map[string]time.Time{}, active: map[string]bool{}}
}

// Tick records the current frame's time and the truth of each state.
func (c *Clock) Tick(now time.Time, states map[string]bool) {
	c.now = now
	for name, on := range states {
		if on != c.active[name] {
			c.active[name] = on
			c.edges[name] = now
		}
	}
}

// Edge reports when a state last flipped.
func (c *Clock) Edge(state string) (time.Time, bool) {
	t, ok := c.edges[state]
	return t, ok
}

// Transition tweens a property map from one resolved frame to the next
// over the transition's per-property timings.
func (c *Clock) Transition(tr *ksl.Transition, from, to map[string]eval.Value, now time.Time) map[string]eval.Value {
	edge, ok := c.edges[tr.State]
	if !ok {
		return to
	}
	elapsed := now.Sub(edge)
	out := map[string]eval.Value{}
	for k, tv := range to {
		fv, ok := from[k]
		if !ok {
			out[k] = tv
			continue
		}
		tp := timingFor(tr, k)
		if tp == nil {
			out[k] = tv
			continue
		}
		easing, err := ParseEasing(tp.Easing)
		if err != nil {
			out[k] = tv
			continue
		}
		t := math.Min(1, float64(elapsed)/float64(durationOfScalar(tp.Dur)))
		if v, ok := Lerp(fv, tv, easing(t)); ok {
			out[k] = v
		} else {
			out[k] = tv
		}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/anim/ -run 'TestClock|TestTransition' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/anim/clock.go internal/anim/clock_test.go
git commit -m "feat(anim): frame clock and per-property transitions"
```

---

### Task 5: Async image loader and texture cache

**Files:**
- Create: `internal/render/images.go`
- Test: `internal/render/images_test.go`

**Interfaces:**
- Consumes: `image`, `image/png`, `image/jpeg`, `golang.org/x/image/webp` (new dep), `eval.Color` (placeholder).
- Produces: `type ImageCache struct` with `func (c *ImageCache) Get(path string, w, h float64) (image.Image, bool)` (decoded and sized, or not-ready), `func (c *ImageCache) Placeholder() image.Image` (a muted box), and `func (c *ImageCache) Preload(paths []string)`.

- [ ] **Step 1: Write the failing test**

```go
func TestImageCacheLoadsAndCaches(t *testing.T) {
	dir := t.TempDir()
	pngPath := writeTinyPNG(t, dir, "a.png") // 2x2 red PNG
	c := NewImageCache()
	if _, ok := c.Get(pngPath, 4, 4); ok {
		t.Fatal("first Get must report not-ready")
	}
	c.Pump() // test hook: drain the decode queue synchronously
	img, ok := c.Get(pngPath, 4, 4)
	if !ok || img == nil {
		t.Fatal("second Get must hit the cache")
	}
	// same path+size returns the same image
	img2, ok := c.Get(pngPath, 4, 4)
	if !ok || img2 != img {
		t.Errorf("cache returned a different image")
	}
}

func TestImageCachePlaceholder(t *testing.T) {
	c := NewImageCache()
	ph := c.Placeholder()
	if ph == nil || ph.Bounds().Dx() == 0 {
		t.Errorf("no placeholder")
	}
}

func TestImageCacheMissingFile(t *testing.T) {
	c := NewImageCache()
	if _, ok := c.Get("/no/such.png", 4, 4); ok {
		t.Errorf("missing file must not be ready")
	}
	c.Pump()
	if _, ok := c.Get("/no/such.png", 4, 4); ok {
		t.Errorf("missing file must stay not-ready")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run TestImage -v`
Expected: FAIL — `ImageCache` undefined.

- [ ] **Step 3: Write the implementation**

`images.go`: a queue of pending decodes; `Get` enqueues on miss and reports not-ready; a worker goroutine decodes (`image.Decode` after registering png/jpeg/webp), scales to the requested size, and stores; `Pump` drains synchronously for tests. Key = `path + "@" + wxh`. The placeholder is a 1x1 muted grey image, scaled by the painter.

Add the dep: `go get golang.org/x/image` and import `_ "golang.org/x/image/webp"` in images.go. This is the only new module this plan allows; everything else is stdlib.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/render/ -run TestImage -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render/images.go internal/render/images_test.go go.mod go.sum
git commit -m "feat(render): async image loading with a scene-wide cache"
```

---

### Task 6: Image painter and effects

**Files:**
- Create: `internal/render/effects.go`
- Modify: `internal/render/painters.go`
- Test: `internal/render/effects_test.go`

**Interfaces:**
- Consumes: `ImageCache`, `eval.FuncVal` (effects), the painter registry.
- Produces: the `Image` painter and the effect painters (`linear-gradient`, `radial-gradient`, `shadow`, `brightness`, `saturate`, `blur`→identity fallback).

- [ ] **Step 1: Write the failing test**

```go
func TestImagePainter(t *testing.T) {
	th, _ := theme.Load()
	img := image.NewRGBA(image.Rect(0, 0, 100, 60))
	dc := gg.NewContextForPixmap(gg.NewPixmapFromBuffer(img.Pix, 100, 60, img.Stride))
	cache := NewImageCache()
	n := &eval.ResolvedNode{Name: "Image", Rect: eval.RectF{X: 10, Y: 10, W: 40, H: 30},
		Props: map[string]eval.Value{"source": eval.Str("/no/such.png"), "fit": eval.Kw("cover"), "tint": eval.Color{0xFF, 0, 0, 0x80}}}
	painterFor("Image")(dc, th, n, &PainterDeps{Images: cache})
	if img.Pix[25*img.Stride+20*4+3] == 0 {
		t.Errorf("placeholder not painted for an unloaded image")
	}
}

func TestGradientFill(t *testing.T) {
	th, _ := theme.Load()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	dc := gg.NewContextForPixmap(gg.NewPixmapFromBuffer(img.Pix, 10, 10, img.Stride))
	n := &eval.ResolvedNode{Name: "Box", Rect: eval.RectF{X: 0, Y: 0, W: 10, H: 10},
		Props: map[string]eval.Value{"fill": eval.FuncVal{Name: "linear-gradient", Args: []eval.Value{
			eval.Color{0, 0, 0, 0xFF}, eval.Color{0xFF, 0xFF, 0xFF, 0xFF}}}}}
	painterFor("Box")(dc, th, n, &PainterDeps{})
	top := img.Pix[0*img.Stride+5*4]
	bot := img.Pix[9*img.Stride+5*4]
	if top == bot {
		t.Errorf("gradient painted flat: top=%v bot=%v", top, bot)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run 'TestImagePainter|TestGradientFill' -v`
Expected: FAIL — painters don't exist.

- [ ] **Step 3: Write the implementation**

`effects.go` and the new `Image` painter:

- `paintImage`: look up `source` in `PainterDeps.Images`; paint placeholder until ready; `fit` cover/contain/fill/tile chooses the draw rect; `tint` multiplies each pixel; `radius` clips via a path clip (cost noted in render.go:97-120, so clip only when radius is set).
- Effects on any node, applied in the painters' fill step: `linear-gradient(from, to)` and `radial-gradient(from, to)` paint a vertical/radial ramp instead of a flat fill; `shadow(dy, radius, color)` paints the silhouette offset by dy first; `brightness(f)`/`saturate(f)` transform the fill colour before painting; `blur(r)` is identity in v1 (spec §3 graceful fallback).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/render/ -run 'TestImagePainter|TestGradientFill' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render/effects.go internal/render/painters.go internal/render/effects_test.go
git commit -m "feat(render): image painter and the effect set"
```

---

### Task 7: Wire the clock into the frame

**Files:**
- Modify: `internal/render/scene.go`, `internal/render/painters.go`, `internal/render/render.go`
- Test: `internal/render/scene_test.go`

**Interfaces:**
- Consumes: `anim.Clock`, `anim.Sample`, `anim.Transition`, `SceneRenderer`.
- Produces: a per-frame animation pass: `SceneRenderer.DrawFrame` ticks the clock with the resolved state, painters read animated props from a clock-aware resolution step, and the cursor blink moves off `cursorAlpha` onto the `blink` keyframes in the default scene.

- [ ] **Step 1: Write the failing test**

```go
func TestClockDrivesBlink(t *testing.T) {
	// the blink keyframes in the default scene must replace cursorAlpha:
	// at t=0 opacity 1, at t=400ms opacity 0.
	sr := NewSceneRenderer(New(), config.Default())
	kf := sr.scene.Keyframes()[0]
	if kf.Name != "blink" {
		t.Fatalf("default scene keyframes = %+v", sr.scene.Keyframes())
	}
	at0 := sr.sampleAnim(kf, 0)
	at400 := sr.sampleAnim(kf, 400*time.Millisecond)
	if at0["opacity"] != eval.Num(1) || at400["opacity"] != eval.Num(0) {
		t.Errorf("blink = %v, %v", at0, at400)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run TestClockDrivesBlink -v`
Expected: FAIL — `sampleAnim` undefined.

- [ ] **Step 3: Write the implementation**

- `SceneRenderer` gains an `anim.Clock` (created on open, from `time.Now()`).
- `DrawFrame` calls `clock.Tick(now, map[string]bool{"open": st.Open, "selected": ...})` before resolving, and `eval.State.Now = now`.
- Painters look up a node's `animation` prop (an `*eval.AnimVal`), call `sr.sampleAnim` for each step, and overlay the sampled props on the resolved ones before painting. The `Cursor` painter drops `deps.CursorAlpha` in favour of its animated opacity.
- `cursorAlpha` in render.go becomes dead code; delete it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/render/ -run TestClockDrivesBlink -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render/scene.go internal/render/painters.go internal/render/render.go internal/render/scene_test.go
git commit -m "feat(render): drive animations from the frame clock"
```

---

### Task 8: Transitions on state edges

**Files:**
- Modify: `internal/render/scene.go`, `internal/render/painters.go`
- Test: `internal/render/scene_test.go`

**Interfaces:**
- Consumes: `anim.Clock.Transition`, the resolved Frame.
- Produces: per-node transitions on state edges (open/closed/selected/mode), so the reveal open transition and selection changes animate.

- [ ] **Step 1: Write the failing test**

```go
func TestSelectionTransition(t *testing.T) {
	// a selected-card transition tweens its fill over 180ms
	v := fakeView{open: true, results: twoItems(), selected: 0}
	r := New()
	sr := NewSceneRenderer(r, config.Default())
	// first frame: item 0 selected, transition armed
	f1 := sr.resolveFrame(t, v, at(0))
	sel1 := f1.Root.Children[0].Children[1].Children[0] // band/list/first item
	if sel1.Props["fill"] != eval.Color{0xFF, 0x2E, 0x63, 0xFF} {
		t.Fatalf("first selected fill = %v", sel1.Props["fill"])
	}
	// move selection, mid-transition the fill is between slab and accent
	v.selected = 1
	f2 := sr.resolveFrame(t, v, at(90*time.Millisecond))
	sel2 := f2.Root.Children[0].Children[1].Children[1]
	got := sel2.Props["fill"].(eval.Color)
	if got == (eval.Color{0x0B, 0x0B, 0x0F, 0xFF}) || got == (eval.Color{0xFF, 0x2E, 0x63, 0xFF}) {
		t.Errorf("mid-transition fill = %v, want between slab and accent", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run TestSelectionTransition -v`
Expected: FAIL — `resolveFrame` undefined.

- [ ] **Step 3: Write the implementation**

- `SceneRenderer` keeps the previous frame's resolved `Props` per node id.
- On a state edge (`open`, `selected`, `mode`), for every node whose props changed, run `clock.Transition` with the scene's matching `transition` block and merge the result into the node's final props before painting.
- Nodes without an id get transitions keyed by their tree path (name + index) so cards without ids still animate.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/render/ -run TestSelectionTransition -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render/scene.go internal/render/painters.go internal/render/scene_test.go
git commit -m "feat(render): transitions on state edges"
```

---

## Self-review (done during planning)

- **Spec coverage:** §3 effects → Task 6 (blur identity fallback documented); §6 animation → Tasks 1–4 and 7–8; §8 imagery → Task 5 (x/image is the single new dependency, PNG/JPEG/WebP static only, cache keyed by path+size, placeholder until ready). The cursor-blink parity test (Task 7) pins the one place the default scene replaces hard-coded timing.
- **Placeholder scan:** `Pump` (the test hook draining the decode queue) is defined; `durationOf`/`easingOf`/`timingFor`/`propsMap` are named with their single-sentence behaviour; no step defers code to a later task.
- **Type consistency:** `anim.Sample` returns `map[string]eval.Value`, which is exactly what the painters overlay onto resolved props; `Clock.Transition` takes the same map shape; `PainterDeps.Images` is added by Task 6 and consumed by the `Image` painter.

## Follow-ups (not in this scope)

- GPU shader path for blur/gradients when the gogpu renderer grows one.
- Animated image formats and remote URLs (spec §Out of scope).
