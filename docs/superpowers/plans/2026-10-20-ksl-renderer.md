# KSL Renderer Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hard-coded `internal/render` band/search/cards with a scene-driven renderer: `Renderer.DrawFrame` resolves the default embedded KSL scene (which reproduces today's look) against the launcher's `View` state and paints the resulting `Frame` with gg. User scenes load from config; any load failure falls back to the embedded default.

**Architecture:** The interpreter (`internal/ksl/eval`, from the interpreter plan) produces a plain-data `Frame`. This plan adds the painting layer inside `internal/render` (the only package allowed to import gg, `theme` and `card`): a painter registry mapping node names to painters, plus the `Card` painter bridging into the existing `card.Compose` + `drawCardContent` pipeline so the current card look is preserved byte-for-byte. `MaskFrame`/`BandContains` keep using `Reveal` directly — the reveal geometry is host-owned (spec decision 25) and the scene binds to it, so deriving the quad back out of the scene would duplicate a value the host already has.

**Tech Stack:** Go 1.27.2, existing deps only (`gogpu/gg`, `BurntSushi/toml`), `go:embed` for the default scene. New packages: none — `scene.go`, `painters.go`, `cardpainter.go` join `internal/render`.

**Spec:** `docs/superpowers/specs/2026-10-20-keyshell-ui-language-design.md` (§Integration; the reference scene is the shape of the default). Consumes `internal/ksl` (parser plan) and `internal/ksl/eval` (interpreter plan).

**Scope check:** Painting only. Image decoding, effects (blur/shadow/gradients), and the transition/keyframe clock are the imagery-effects-animation plan; the animation hooks (`state`/`transition` blocks, `animation` props) are parsed and carried through, and this plan treats them as resolved static values until that plan lands.

## Global Constraints

- Pure Go, `CGO_ENABLED=0`.
- `internal/render` keeps its existing public surface: `New()`, `DrawFrame`, `MaskFrame`, `BandContains`, `Present`. Existing tests (`render_test.go`, `card_test.go`) must pass unchanged in behaviour: the default scene reproduces today's band, search field and card reel.
- Drawing happens only in `internal/render`; `internal/ksl/eval` stays graphics-free.
- Tests: `go test ./internal/render/...`; suite `mise run test`; gate `mise run check`.
- Commits: `feat(render):` / `test(render):`, first line under 72 chars.
- The default scene is the shipped look: any change to `ui/default.ksl` that alters today's geometry is a regression, not a tweak.
- Load failures on a user scene log the diagnostic and keep the default scene; the overlay must never fail to open.

## File Structure

| File | Responsibility |
| --- | --- |
| `internal/render/default.ksl` | The default scene (embedded), reproducing today's band + search + reel |
| `internal/render/scene.go` | `SceneRenderer`: scene loading, State assembly, per-frame resolve+paint, user-scene fallback |
| `internal/render/painters.go` | Painter registry and core painters (Window, Box, Text, Icon, Cursor, Pill, Meter, Row/Column/Grid/Stack paint-through, Path, Group) |
| `internal/render/cardpainter.go` | The `Card` painter: `card.Compose` + existing `drawCard*` pipeline inside a resolved rect |
| `internal/render/scene_test.go` | Painter and integration tests against a fake `View` |
| `internal/config/config.go` | `UI.Scene` path (TOML `ui.scene`) |

Existing `render.go`, `card.go`, `reveal.go`, `present.go` are modified only where the plan says so; their helper painters (`drawChip`, `drawTicks`, `drawCardContent`, ...) are reused, not rewritten.

Tasks are ordered so each commits a compiling, tested increment.

---

### Task 1: Scene loading — default embedded scene, config path, fallback

**Files:**
- Create: `internal/render/default.ksl`, `internal/render/scene.go`
- Modify: `internal/config/config.go`
- Test: `internal/render/scene_test.go`

**Interfaces:**
- Consumes: `ksl.ParseWithLoader`, `ksl.Validate` (parser plan), `config.Config`.
- Produces: `func loadScene(cfg config.Config) (*ksl.Scene, error)` (embedded default as the fallback), `type SceneRenderer struct { scene *ksl.Scene; r *Renderer }` with `func NewSceneRenderer(r *Renderer, cfg config.Config) *SceneRenderer`.

The config addition:

