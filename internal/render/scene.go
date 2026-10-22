package render

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/darkliquid/keyshell/internal/anim"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/ksl"
	"github.com/darkliquid/keyshell/internal/ksl/eval"
	"github.com/darkliquid/keyshell/internal/telemetry"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

//go:embed default.ksl
var defaultScene string

// LoadScene reads and parses the configured scene file, or the embedded
// default when none is set. It returns an error instead of falling back, so
// hot reloads can report failures and keep the previous scene; startup
// callers decide their own fallback.
func LoadScene(cfg config.Config) (*ksl.Scene, error) {
	name, src := "default.ksl", []byte(defaultScene)
	if cfg.UI.Scene != "" {
		b, err := os.ReadFile(cfg.UI.Scene)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", cfg.UI.Scene, err)
		}
		name, src = cfg.UI.Scene, b
	}
	return parseScene(name, src, loaderFor(name, cfg))
}

func parseScene(name string, src []byte, load ksl.Loader) (*ksl.Scene, error) {
	s, err := ksl.ParseWithLoader(name, src, load)
	if err != nil {
		return nil, err
	}
	if err := ksl.ValidateWith(s, "Card"); err != nil {
		return nil, err
	}
	return s, nil
}

// itemFromBackend maps one backend item into the interpreter's schema.
func itemFromBackend(b backend.Item) eval.Item {
	controls := map[string]eval.ItemControl{}
	for _, c := range b.Controls {
		controls[c.ID] = eval.ItemControl{Value: c.Text, Ratio: c.Value, On: c.Bool}
	}
	return eval.Item{
		ID: string(b.ID), Title: b.Title, Subtitle: b.Subtitle,
		Badge: b.Badge, Status: b.Status, StatusTone: string(b.StatusTone),
		Glyph: b.Glyph, Icon: b.Icon, Image: b.Image, Preview: b.Preview,
		Detail: b.Detail, Hint: b.Hint, Controls: controls,
	}
}

// themeTokens maps the loaded palette into eval colors.
func themeTokens(th *theme.Theme) map[string]eval.Color {
	if th == nil {
		return nil
	}
	to8 := func(f float64) uint8 { return uint8(min(255, max(0, f*255))) }
	return map[string]eval.Color{
		"slab":        {R: to8(th.Slab.R), G: to8(th.Slab.G), B: to8(th.Slab.B), A: to8(th.Slab.A)},
		"surface-alt": {R: to8(th.SurfaceAlt.R), G: to8(th.SurfaceAlt.G), B: to8(th.SurfaceAlt.B), A: to8(th.SurfaceAlt.A)},
		"ink":         {R: to8(th.Ink.R), G: to8(th.Ink.G), B: to8(th.Ink.B), A: to8(th.Ink.A)},
		"accent":      {R: to8(th.Accent.R), G: to8(th.Accent.G), B: to8(th.Accent.B), A: to8(th.Accent.A)},
		"accent-alt":  {R: to8(th.AccentAlt.R), G: to8(th.AccentAlt.G), B: to8(th.AccentAlt.B), A: to8(th.AccentAlt.A)},
		"muted":       {R: to8(th.Muted.R), G: to8(th.Muted.G), B: to8(th.Muted.B), A: to8(th.Muted.A)},
	}
}

// loaderFor resolves includes relative to the scene file's directory.
func loaderFor(name string, cfg config.Config) ksl.Loader {
	if cfg.UI.Scene == "" || name == "default.ksl" {
		return nil
	}
	dir := filepath.Dir(name)
	return func(rel string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, rel))
	}
}

// themeMeasurer sizes text through the loaded theme, so layouts know
// how wide and tall text is.
type themeMeasurer struct {
	th *theme.Theme
}

func (m themeMeasurer) TextSize(text string, size float64, _ string) (float64, float64) {
	if m.th == nil {
		return 0, size
	}
	face := m.th.Face(size)
	return face.Advance(text), face.Metrics().LineHeight()
}

// SceneRenderer drives the frame from a resolved scene. The scene can be
// swapped live when the file changes on disk.
type SceneRenderer struct {
	inner       *Renderer
	scene       *ksl.Scene
	sceneMu     sync.Mutex
	summonEpoch int
	seed        uint64
	randCache   *eval.RandCache
	stateTokens map[string]eval.Color
	clock       *anim.Clock
	prevProps   map[string]map[string]eval.Value
}

// NewSceneRenderer builds a renderer for the configured scene.
func NewSceneRenderer(r *Renderer, cfg config.Config) *SceneRenderer {
	s, err := LoadScene(cfg)
	if err != nil {
		// A broken user scene must never keep the overlay from opening:
		// fall back to the embedded default.
		slog.Warn("ksl: falling back to the embedded default scene", "err", err)
		s, err = parseScene("default.ksl", []byte(defaultScene), nil)
		if err != nil {
			panic(err) // the embedded default is a build-time invariant
		}
	}
	return &SceneRenderer{inner: r, scene: s, seed: 1, randCache: &eval.RandCache{}, clock: anim.NewClock(time.Now()), prevProps: map[string]map[string]eval.Value{}}
}

