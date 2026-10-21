package render

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/darkliquid/keyshell/internal/anim"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/ksl"
	"github.com/darkliquid/keyshell/internal/ksl/eval"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
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
	s, err := parseScene(name, src, loaderFor(name, cfg))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksl: %v (using default scene)\n", err)
		s, err = parseScene("default.ksl", []byte(defaultScene), nil)
	}
	if err != nil {
		return nil, fmt.Errorf("embedded default scene: %w", err)
	}
	return s, nil
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
	to8 := func(f float64) uint8 { return uint8(min(255, max(0, f))) }
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

// SceneRenderer drives the frame from a resolved scene.
type SceneRenderer struct {
	inner       *Renderer
	scene       *ksl.Scene
	summonEpoch int
	seed        uint64
	randCache   *eval.RandCache
	stateTokens map[string]eval.Color
	clock       *anim.Clock
}

// NewSceneRenderer builds a renderer for the configured scene.
func NewSceneRenderer(r *Renderer, cfg config.Config) *SceneRenderer {
	s, err := loadScene(cfg)
	if err != nil {
		panic(err) // the embedded default is a build-time invariant
	}
	return &SceneRenderer{inner: r, scene: s, seed: 1, randCache: &eval.RandCache{}, clock: anim.NewClock(time.Now())}
}

// state assembles the binding snapshot for one frame. The reveal quad
// and the search geometry come from the host's resampled Reveal; the
// default scene binds to them.
func (sr *SceneRenderer) state(l View, th *theme.Theme, rev Reveal, cfg config.Reveal) eval.State {
	results := l.Results()
	items := make([]eval.Item, len(results))
	for i, m := range results {
		items[i] = itemFromBackend(m.Item)
	}
	if th != nil && sr.stateTokens == nil {
		sr.stateTokens = themeTokens(th)
	}
	return eval.State{
		Name:     "keyshell",
		Open:     l.IsOpen(),
		Focused:  true,
		Query:    l.Query(),
		Mode:     l.ModeBadge(),
		Selected: l.Selected(),
		Results:  items,
		Reveal: eval.Reveal{
			Y: rev.Yc, H: minF(rev.HLeft, rev.HRight),
			HLeft: rev.HLeft, HRight: rev.HRight,
			Angle: rev.Angle, Shear: rev.Shear, CardShear: rev.Shear,
			D: bandD(rev, cfg, 0),
		},
		Tokens:    sr.stateTokens,
		Fonts:     map[string]string{"display": theme.DefaultFace},
		Summon:    sr.summonEpoch,
		Seed:      sr.seed,
		RandCache: sr.randCache,
		Width:     sr.surfaceWidth(),
		Height:    sr.surfaceHeight(),
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
	return fmt.Sprintf("M 0 %g L %g %g L %g %g L 0 %g Z",
		rev.Yc-halfL, w, rev.YRight-halfR, w, rev.YRight+halfR, rev.Yc+halfL)
}

// DrawFrame paints one frame of the scene. It owns the reveal
// resample, the resolve, and the paint pass.
func (sr *SceneRenderer) DrawFrame(dc *gg.Context, l View, th *theme.Theme, cfg config.Reveal) {
	w, h := float64(dc.Width()), float64(dc.Height())

	l.Poll()
	if !l.IsOpen() {
		dc.ClearWithColor(gg.RGBA{})
		sr.inner.wasOpen = false
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

	dc.ClearWithColor(gg.RGBA{})
	now := time.Now()
	st := sr.state(l, th, rev, cfg)
	frame, err := eval.Resolve(sr.scene, st)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ksl: frame resolve: %v\n", err)
		return
	}
	sr.applyAnimations(frame.Root, now)
	deps := &PainterDeps{Reveal: rev, CardCfg: cfg, CursorAlpha: cursorAlpha(elapsed)}
	painterFor("Window")(dc, th, frame.Root, deps)
}

// applyAnimations walks the resolved frame and samples any `animation`
// prop against the scene's keyframes at the given time, overlaying the
// sampled values on the node's props.
func (sr *SceneRenderer) applyAnimations(n *eval.ResolvedNode, now time.Time) {
	if av, ok := n.Props["animation"].(eval.AnimVal); ok {
		for _, step := range av.A.Steps {
			kf := sr.keyframes(step.Name)
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
		sr.applyAnimations(c, now)
	}
}

func (sr *SceneRenderer) keyframes(name string) *ksl.Keyframes {
	for _, kf := range sr.scene.Keyframes() {
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
