package render

import (
	"os"
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/anim"
	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/ksl"
	"github.com/darkliquid/keyshell/internal/ksl/eval"
	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/theme"
)

func TestLoadSceneDefault(t *testing.T) {
	s, err := LoadScene(config.Default())
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
	s, err := LoadScene(cfg)
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if len(s.Nodes()[0].Children) != 1 || s.Nodes()[0].Children[0].Name != "Box" {
		t.Fatalf("override root = %+v", s.Nodes()[0])
	}
}

func TestLoadSceneReportsBrokenOverride(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ui.ksl"
	os.WriteFile(path, []byte("Window { Box { fill"), 0o644) // broken
	cfg := config.Default()
	cfg.UI.Scene = path
	if _, err := LoadScene(cfg); err == nil {
		t.Fatal("broken override must return an error, so a live edit can keep the previous scene")
	}
}

func TestNewSceneRendererFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ui.ksl"
	os.WriteFile(path, []byte("Window { Gizmo { } }"), 0o644)
	cfg := config.Default()
	cfg.UI.Scene = path
	// A broken user scene must not stop the overlay from opening: the
	// renderer falls back to the embedded default.
	r := NewConfigured(cfg)
	sr := r.scene
	if sr == nil || len(sr.scene.Nodes()) != 1 || sr.scene.Nodes()[0].Name != "Window" {
		t.Fatalf("fallback root = %+v", sr.scene)
	}
}