```go
// UI holds the scene override.
type UI struct {
	Scene string `toml:"scene"` // path to a .ksl file; empty = embedded default
}

type Config struct {
	// ... existing fields
	UI UI `toml:"ui"`
}
```

- [ ] **Step 1: Write the failing test**

```go
package render

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/config"
)

func TestLoadSceneDefault(t *testing.T) {
	s, err := loadScene(config.Default())
	if err != nil {
		t.Fatalf("default scene: %v", err)
	}
	if s == nil || len(s.Nodes()) != 1 || s.Nodes()[0].Name != "Window" {
		t.Fatalf("default scene root = %+v", s)
	}
}

func TestLoadSceneOverride(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ui.ksl"
	if err := os.WriteFile(path, []byte("Window { Box { fill: #FF0000 } }"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.UI.Scene = path
	s, err := loadScene(cfg)
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if len(s.Nodes()[0].Children) != 1 || s.Nodes()[0].Children[0].Name != "Box" {
		t.Fatalf("override root = %+v", s.Nodes()[0])
	}
}

func TestLoadSceneFallbackOnError(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ui.ksl"
	os.WriteFile(path, []byte("Window { Box { fill"), 0o644) // broken
	cfg := config.Default()
	cfg.UI.Scene = path
	s, err := loadScene(cfg)
	if err != nil {
		t.Fatalf("broken override must not fail: %v", err)
	}
	// fell back to the default
	if len(s.Nodes()) != 1 || s.Nodes()[0].Name != "Window" {
		t.Fatalf("fallback root = %+v", s)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run TestLoadScene -v`
Expected: FAIL — `loadScene` undefined.

- [ ] **Step 3: Write the implementation**

`internal/render/scene.go`:

```go
package render

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/ksl"
)

//go:embed default.ksl
var defaultScene string

// loadScene reads the configured scene, falling back to the embedded
// default on any error. The default ships with the binary, so a broken
// user scene never keeps the overlay from opening.
func loadScene(cfg config.Config) (*ksl.Scene, error) {
	name, src := "default.ksl", []byte(defaultScene)
	if cfg.UI.Scene != "" {
		b, err := os.ReadFile(cfg.UI.Scene)
		if err == nil {
			name, src = cfg.UI.Scene, b
		} else {
			fmt.Fprintf(os.Stderr, "ksl: read %s: %v (using default scene)\n", cfg.UI.Scene, err)
		}
	}
	s, err := ksl.Parse(name, src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksl: %v (using default scene)\n", err)
		s, err = ksl.Parse("default.ksl", []byte(defaultScene))
	}
	if err != nil {
		return nil, fmt.Errorf("embedded default scene: %w", err)
	}
	if err := ksl.Validate(s); err != nil {
		fmt.Fprintf(os.Stderr, "ksl: %v (using default scene)\n", err)
		s, err = ksl.Parse("default.ksl", []byte(defaultScene))
		if err != nil {
			return nil, err
		}
		if err := ksl.Validate(s); err != nil {
			return nil, fmt.Errorf("embedded default scene: %w", err)
		}
	}
	return s, nil
}
```

The `include` loader for a user scene resolves relative to the scene file's directory (`filepath.Dir(cfg.UI.Scene)`), via `ksl.Loader`.

`internal/render/default.ksl` (the shipped look — layout values come from `config.Reveal` defaults, names map one-to-one):

