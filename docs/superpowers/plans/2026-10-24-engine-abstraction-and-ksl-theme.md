# Engine Abstraction & KSL Theme Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Decouple Keyshell's engine by unifying all backends under a common registry without hardcoded branches, moving band geometry into a KSL helper with scene-driven hit-testing, and supporting 3-tier overridable KSL theme variables.

**Architecture:** Extend KSL AST and evaluator with typed theme variables and a `band(...)` geometry helper; implement `Frame.Contains(x, y)` to replace hardcoded hit-testing; replace Go `config.Reveal` with generic `config.Theme`; unify all backends (in-process and subprocess) behind `backend.ProviderRegistry` and `sdk.Plugin`.

**Tech Stack:** Go standard library, `github.com/darkliquid/keyshell/internal/ksl`, `github.com/darkliquid/keyshell/internal/ksl/eval`, `github.com/darkliquid/keyshell/internal/plugin/sdk`.

---

### Task 1: KSL Theme Variables & General Value Declarations

**Files:**
- Modify: `internal/ksl/ast.go:68-80`
- Modify: `internal/ksl/decl.go:10-48`
- Modify: `internal/ksl/eval/resolve.go:60-70, 178-196`
- Modify: `internal/ksl/eval/eval.go:20-45`
- Test: `internal/ksl/decl_test.go`
- Test: `internal/ksl/eval/resolve_test.go`

- [x] **Step 1: Write failing test for theme variables of various types**

In `internal/ksl/eval/resolve_test.go`:
```go
func TestThemeVariablesResolve(t *testing.T) {
	src := `
theme {
    accent: #00FFC2
    band_height: 320
    min_tilt: 6.5
    title: "Keyshell"
    enabled: true
}
Window {
    Text { text: $title, size: $min_tilt, color: $accent }
}
`
	sc, err := ksl.Parse("test.ksl", []byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	st := State{}
	frame, err := Resolve(sc, st)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(frame.Nodes) == 0 {
		t.Fatal("expected resolved nodes in frame")
	}
	textNode := frame.Nodes[0].Children[0]
	if textNode.Text != "Keyshell" {
		t.Errorf("text = %q, want Keyshell", textNode.Text)
	}
	if textNode.Size != 6.5 {
		t.Errorf("size = %v, want 6.5", textNode.Size)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/eval -run TestThemeVariablesResolve -v`
Expected: FAIL (theme block parser expects only color hex or variables are not exposed in environment)

- [x] **Step 3: Update KSL theme parser and evaluator**

In `internal/ksl/ast.go`:
Update `ThemeBlock`:
```go
// ThemeBlock carries theme tokens, dimensions, numbers, and settings.
type ThemeBlock struct {
	Tokens []*Prop
}
```

In `internal/ksl/decl.go`:
Update theme parsing to accept any valid property value (not just color):
```go
func (p *Parser) parseTheme() *ThemeBlock {
	p.next() // theme
	t := p.current()
	if t.kind != tokenLBrace {
		p.errf(t.pos, "expected '{' after theme, got %s", p.describe(t))
		return nil
	}
	p.next()

	var tokens []*Prop
	for {
		t = p.current()
		if t.kind == tokenRBrace {
			p.next()
			break
		}
		if t.kind == tokenEOF {
			p.errf(t.pos, "unterminated theme block")
			return nil
		}
		if t.kind == tokenIdent {
			pr := p.parseProp()
			if pr != nil {
				tokens = append(tokens, pr)
			}
			continue
		}
		p.errf(t.pos, "unexpected %s in theme block", p.describe(t))
		p.next()
	}
	return &ThemeBlock{Tokens: tokens}
}
```

In `internal/ksl/eval/eval.go`:
Add `Vars map[string]any` to `State`:
```go
type State struct {
	Name      string
	Open      bool
	Focused   bool
	Query     string
	Mode      string
	Selected  int
	Results   []Item
	Searching bool
	Panel     Panel
	Reveal    Reveal
	Tokens    map[string]Color
	Vars      map[string]any
	Fonts     map[string]string
	Summon    uint64
	Seed      int64
	RandCache *RandCache
	Width     float64
	Height    float64
	Scale     float64
}
```