// SetScene swaps the scene for every subsequent frame. Live scene editing
// calls this from the watcher; in-flight animation state is reset because
// node identities may have changed.
func (sr *SceneRenderer) SetScene(s *ksl.Scene) {
	sr.sceneMu.Lock()
	sr.scene = s
	sr.randCache = &eval.RandCache{}
	sr.prevProps = map[string]map[string]eval.Value{}
	sr.sceneMu.Unlock()
}

// snapshot returns the scene and the previous-props map the frame pass
// works on. A SetScene during the frame swaps in a fresh pair, so the
// frame keeps a consistent snapshot.
func (sr *SceneRenderer) snapshot() (*ksl.Scene, map[string]map[string]eval.Value) {
	sr.sceneMu.Lock()
	defer sr.sceneMu.Unlock()
	return sr.scene, sr.prevProps
}
func (sr *SceneRenderer) state(l View, th *theme.Theme, rev Reveal, cfg config.Reveal) eval.State {
	results := l.Results()
	items := make([]eval.Item, len(results))
	for i, m := range results {
		items[i] = itemFromBackend(m.Item)
	}
	if th != nil && sr.stateTokens == nil {
		sr.stateTokens = themeTokens(th)
	}
	sw, sh := sr.surfaceWidth(), sr.surfaceHeight()
	if sw <= 0 {
		sw, sh = 1600, 900
	}
	mode := l.ModeBadge()
	if mode == "" {
		mode = "SEARCH"
	}
	return eval.State{
		Name:      "keyshell",
		Open:      l.IsOpen(),
		Focused:   true,
		Query:     l.Query(),
		Mode:      mode,
		Selected:  l.Selected(),
		Results:   items,
		Searching: l.Searching(),
		Reveal: eval.Reveal{
			Y: rev.Yc, H: minF(rev.HLeft, rev.HRight),
			HLeft: rev.HLeft, HRight: rev.HRight,
			Angle: rev.Angle, Shear: rev.Shear, CardShear: rev.Shear,
			D:  bandD(rev, cfg, sw),
			Cw: contentW(rev, sw, sh), Ch: contentH(rev),
			FrameX: (sw - contentW(rev, sw, sh)) / 2,
			FrameY: rev.Yc - contentH(rev)/2,
		},
		Tokens:    sr.stateTokens,
		Fonts:     map[string]string{"display": theme.DefaultFace},
		Summon:    sr.summonEpoch,
		Seed:      sr.seed,
		RandCache: sr.randCache,
		Width:     sw,
		Height:    sh,
		Scale:     cfg.BandHeightPx / 290,
	}
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// surfaceWidth returns the renderer's cached surface width.
func (sr *SceneRenderer) surfaceWidth() float64 {
	sr.inner.mu.Lock()
	defer sr.inner.mu.Unlock()
	return sr.inner.surfaceW
}

func (sr *SceneRenderer) surfaceHeight() float64 {
	sr.inner.mu.Lock()
	defer sr.inner.mu.Unlock()
	return sr.inner.surfaceH
}

// bandD renders the trapezoid outline as a path string for a Path node.
func bandD(rev Reveal, cfg config.Reveal, w float64) string {
	if w <= 0 {
		w = 1600
	}
	halfL := rev.HLeft / 2
	halfR := rev.HRight / 2
	// Match legacy bandPath: left edge hangs off YLeft, right off YRight.
	return fmt.Sprintf("M 0 %g L %g %g L %g %g L 0 %g Z",
		rev.YLeft-halfL, w, rev.YRight-halfR, w, rev.YRight+halfR, rev.YLeft+halfL)
}

// DrawFrame paints one frame of the scene. It owns the reveal
// resample, the resolve, and the paint pass.
func (sr *SceneRenderer) DrawFrame(dc *gg.Context, l View, th *theme.Theme, cfg config.Reveal) {
	w, h := float64(dc.Width()), float64(dc.Height())

	l.Poll()
	if !l.IsOpen() {
		dc.ClearWithColor(gg.RGBA{})
		sr.inner.wasOpen = false
		clear(sr.prevProps)
		return
	}

	sr.inner.mu.Lock()
	if !sr.inner.wasOpen {
		sr.summonEpoch++
		sr.inner.reveal = Resample(w, h, cfg, sr.inner.rng)
		sr.inner.wasOpen = true
		sr.inner.opened = time.Now()
	}
	rev := sr.inner.reveal
	sr.inner.surfaceW, sr.inner.surfaceH = w, h
	elapsed := time.Since(sr.inner.opened)
	sr.inner.mu.Unlock()

	scene, prev := sr.snapshot()

	dc.ClearWithColor(gg.RGBA{})
	now := time.Now()
	st := sr.state(l, th, rev, cfg)
	resolveStart := time.Now()
	frame, err := eval.Resolve(scene, st, eval.WithMeasurer(themeMeasurer{th}))
	telemetry.ResolveDuration().Record(context.Background(), time.Since(resolveStart).Seconds())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksl: frame resolve: %v\n", err)
		return
	}
	sr.applyAnimations(frame.Root, now, scene)
	sr.clock.Tick(now, map[string]bool{"open": st.Open, "closed": !st.Open, "selected": st.Selected >= 0})
	sr.applyTransitions(scene, frame.Root, now, prev)
	deps := &PainterDeps{Reveal: rev, CardCfg: cfg, CursorAlpha: cursorAlpha(elapsed), Images: sr.inner.images, Icons: sr.inner.icons}
	painterFor("Window")(dc, th, frame.Root, deps)
}