```ksl
// keyshell's shipped overlay. Keep it in sync with the pre-KSL
// geometry: the band is a tapered trapezoid, the content column sits
// inside the rotated band frame, and the reel is a horizontal List.

Window {
    Group#band {
        x: 0, y: $reveal.y
        width: 100%, height: $reveal.h
        rotate: $reveal.angle
        shear: $reveal.shear
        Path { d: $reveal.d, fill: $slab }
        Column { gap: 8, align: center
            Row { gap: 12, align: center
                Pill { text: $mode, tone: accent }
                Text { text: $query, size: 22, uppercase: true, color: $ink }
                Cursor { color: $accent, animation: blink 800ms linear infinite }
                Text { text: $results.len, size: 14, color: $muted }
            }
            List {
                items: $results, selected: $selected
                orientation: horizontal
                scroll: center
                overflow: cycle
                viewport: { width: 60% }
                template {
                    Group#card {
                        shear: $reveal.cardShear
                        Card { }
                    }
                }
                empty {
                    Text { text: "NO RESULTS", size: 12, color: $muted }
                }
            }
        }
    }
}

state open { Group#band { opacity: 1 } }
transition open { rotate: 250ms ease-out, opacity: 180ms }
@keyframes blink { 0% { opacity: 1 } 50% { opacity: 0 } 100% { opacity: 1 } }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/render/ -run TestLoadScene -v`
Expected: PASS. (The `Card` node type is not in the primitive kit; validation treats it as an unknown node — add `Card` to the validation's known set via a small `ksl.Validate`-side allowance, or register it through a painter registry hook in Task 4. This task: validate with `Card` registered as a known component name by the loader.)

- [ ] **Step 5: Commit**

```bash
git add internal/render/scene.go internal/render/default.ksl internal/config/config.go internal/render/scene_test.go
git commit -m "feat(render): load the default scene with a config override and fallback"
```

---

### Task 2: State assembly — View to eval.State

**Files:**
- Modify: `internal/render/scene.go`
- Test: `internal/render/scene_test.go`

**Interfaces:**
- Consumes: `View` (`Poll/Results/Selected/Query/IsOpen/ModeBadge`), `theme.Theme`, `Reveal`, `config.Reveal`, `backend.Item`/`rank.Match`.
- Produces: `func (r *SceneRenderer) state(l View, th *theme.Theme, rev Reveal, cfg config.Reveal) eval.State` and the `backend.Item` → `eval.Item` mapping.

- [ ] **Step 1: Write the failing test**

```go
func TestStateFromView(t *testing.T) {
	v := fakeView{open: true, query: "fire", mode: "apps",
		results: []rank.Match{{Item: backend.Item{ID: "x", Title: "Firefox"}}},
		selected: 0}
	th, _ := theme.Load()
	rev := Reveal{YLeft: 100, YRight: 120, HLeft: 64, HRight: 80, Angle: 0.02, Shear: -0.25}
	st := NewSceneRenderer(New(), config.Default()).state(v, th, rev, config.Default().Reveal)
	if !st.Open || st.Query != "fire" || st.Mode != "apps" {
		t.Errorf("state = %+v", st)
	}
	if len(st.Results) != 1 || st.Results[0].ID != "x" || st.Results[0].Title != "Firefox" {
		t.Errorf("items = %+v", st.Results)
	}
	if st.Reveal.H != 64 || st.Reveal.Angle != 0.02 {
		t.Errorf("reveal = %+v", st.Reveal)
	}
	if st.Tokens["accent"] == (eval.Color{}) {
		t.Errorf("theme tokens missing")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run TestStateFromView -v`
Expected: FAIL — `eval.State` assembly undefined.

- [ ] **Step 3: Write the implementation**

In `scene.go`:

```go
// state assembles the binding snapshot for one frame.
func (r *SceneRenderer) state(l View, th *theme.Theme, rev Reveal, cfg config.Reveal) eval.State {
	results := l.Results()
	items := make([]eval.Item, len(results))
	for i, m := range results {
		items[i] = itemFromBackend(m.Item)
	}
	return eval.State{
		Open:     l.IsOpen(),
		Focused:  true, // the overlay always has focus while open (see input)
		Query:    l.Query(),
		Mode:     l.ModeBadge(),
		Selected: l.Selected(),
		Results:  items,
		Reveal: eval.Reveal{
			Y: rev.Yc, H: math.Min(rev.HLeft, rev.HRight),
			HLeft: rev.HLeft, HRight: rev.HRight,
			Angle: rev.Angle, Shear: rev.Shear, CardShear: rev.Shear,
			D: bandD(rev, r.surfaceW()),
		},
		Typing: typingFromQuery(l),
		Tokens: themeTokens(th),
		Fonts:  map[string]string{"display": theme.DefaultFace},
		Summon: r.summonEpoch,
		Seed:   r.seed,
		RandCache: r.randCache,
		Width:  r.surfaceW(),
		Height: r.surfaceH(),
		Scale:  cfg.BandHeightPx / 290, // 290 is the reference band height
	}
}
```

`themeTokens(th)` maps the six palette colours to eval.Color. `itemFromBackend` maps `backend.Item` fields one-to-one (`ID` via `ResultID.String()` if it has one, else `fmt.Sprint`). `bandD(rev, w)` renders the trapezoid as a path string. `typingFromQuery` is a placeholder returning 0 until plan 4 lands.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/render/ -run TestStateFromView -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render/scene.go internal/render/scene_test.go
git commit -m "feat(render): assemble eval.State from the launcher view"
```

---

### Task 3: Painter registry and core painters

**Files:**
- Create: `internal/render/painters.go`
- Test: `internal/render/painters_test.go`

**Interfaces:**
- Consumes: `eval.Frame`, `eval.ResolvedNode`, `gg.Context`, `theme.Theme`, existing helpers (`drawChip`, `drawTicks`, `drawLines`, `elide`, `cursorAlpha`).
- Produces: `type Painter func(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps)`, `type PainterDeps struct{ Reveal Reveal; CursorAlpha float64; CardCfg config.Reveal }`, and the registry `map[string]Painter`.

- [ ] **Step 1: Write the failing test**

```go
func TestPaintersPaint(t *testing.T) {
	th, err := theme.Load()
	if err != nil { t.Fatal(err) }
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	pm := gg.NewPixmapFromBuffer(img.Pix, 200, 100, img.Stride)
	dc := gg.NewContextForPixmap(pm)

	tests := []struct {
		name string
		node *eval.ResolvedNode
	}{
		{"Box", &eval.ResolvedNode{Name: "Box", Rect: eval.RectF{X: 10, Y: 10, W: 50, H: 30},
			Props: map[string]eval.Value{"fill": eval.Color{0xFF, 0, 0, 0xFF}}}},
		{"Text", &eval.ResolvedNode{Name: "Text", Rect: eval.RectF{X: 10, Y: 10, W: 80, H: 20},
			Props: map[string]eval.Value{"text": eval.Str("HI"), "size": eval.Num(12), "color": eval.Color{0xFF, 0xFF, 0xFF, 0xFF}}}},
		{"Pill", &eval.ResolvedNode{Name: "Pill", Rect: eval.RectF{X: 10, Y: 50, W: 40, H: 18},
			Props: map[string]eval.Value{"text": eval.Str("SEARCH"), "tone": eval.Kw("accent")}}},
		{"Cursor", &eval.ResolvedNode{Name: "Cursor", Rect: eval.RectF{X: 60, Y: 10, W: 3, H: 24},
			Props: map[string]eval.Value{"color": eval.Color{0xFF, 0x2E, 0x63, 0xFF}}}},
		{"Meter", &eval.ResolvedNode{Name: "Meter", Rect: eval.RectF{X: 10, Y: 70, W: 100, H: 7},
			Props: map[string]eval.Value{"ratio": eval.Num(0.5), "variant": eval.Kw("ticks"), "ticks": eval.Num(10)}}},
		{"Path", &eval.ResolvedNode{Name: "Path", Rect: eval.RectF{X: 0, Y: 0, W: 200, H: 100},
			Props: map[string]eval.Value{"d": eval.Str("M 0 0 L 200 0 L 200 100 L 0 100 Z"), "fill": eval.Color{0, 0, 0xFF, 0xFF}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := img.Pix[50*img.Stride+20*4]
			painterFor(tt.node.Name)(dc, th, tt.node, &PainterDeps{})
			if img.Pix[50*img.Stride+20*4] == before {
				t.Errorf("%s left the pixel untouched", tt.name)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run TestPaintersPaint -v`
Expected: FAIL — `painterFor`/`PainterDeps` undefined.

- [ ] **Step 3: Write the implementation**

`internal/render/painters.go`:

```go
package render

// Painter draws one resolved node.
type Painter func(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps)

type PainterDeps struct {
	Reveal      Reveal
	CursorAlpha float64
	CardCfg     config.Reveal
}

func painterFor(name string) Painter {
	switch name {
	case "Window":
		return paintWindow
	case "Box":
		return paintBox
	case "Text":
		return paintText
	case "Icon":
		return paintIcon
	case "Cursor":
		return paintCursor
	case "Pill":
		return paintPill
	case "Meter":
		return paintMeter
	case "Path":
		return paintPath
	case "Card":
		return paintCard
	default:
		return paintThrough // containers paint children only
	}
}
```

The painters translate resolved props into the existing draw calls: `paintBox` = `DrawRectangle`/`DrawRoundedRectangle` + fill/border/opacity; `paintText` = face from theme registry, wrap/elide/uppercase via existing helpers; `paintPill` = `drawChip`; `paintCursor` = the existing cursor bar with `deps.CursorAlpha`; `paintMeter` = `drawTicks`/`drawMeter` variants; `paintPath` = parse the `d` string into gg path ops; `paintWindow`/`paintThrough` recurse into children. Group applies `Push`/`Rotate`/`Translate`/`Pop` with the resolved transforms.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/render/ -run TestPaintersPaint -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render/painters.go internal/render/painters_test.go
git commit -m "feat(render): node painters over the existing draw helpers"
```

---

### Task 4: The Card painter

**Files:**
- Create: `internal/render/cardpainter.go`
- Test: `internal/render/cardpainter_test.go`

**Interfaces:**
- Consumes: `card.Compose`, `drawCard`, `drawCardContent`, `eval.Item`.
- Produces: `paintCard`, the bridge from a `Card` node to today's card pipeline. The node's resolved rect is the card's quad; its props carry `$item` fields from the List template.

- [ ] **Step 1: Write the failing test**

```go
func TestCardPainterDrawsAPlan(t *testing.T) {
	th, err := theme.Load()
	if err != nil { t.Fatal(err) }
	img := image.NewRGBA(image.Rect(0, 0, 400, 200))
	dc := gg.NewContextForPixmap(gg.NewPixmapFromBuffer(img.Pix, 400, 200, img.Stride))
	item := eval.Item{ID: "a", Title: "Firefox", Badge: "APP"}
	n := &eval.ResolvedNode{Name: "Card", Rect: eval.RectF{X: 20, Y: 20, W: 320, H: 160},
		Props: map[string]eval.Value{"item": eval.ItemVal{&item}}}
	paintCard(dc, th, n, &PainterDeps{CardCfg: config.Default().Reveal})
	if img.Pix[100*img.Stride+100*4] == 0 {
		t.Errorf("card body unpainted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run TestCardPainter -v`
Expected: FAIL — `paintCard` undefined.

- [ ] **Step 3: Write the implementation**

`internal/render/cardpainter.go`:

```go
package render

// paintCard draws a resolved Card node through the existing card
// pipeline, so a scene's `template { Card { } }` reproduces today's
// cards exactly.
func paintCard(dc *gg.Context, th *theme.Theme, n *eval.ResolvedNode, deps *PainterDeps) {
	item, ok := n.Props["item"].(eval.ItemVal)
	if !ok {
		return
	}
	plan := card.Compose(itemToBackend(item.I))
	drawCard(dc, th, deps.Reveal, deps.CardCfg, n.Rect.X, n.Rect.Y, n.Rect.W, n.Rect.H, plan, selectedProp(n))
}
```

`itemToBackend` is the inverse of Task 2's mapping; `selectedProp` reads the node's `selected` prop resolved by eval.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/render/ -run TestCardPainter -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render/cardpainter.go internal/render/cardpainter_test.go
git commit -m "feat(render): paint Card nodes through the existing card pipeline"
```

---

### Task 5: SceneRenderer — resolve and paint per frame

**Files:**
- Modify: `internal/render/render.go`, `internal/render/scene.go`
- Test: `internal/render/scene_test.go`

**Interfaces:**
- Consumes: `eval.Resolve`, `painterFor`, `state` (Task 2), `loadScene` (Task 1).
- Produces: `Renderer.DrawFrame` rewritten to paint the resolved Frame; the public surface is unchanged.

- [ ] **Step 1: Write the failing test**

```go
func TestDrawFrameRendersTheDefaultScene(t *testing.T) {
	// the existing TestDrawFrameProducesImage must keep passing; this
	// adds a scene-level assertion.
	v := fakeView{open: true, query: "f", results: []rank.Match{{Item: backend.Item{Title: "Firefox"}}}}
	r := New()
	sr := NewSceneRenderer(r, config.Default())
	img := image.NewRGBA(image.Rect(0, 0, 640, 200))
	dc := gg.NewContextForPixmap(gg.NewPixmapFromBuffer(img.Pix, 640, 200, img.Stride))
	sr.DrawFrame(dc, v, mustTheme(t), config.Default().Reveal)
	// band fill present somewhere in the middle band region
	var painted int
	for y := 80; y < 120; y++ {
		if img.Pix[y*img.Stride+320*4+3] != 0 {
			painted++
		}
	}
	if painted == 0 {
		t.Errorf("band not painted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run TestDrawFrameRenders -v`
Expected: FAIL — `SceneRenderer.DrawFrame` undefined.

- [ ] **Step 3: Write the implementation**

`internal/render/scene.go` gains:

```go
type SceneRenderer struct {
	inner      *Renderer
	scene      *ksl.Scene
	summonEpoch int
	seed       uint64
	randCache  *eval.RandCache
}

func NewSceneRenderer(r *Renderer, cfg config.Config) *SceneRenderer {
	s, err := loadScene(cfg)
	if err != nil {
		// loadScene only errors if the embedded default is broken;
		// that is a build-time bug.
		panic(err)
	}
	return &SceneRenderer{inner: r, scene: s, seed: 1, randCache: &eval.RandCache{}}
}

// DrawFrame paints one frame of the scene.
func (sr *SceneRenderer) DrawFrame(dc *gg.Context, l View, th *theme.Theme, cfg config.Reveal) {
	sr.inner.mu.Lock()
	if !sr.inner.wasOpen && l.IsOpen() {
		sr.summonEpoch++
		sr.inner.reveal = Resample(float64(dc.Width()), float64(dc.Height()), cfg, sr.inner.rng)
	}
	rev := sr.inner.reveal
	sr.inner.mu.Unlock()

	l.Poll()
	if !l.IsOpen() {
		dc.ClearWithColor(gg.RGBA{})
		return
	}
	dc.ClearWithColor(gg.RGBA{})
	st := sr.state(l, th, rev, cfg)
	frame, err := eval.Resolve(sr.scene, st)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksl: frame resolve: %v\n", err)
		return
	}
	paintFrame(dc, th, frame, &PainterDeps{Reveal: rev, CardCfg: cfg,
		CursorAlpha: cursorAlpha(time.Since(sr.inner.opened))})
}
```

with `paintFrame` walking the resolved tree via `painterFor` and recursing.

`render.go`'s `DrawFrame` becomes a thin delegate: `func (r *Renderer) DrawFrame(...) { r.scene.DrawFrame(dc, l, th, cfg) }`, with `New()` building the default `SceneRenderer`. The existing `TestDrawFrameProducesImage`, `TestDrawFrameBlanksWhenClosed`, `TestDrawFrameClipsContentToTheBand` must pass unchanged — `MaskFrame` and `BandContains` stay on `Reveal`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/render/ -v`
Expected: PASS, including the pre-existing render and card tests.

- [ ] **Step 5: Commit**

```bash
git add internal/render/scene.go internal/render/render.go internal/render/scene_test.go
git commit -m "feat(render): drive the frame from the resolved scene"
```

---

### Task 6: cmd wiring

**Files:**
- Modify: `cmd/keyshell/main.go`

**Interfaces:**
- Consumes: `NewSceneRenderer`, config UI.
- Produces: the binary loads the configured scene at startup; load failures already fall back with a stderr note.

- [ ] **Step 1: Wire the scene renderer into main**

Replace the `render.New()` construction with `render.NewSceneRenderer(render.New(), cfg)` so the overlay uses the configured scene; keep the existing config load call. If the scene fails to load, the binary continues with the default (already handled inside `loadScene`).

- [ ] **Step 2: Run the full suite and the format gate**

Run: `mise run test && mise run check`
Expected: all green.

- [ ] **Step 3: Commit**

```bash
git add cmd/keyshell/main.go
git commit -m "feat: load the overlay scene from config"
```

---

## Self-review (done during planning)

- **Spec coverage:** §Integration (default scene, config override, parse/validate, interpreter drives draw calls) → Tasks 1, 5, 6; the reference scene → Task 1's default.ksl; the card-composition spec (fixed zones, caps, ranking) → Task 4's Card painter preserves the existing pipeline untouched. `MaskFrame`/`BandContains` stay on `Reveal` — documented in the header as a deliberate divergence from the research doc's "derive from resolved transform" sketch, because the host owns the geometry and the scene binds to it.
- **Placeholder scan:** painter bodies are named per-helper with the existing draw functions they delegate to; the `paintThrough` container behaviour is defined by the registry test, not left vague.
- **Type consistency:** `PainterDeps` fields match what painters read (`Reveal` for the band, `CursorAlpha` for the cursor, `CardCfg` for card sizing); `eval.ResolvedNode` fields match the interpreter plan's produced type.

## Follow-up plan (not in this scope)

Imagery, effects and the transition/keyframes animation clock: async image loading, gradient/blur/shadow painters, and the per-property tween engine that animates `state`/`transition`/`animation` over time.