In `internal/ksl/eval/resolve.go`:
In `sceneTokens`, extract both colors and general variables:
```go
func sceneVars(scene *ksl.Scene) map[string]any {
	out := map[string]any{}
	for _, th := range scene.ThemeBlocks() {
		for _, pr := range th.Tokens {
			if v := evalStaticVal(pr.Val); v != nil {
				out[pr.Key] = v
			}
		}
	}
	return out
}

func evalStaticVal(v ksl.Val) any {
	switch val := v.(type) {
	case *ksl.ColorVal:
		if c, err := hexColor(val.Hex); err == nil {
			return c
		}
	case *ksl.NumVal:
		return val.N
	case *ksl.DimVal:
		return val.N
	case *ksl.StrVal:
		return val.S
	case *ksl.BoolVal:
		return val.B
	}
	return nil
}
```
And inside `Resolve(scene, st)`:
Populate `r.env` with `st.Vars` and `sceneVars(scene)`:
```go
	vars := make(map[string]any)
	for k, v := range sceneVars(scene) {
		vars[k] = v
		if c, ok := v.(Color); ok {
			tokens[k] = c
		}
	}
	for k, v := range st.Vars {
		vars[k] = v
		if c, ok := v.(Color); ok {
			tokens[k] = c
		}
	}
	for k, v := range vars {
		r.env.Set(k, v)
	}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/eval -run TestThemeVariablesResolve -v`
Expected: PASS

- [x] **Step 5: Commit changes**

```bash
git add internal/ksl/ast.go internal/ksl/decl.go internal/ksl/eval/
git commit -m "feat(ksl): support arbitrary typed theme variables in scene theme blocks"
```

---

### Task 2: Config Cascading for Theme Variables

**Files:**
- Modify: `internal/config/config.go:13-31, 89-120`
- Modify: `internal/config/config_test.go`
- Modify: `internal/render/scene.go:212-240`
- Modify: `internal/render/load.go`
- Test: `internal/render/scene_test.go`

- [x] **Step 1: Write failing test for config theme cascading**

In `internal/config/config_test.go`:
```go
func TestConfigThemeVariables(t *testing.T) {
	raw := `
[theme]
accent = "#08F7FE"
band_height = 320
card_width = 380
min_tilt = 7.5
`
	var cfg Config
	if err := toml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if cfg.Theme["accent"] != "#08F7FE" {
		t.Errorf("accent = %v, want #08F7FE", cfg.Theme["accent"])
	}
	if h, ok := cfg.Theme["band_height"].(int64); !ok || h != 320 {
		t.Errorf("band_height = %v, want 320", cfg.Theme["band_height"])
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestConfigThemeVariables -v`
Expected: FAIL (field `Theme` not defined on `Config`)

- [x] **Step 3: Replace `config.Reveal` with generic `config.Theme`**

In `internal/config/config.go`:
Remove `type Reveal struct`.
Add `Theme map[string]any` to `Config`:
```go
type Config struct {
	Theme     map[string]any `toml:"theme"`
	Anim      Anim
	Overlay   Overlay
	Commands  Commands `toml:"commands"`
	Frecency  Frecency
	Instances []Instance `toml:"instance"`
	UI        UI         `toml:"ui"`
}

func Default() Config {
	return Config{
		Theme: map[string]any{
			"slab":        "#0B0B0F",
			"surface_alt": "#121218",
			"ink":         "#F2F2F7",
			"accent":      "#FF2E63",
			"accent_alt":  "#08F7FE",
			"muted":       "#5A5A66",
		},
		Anim: Anim{
			ExpandMs: 120,
			ScrollMs: 90,
		},
		Overlay: Overlay{
			MaxResults: 10,
			SettleMs:   80,
			IdleMs:     30000,
		},
	}
}
```