// applyAnimations walks the resolved frame and samples any `animation`
// prop against the scene's keyframes at the given time, overlaying the
// sampled values on the node's props.
func (sr *SceneRenderer) applyAnimations(n *eval.ResolvedNode, now time.Time, scene *ksl.Scene) {
	if av, ok := n.Props["animation"].(eval.AnimVal); ok {
		for _, step := range av.A.Steps {
			kf := sr.keyframes(step.Name, scene)
			if kf == nil {
				continue
			}
			if sampled, err := anim.Sample(kf, step, now.Sub(sr.openTime())); err == nil {
				for k, v := range sampled {
					n.Props[k] = v
				}
			}
		}
	}
	for _, c := range n.Children {
		sr.applyAnimations(c, now, scene)
	}
}

func (sr *SceneRenderer) keyframes(name string, scene *ksl.Scene) *ksl.Keyframes {
	for _, kf := range scene.Keyframes() {
		if kf.Name == name {
			return kf
		}
	}
	return nil
}

func (sr *SceneRenderer) openTime() time.Time {
	sr.inner.mu.Lock()
	defer sr.inner.mu.Unlock()
	return sr.inner.opened
}

// applyTransitions tweens props that just changed as part of a fresh
// state edge. Each node's props are compared to the previous frame; a
// transition covering the flipped state and the property interpolates
// from the old value toward the new one.
func (sr *SceneRenderer) applyTransitions(scene *ksl.Scene, root *eval.ResolvedNode, now time.Time, prev map[string]map[string]eval.Value) {
	// collect states whose edge just fired
	fresh := map[string]bool{}
	for _, tr := range scene.Transitions() {
		if edge, ok := sr.clock.Edge(tr.State); ok && edge.After(now.Add(-time.Second)) {
			fresh[tr.State] = true
		}
	}
	if len(fresh) == 0 {
		sr.snapshotProps(root, "", prev)
		return
	}
	for _, tr := range scene.Transitions() {
		if !fresh[tr.State] {
			continue
		}
		edge, _ := sr.clock.Edge(tr.State)
		elapsed := now.Sub(edge)
		sr.tweenTree(root, tr, elapsed, "", prev)
	}
	sr.snapshotProps(root, "", prev)
}

func nodeKey(n *eval.ResolvedNode, path string) string {
	id := ""
	if n.ID != "" {
		id = "#" + n.ID
	}
	return path + "/" + n.Name + id
}

func (sr *SceneRenderer) tweenTree(n *eval.ResolvedNode, tr *ksl.Transition, elapsed time.Duration, path string, prevProps map[string]map[string]eval.Value) {
	key := nodeKey(n, path)
	prev := prevProps[key]
	if prev != nil {
		for k, tv := range n.Props {
			fv, ok := prev[k]
			if !ok {
				continue
			}
			tp := timingForProp(tr, k)
			if tp == nil {
				continue
			}
			easing := anim.MustEasing(tp.Easing)
			t := float64(elapsed/time.Millisecond) / anim.DurMs(tp.Dur)
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			if v, ok := anim.Lerp(fv, tv, easing(t)); ok {
				n.Props[k] = v
			}
		}
	}
	for _, c := range n.Children {
		sr.tweenTree(c, tr, elapsed, key, prevProps)
	}
}

func (sr *SceneRenderer) snapshotProps(n *eval.ResolvedNode, path string, prevProps map[string]map[string]eval.Value) {
	key := nodeKey(n, path)
	m := make(map[string]eval.Value, len(n.Props))
	for k, v := range n.Props {
		m[k] = v
	}
	prevProps[key] = m
	for _, c := range n.Children {
		sr.snapshotProps(c, key, prevProps)
	}
}

func timingForProp(tr *ksl.Transition, key string) *ksl.TimingProp {
	for _, tp := range tr.Props {
		if tp.Key == key {
			return tp
		}
	}
	return nil
}

// contentW is the width of the rotated content frame in the band.
func contentW(rev Reveal, w, h float64) float64 {
	return math.Sqrt(w*w + (rev.YRight-rev.YLeft)*(rev.YRight-rev.YLeft))
}

// contentH is the height of the rotated content frame.
func contentH(rev Reveal) float64 {
	return math.Min(rev.HLeft, rev.HRight) * math.Cos(rev.Angle)
}