func TestStateFromView(t *testing.T) {
	v := &fakeView{open: true, query: "fire", mode: "apps",
		results:  []rank.Match{{Item: backend.Item{ID: "x", Title: "Firefox"}}},
		selected: 0}
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	sr := NewSceneRenderer(New(), config.Default())
	rev := Reveal{YLeft: 100, YRight: 120, HLeft: 64, HRight: 80, Angle: 0.02, Shear: -0.25}
	st := sr.state(v, th, rev, config.Default().Reveal)
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

func TestStateFromViewPanel(t *testing.T) {
	v := &fakeView{open: true, mode: "audio",
		panelOpen: true, panelFocus: 1, panelDirty: true,
		panelRows: []backend.PanelRow{
			{Kind: "action", Label: "RUN", Action: &backend.Action{Type: "exec"}},
			{Kind: backend.ControlRange, Label: "Volume", Control: &backend.Control{ID: "volume", Value: 0.5, Min: 0, Max: 1, Step: 0.01}},
		}}
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()
	sr := NewSceneRenderer(New(), config.Default())
	rev := Reveal{YLeft: 100, YRight: 120, HLeft: 64, HRight: 80, Angle: 0.02, Shear: -0.25}
	st := sr.state(v, th, rev, config.Default().Reveal)
	if !st.Panel.Open || st.Panel.Focus != 1 || !st.Panel.Dirty {
		t.Errorf("panel = %+v, want open focus=1 dirty", st.Panel)
	}
	if len(st.Panel.Rows) != 2 {
		t.Fatalf("panel rows = %d, want 2", len(st.Panel.Rows))
	}
	if st.Panel.Rows[0].Kind != "action" || st.Panel.Rows[0].Title != "RUN" {
		t.Errorf("row0 = %+v, want action row", st.Panel.Rows[0])
	}
	if st.Panel.Rows[1].Kind != string(backend.ControlRange) || st.Panel.Rows[1].Ratio != 0.5 {
		t.Errorf("row1 = %+v, want range row at 0.5", st.Panel.Rows[1])
	}
}

func TestClockDrivesBlink(t *testing.T) {
	sr := NewSceneRenderer(New(), config.Default())
	kf := sr.scene.Keyframes()[0]
	if kf.Name != "blink" {
		t.Fatalf("default scene keyframes = %+v", sr.scene.Keyframes())
	}
	step := ksl.AnimStep{Name: "blink", Dur: &ksl.Scalar{N: 800, Unit: "ms"},
		Easing: &ksl.Keyword{Name: "linear"}, Iteration: &ksl.Keyword{Name: "infinite"}}
	at0, err := anim.Sample(kf, step, 0)
	if err != nil {
		t.Fatal(err)
	}
	at400, err := anim.Sample(kf, step, 400*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if at0["opacity"] != eval.Num(1) || at400["opacity"] != eval.Num(0) {
		t.Errorf("blink = %v, %v", at0, at400)
	}
}

func TestSelectionTransitionTweensFill(t *testing.T) {
	scene := mustParseScene(t, `
		Window { Row { Box#card { fill: #0B0B0F } } }
		transition selected { fill: 200ms ease-out }
	`)
	sr := NewSceneRenderer(New(), config.Default())
	sr.scene = scene
	now := time.Unix(0, 0)
	sr.clock.Tick(now, map[string]bool{"open": true, "selected": false})

	// frame 1: selection moves to 0, arming the edge
	edge := now.Add(10 * time.Millisecond)
	sr.clock.Tick(edge, map[string]bool{"open": true, "selected": true})

	f := &eval.ResolvedNode{Name: "Window",
		Children: []*eval.ResolvedNode{{Name: "Row",
			Children: []*eval.ResolvedNode{{
				Name: "Box", ID: "card",
				Props: map[string]eval.Value{"fill": eval.Color{R: 0xFF, G: 0x2E, B: 0x63, A: 0xFF}},
			}}}}}
	// previous frame had slab fill
	sr.prevProps["/Window/Row/Box#card"] = map[string]eval.Value{"fill": eval.Color{R: 0x0B, G: 0x0B, B: 0x0F, A: 0xFF}}

	sr.applyTransitions(sr.scene, f, edge.Add(100*time.Millisecond), sr.prevProps)
	got := f.Children[0].Children[0].Props["fill"].(eval.Color)
	if got == (eval.Color{R: 0x0B, G: 0x0B, B: 0x0F, A: 0xFF}) || got == (eval.Color{R: 0xFF, G: 0x2E, B: 0x63, A: 0xFF}) {
		t.Errorf("mid-transition fill = %v, want between slab and accent", got)
	}
}

func mustParseScene(t *testing.T, src string) *ksl.Scene {
	t.Helper()
	s, err := ksl.Parse("t.ksl", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := ksl.ValidateWith(s, "Card"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return s
}

func TestDropdownSceneRenders(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()

	cfg := config.Default()
	cfg.UI.Scene = "../../scenes/dropdown.ksl"
	s, err := LoadScene(cfg)
	if err != nil {
		t.Fatalf("LoadScene(dropdown.ksl): %v", err)
	}

	sr := NewSceneRenderer(New(), cfg)
	sr.SetScene(s)

	view := &fakeView{
		open:  true,
		query: "term",
		results: []rank.Match{
			{Item: backend.Item{ID: "app1", Title: "Alacritty", Subtitle: "Terminal Emulator", Icon: "terminal", Badge: "APP", Status: "OPEN"}},
			{Item: backend.Item{ID: "app2", Title: "Firefox", Subtitle: "Web Browser", Icon: "firefox", Badge: "APP"}},
		},
		selected: 0,
	}

	dc, img := testContext(t, 1920, 1080)
	sr.DrawFrame(dc, view, th, cfg.Reveal)

	if sr.UsesBandMask() {
		t.Error("dropdown scene must not use the band mask")
	}

	if !anyColor(img) {
		t.Fatal("dropdown scene did not paint any pixels")
	}

	// Verify hit testing: inside dropdown box (640..1280, 120..580) vs outside
	rdr := NewConfigured(cfg)
	rdr.scene = sr
	if !rdr.BandContains(960, 300) {
		t.Error("BandContains(960, 300) = false, want true for dropdown window")
	}
	if rdr.BandContains(100, 100) {
		t.Error("BandContains(100, 100) = true, want false outside dropdown window")
	}

	// Verify pixels inside the dropdown box (x centered: (1920-640)/2 = 640..1280, y: 120..580)
	insideColor := false
	for y := 130; y < 200; y += 10 {
		for x := 650; x < 800; x += 10 {
			if img.RGBAAt(x, y).A != 0 {
				insideColor = true
				break
			}
		}
	}
	if !insideColor {
		t.Error("no pixels painted inside dropdown window region")
	}

	// Verify NO content leaked outside the dropdown bounds (e.g. at screen edge x=16 or y=0)
	outsideLeaks := 0
	for y := 0; y < 1080; y += 20 {
		for x := 0; x < 1920; x += 20 {
			if x >= 640 && x <= 1280 && y >= 120 && y <= 580 {
				continue // inside dropdown box
			}
			if img.RGBAAt(x, y).A != 0 {
				outsideLeaks++
			}
		}
	}
	if outsideLeaks > 0 {
		t.Errorf("found %d sampled painted pixels outside dropdown bounds [640..1280, 120..580]", outsideLeaks)
	}
}

func TestDropdownSceneLargeResultVirtualizationAndClipping(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()

	cfg := config.Default()
	cfg.UI.Scene = "../../scenes/dropdown.ksl"
	s, err := LoadScene(cfg)
	if err != nil {
		t.Fatalf("LoadScene(dropdown.ksl): %v", err)
	}

	sr := NewSceneRenderer(New(), cfg)
	sr.SetScene(s)

	// Build 200 items to stress test virtualization and clipping
	matches := make([]rank.Match, 200)
	for i := range matches {
		matches[i] = rank.Match{Item: backend.Item{
			ID:       backend.ResultID("app" + itoa(i)),
			Title:    "Application " + itoa(i),
			Subtitle: "Description for item " + itoa(i),
			Icon:     "application-x-executable",
			Badge:    "APP",
		}}
	}

	view := &fakeView{
		open:     true,
		query:    "app",
		results:  matches,
		selected: 0,
	}

	// Test case 1: selected = 0 (top)
	start := time.Now()
	dc, img := testContext(t, 1920, 1080)
	sr.DrawFrame(dc, view, th, cfg.Reveal)
	dur := time.Since(start)

	if dur > 50*time.Millisecond {
		t.Errorf("DrawFrame took %v, want < 50ms for virtualized dropdown", dur)
	}

	checkLeaks := func(t *testing.T, label string) {
		t.Helper()
		outsideLeaks := 0
		for y := 0; y < 1080; y += 15 {
			for x := 0; x < 1920; x += 15 {
				if x >= 640 && x <= 1280 && y >= 120 && y <= 580 {
					continue // inside dropdown box
				}
				if img.RGBAAt(x, y).A != 0 {
					outsideLeaks++
				}
			}
		}
		if outsideLeaks > 0 {
			t.Errorf("[%s] found %d sampled painted pixels outside dropdown bounds [640..1280, 120..580]", label, outsideLeaks)
		}
	}

	checkLeaks(t, "selected=0")

	// Test case 2: selected = 50 (middle of the list)
	view.selected = 50
	dc, img = testContext(t, 1920, 1080)
	sr.DrawFrame(dc, view, th, cfg.Reveal)
	checkLeaks(t, "selected=50")

	// Test case 3: selected = 199 (bottom of the list)
	view.selected = 199
	dc, img = testContext(t, 1920, 1080)
	sr.DrawFrame(dc, view, th, cfg.Reveal)
	checkLeaks(t, "selected=199")
}