In `internal/render/scene.go`:
Pass `cfg.Theme` into `eval.State.Vars`:
```go
	vars := make(map[string]any, len(cfg.Theme)+8)
	for k, v := range cfg.Theme {
		vars[k] = coerceThemeVal(v)
	}
```
Implement `coerceThemeVal(v any) any` to convert hex strings starting with `#` into `eval.Color`, numbers into `float64`, etc.

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestConfigThemeVariables -v`
Expected: PASS

- [x] **Step 5: Run render tests and ensure clean compilation**

Run: `go test ./internal/config ./internal/render -v`
Expected: PASS

- [x] **Step 6: Commit changes**

```bash
git add internal/config/ internal/render/
git commit -m "feat(config): replace hardcoded Reveal with generic overridable Theme map"
```

---

### Task 3: KSL `band(...)` Geometry Helper Function

**Files:**
- Create: `internal/ksl/eval/band.go`
- Create: `internal/ksl/eval/band_test.go`
- Modify: `internal/render/default.ksl`
- Delete/Refactor: `internal/render/reveal.go`
- Modify: `internal/render/scene.go`

- [x] **Step 1: Write failing test for `band(...)` helper in KSL**

Create `internal/ksl/eval/band_test.go`:
```go
package eval

import (
	"testing"
)

func TestBandHelperGeneratesGeometry(t *testing.T) {
	env := NewEnv(&State{Width: 1920, Height: 1080}, nil)
	val, err := callBandHelper(env, []any{
		290.0, // height
		5.0,   // min_tilt
		0.10,  // spread
		0.15,  // taper
		6.0,   // shear_min
		14.0,  // shear_max
	})
	if err != nil {
		t.Fatalf("band() failed: %v", err)
	}
	m, ok := val.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", val)
	}
	for _, key := range []string{"d", "angle", "shear", "cw", "ch", "frameX", "frameY"} {
		if _, exists := m[key]; !exists {
			t.Errorf("missing key %q in band result", key)
		}
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/eval -run TestBandHelperGeneratesGeometry -v`
Expected: FAIL with `undefined: callBandHelper`

- [x] **Step 3: Implement `band(...)` helper**

Create `internal/ksl/eval/band.go`:
```go
package eval

import (
	"fmt"
	"math"
	"math/rand"
)

func callBandHelper(env *Env, args []any) (any, error) {
	st := env.State()
	w, h := st.Width, st.Height
	if w <= 0 {
		w = 1920
	}
	if h <= 0 {
		h = 1080
	}

	height := getFloat(args, 0, 290.0)
	minTiltDeg := getFloat(args, 1, 5.0)
	centerSpread := getFloat(args, 2, 0.10)
	taperMax := getFloat(args, 3, 0.15)
	shearMinDeg := getFloat(args, 4, 6.0)
	shearMaxDeg := getFloat(args, 5, 14.0)

	rng := rand.New(rand.NewSource(st.Seed ^ int64(st.Summon)))

	const degToRad = math.Pi / 180
	yMid := h * 0.5
	spread := h * centerSpread
	minDelta := w * math.Tan(minTiltDeg*degToRad)

	yL := yMid + (rng.Float64()*2-1)*spread
	yR := yMid + (rng.Float64()*2-1)*spread
	if math.Abs(yR-yL) < minDelta {
		mid := (yL + yR) / 2
		dir := 1.0
		if yR < yL {
			dir = -1
		}
		yL = mid - dir*minDelta/2
		yR = mid + dir*minDelta/2
	}

	angle := math.Atan2(yR-yL, w)
	center := (yL + yR) / 2

	taperL := 1 + (rng.Float64()*2-1)*taperMax
	taperR := 1 + (rng.Float64()*2-1)*taperMax
	hLeft := math.Max(200.0, height*taperL)
	hRight := math.Max(200.0, height*taperR)

	jitter := (rng.Float64()*2 - 1) * 2.0
	raw := 1.0*math.Abs(angle/degToRad) + jitter
	shearDeg := math.Max(shearMinDeg, math.Min(shearMaxDeg, raw))
	sign := 1.0
	if angle < 0 {
		sign = -1
	}
	shear := math.Tan(shearDeg*degToRad) * sign

	halfL := hLeft / 2
	halfR := hRight / 2
	d := fmt.Sprintf("M 0 %g L %g %g L %g %g L 0 %g Z",
		yL-halfL, w, yR-halfR, w, yR+halfR, yL+halfL)

	cosA := math.Cos(math.Abs(angle))
	sinA := math.Sin(math.Abs(angle))
	cw := w*cosA - (hLeft+hRight)/2*sinA
	ch := math.Min(hLeft, hRight)

	return map[string]any{
		"d":      d,
		"angle":  angle * 180 / math.Pi,
		"shear":  shear,
		"cw":     cw,
		"ch":     ch,
		"frameX": (w - cw) / 2,
		"frameY": center - ch/2,
		"y":      center,
		"hLeft":  hLeft,
		"hRight": hRight,
	}, nil
}

func getFloat(args []any, idx int, def float64) float64 {
	if idx < len(args) {
		switch v := args[idx].(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return def
}
```

Register `band` in `internal/ksl/eval/resolve.go` built-in function calls.
Update `internal/render/default.ksl` to call `let reveal = band($band_height, $min_tilt, $center_spread, $taper_max, $shear_min, $shear_max)`.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ksl/eval -run TestBandHelperGeneratesGeometry -v`
Expected: PASS

- [x] **Step 5: Commit changes**

```bash
git add internal/ksl/eval/ internal/render/
git commit -m "feat(ksl): implement band geometry helper function and move reveal logic out of Go engine"
```

---

### Task 4: Scene-Driven Frame Hit-Testing

**Files:**
- Modify: `internal/ksl/eval/eval.go`
- Create: `internal/ksl/eval/hittest.go`
- Create: `internal/ksl/eval/hittest_test.go`
- Modify: `internal/render/render.go:215-245`
- Modify: `internal/cli/run.go:133-145`
- Test: `internal/render/render_test.go`

- [x] **Step 1: Write failing test for `Frame.Contains(x, y)`**

Create `internal/ksl/eval/hittest_test.go`:
```go
package eval

import "testing"

func TestFrameContainsPointsInsideAndOutside(t *testing.T) {
	frame := &Frame{
		Width:  1920,
		Height: 1080,
		Nodes: []*FrameNode{
			{
				Kind: "Box",
				X:    640,
				Y:    120,
				W:    640,
				H:    460,
			},
		},
	}

	if !frame.Contains(650, 150) {
		t.Error("expected inside point to return true")
	}
	if frame.Contains(100, 100) {
		t.Error("expected outside point to return false")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ksl/eval -run TestFrameContainsPointsInsideAndOutside -v`
Expected: FAIL with `frame.Contains undefined`

- [x] **Step 3: Implement `Frame.Contains(x, y)`**

Create `internal/ksl/eval/hittest.go`:
```go
package eval

// Contains reports whether (x, y) falls inside any rendered top-level window,
// box, panel, or polygon in the frame.
func (f *Frame) Contains(x, y float64) bool {
	if f == nil || len(f.Nodes) == 0 {
		return false
	}
	for _, n := range f.Nodes {
		if nodeContains(n, x, y) {
			return true
		}
	}
	return false
}

func nodeContains(n *FrameNode, x, y float64) bool {
	if n.Kind == "Window" || n.Kind == "Box" || n.Kind == "Group" {
		if x >= n.X && x <= n.X+n.W && y >= n.Y && y <= n.Y+n.H {
			return true
		}
	}
	for _, child := range n.Children {
		if nodeContains(child, x, y) {
			return true
		}
	}
	return false
}
```

In `internal/render/render.go`:
Replace `BandContains(x, y)`:
```go
// Contains reports whether a surface point lies inside any drawn interactive element
// of the active scene frame.
func (r *Renderer) Contains(x, y float64) bool {
	if r.lastFrame != nil {
		return r.lastFrame.Contains(x, y)
	}
	return false
}
```

In `internal/cli/run.go`:
```go
	in.OnPress(func(x, y float64) {
		if !l.IsOpen() {
			return
		}
		if !rdr.Contains(x, y) {
			l.Close()
			return
		}
		_ = surfHost.SetKeyboardFocus(false)
		_ = surfHost.SetKeyboardFocus(true)
	})
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ksl/eval -run TestFrameContainsPointsInsideAndOutside -v`
Expected: PASS

- [x] **Step 5: Run render tests**

Run: `go test ./internal/render -v`
Expected: PASS

- [x] **Step 6: Commit changes**

```bash
git add internal/ksl/eval/hittest.go internal/ksl/eval/hittest_test.go internal/render/render.go internal/cli/run.go
git commit -m "feat(render): scene-driven hit-testing replacing hardcoded BandContains"
```

---

### Task 5: Unified Backend Registry & Dynamic Mode Generation

**Files:**
- Create: `internal/backend/registry.go`
- Create: `internal/backend/registry_test.go`
- Modify: `internal/cli/host.go:166-220, 340-370`
- Modify: `internal/cli/run.go:167-200`
- Modify: `internal/launcher/launcher.go:425-430`

- [x] **Step 1: Write failing test for `ProviderRegistry`**

Create `internal/backend/registry_test.go`:
```go
package backend

import (
	"context"
	"testing"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

type dummyPlugin struct {
	info sdk.BackendInfo
}

func (d *dummyPlugin) Info() sdk.BackendInfo { return d.info }
func (d *dummyPlugin) Query(context.Context, sdk.Query) (sdk.Stream, error) {
	return sdk.NewStream(), nil
}
func (d *dummyPlugin) Control(context.Context, sdk.Change) error { return nil }
func (d *dummyPlugin) Invoke(context.Context, sdk.Invoke) error   { return nil }

func TestRegistryDynamicModesAndSearchable(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&dummyPlugin{info: sdk.BackendInfo{ID: "apps", Searchable: true}})
	reg.Register(&dummyPlugin{info: sdk.BackendInfo{ID: "calc", Searchable: false, Trigger: "="}})
	reg.Register(&dummyPlugin{info: sdk.BackendInfo{ID: "tray", Searchable: true, Trigger: ","}})

	searchable := reg.SearchableIDs()
	if len(searchable) != 2 || searchable[0] != "apps" || searchable[1] != "tray" {
		t.Errorf("searchable = %v, want [apps tray]", searchable)
	}

	modes := reg.Modes()
	if len(modes) != 4 || modes[0] != "" || modes[1] != "apps" || modes[2] != "tray" || modes[3] != "calc" {
		t.Errorf("modes = %v, want ['' apps tray calc]", modes)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/backend -run TestRegistryDynamicModesAndSearchable -v`
Expected: FAIL with `undefined: NewRegistry`

- [x] **Step 3: Implement `ProviderRegistry` and `InProcAdapter`**

Create `internal/backend/registry.go`:
```go
package backend

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
)

// ProviderRegistry manages all registered providers (in-proc and subprocess).
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]Backend
	order     []string
}

func NewRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Backend),
	}
}

func (r *ProviderRegistry) Register(p sdk.Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	info := p.Info()
	r.providers[info.ID] = NewInProcPluginAdapter(p)
	r.order = append(r.order, info.ID)
}

func (r *ProviderRegistry) RegisterBackend(id string, b Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[id] = b
	r.order = append(r.order, id)
}

func (r *ProviderRegistry) Get(id string) (Backend, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("unknown backend %q", id)
	}
	return b, nil
}

func (r *ProviderRegistry) SearchableIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	for _, id := range r.order {
		if b, ok := r.providers[id]; ok && b.Info().Searchable {
			out = append(out, id)
		}
	}
	return out
}

func (r *ProviderRegistry) Modes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	modes := []string{""} // MergedMode first
	var nonSearchable []string
	for _, id := range r.order {
		if b, ok := r.providers[id]; ok {
			if b.Info().Searchable {
				modes = append(modes, id)
			} else {
				nonSearchable = append(nonSearchable, id)
			}
		}
	}
	return append(modes, nonSearchable...)
}

// InProcPluginAdapter wraps sdk.Plugin as backend.Backend
type InProcPluginAdapter struct {
	p sdk.Plugin
}

func NewInProcPluginAdapter(p sdk.Plugin) Backend {
	return &InProcPluginAdapter{p: p}
}

func (a *InProcPluginAdapter) Info() Info {
	info := a.p.Info()
	return Info{
		ID:         info.ID,
		Name:       info.Name,
		Badge:      info.Badge,
		Trigger:    info.Trigger,
		Searchable: info.Searchable,
		StableIDs:  info.StableIDs,
	}
}

func (a *InProcPluginAdapter) Query(ctx context.Context, q Query) (Stream, error) {
	s, err := a.p.Query(ctx, sdk.Query{Text: q.Text, Limit: q.Limit, Merged: q.Merged, Scope: q.Scope})
	if err != nil {
		return nil, err
	}
	return &sdkStreamAdapter{s: s}, nil
}

func (a *InProcPluginAdapter) Control(ctx context.Context, c Change) error {
	return a.p.Control(ctx, sdk.Change{Item: c.Item, Control: c.Control, Value: c.Value})
}

func (a *InProcPluginAdapter) Invoke(ctx context.Context, i Invoke) error {
	return a.p.Invoke(ctx, sdk.Invoke{Item: i.Item, Action: i.Action})
}

func (a *InProcPluginAdapter) Close() error {
	return nil
}

type sdkStreamAdapter struct {
	s sdk.Stream
}

func (sa *sdkStreamAdapter) Next() (Item, bool) {
	it, ok := sa.s.Next()
	if !ok {
		return Item{}, false
	}
	return itemFromSDK(it), true
}

func (sa *sdkStreamAdapter) Close() error {
	return sa.s.Close()
}
```

In `internal/cli/host.go` and `internal/cli/run.go`:
Eliminate static `switch id` statements. Register plugins via registry.
Derive `newLauncherOptions(reg *backend.ProviderRegistry)` dynamically:
```go
func newLauncherOptions(cfg config.Config, reg *backend.ProviderRegistry) launcher.Options {
	return launcher.Options{
		SettleMs:   settleDuration(cfg.Overlay.SettleMs),
		RankOpts:   rank.Options{Limit: cfg.Overlay.MaxResults},
		Searchable: reg.SearchableIDs(),
		Modes:      reg.Modes(),
	}
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/backend -run TestRegistryDynamicModesAndSearchable -v`
Expected: PASS

- [x] **Step 5: Run full CLI and launcher test suite**

Run: `go test ./internal/backend ./internal/cli ./internal/launcher -v`
Expected: PASS

- [x] **Step 6: Commit changes**

```bash
git add internal/backend/registry.go internal/backend/registry_test.go internal/cli/ internal/launcher/
git commit -m "feat(backend): implement unified ProviderRegistry eliminating hardcoded backend branching"
```

---

### Task 6: Full Verification & Multi-Platform Cross-Compilation

**Files:**
- Test: All repository packages

- [x] **Step 1: Test full repository test suite**

Run: `go test -count=1 ./...`
Expected: PASS across all packages

- [x] **Step 2: Test race detector on core modules**

Run: `CGO_ENABLED=1 go test -race -count=1 ./internal/launcher ./internal/render ./internal/backend ./internal/ksl/...`
Expected: PASS (0 race conditions)

- [x] **Step 3: Test Linux cross-compilation**

Run: `GOOS=linux go build ./...`
Expected: exit code 0

- [x] **Step 4: Test macOS cross-compilation**

Run: `GOOS=darwin go build ./...`
Expected: exit code 0

- [x] **Step 5: Test Windows cross-compilation**

Run: `GOOS=windows go build ./...`
Expected: exit code 0

- [x] **Step 6: Verify linter and formatting**

Run: `gofmt -l . && go vet ./...`
Expected: 0 unformatted files, 0 vet issues
