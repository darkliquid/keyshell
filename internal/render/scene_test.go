package render

import (
	"os"
	"strings"
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

	budget := 50 * time.Millisecond
	if isRaceEnabled() {
		budget = 500 * time.Millisecond
	}
	if dur > budget {
		t.Errorf("DrawFrame took %v, want < %v for virtualized dropdown", dur, budget)
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

func TestDropdownSceneControls(t *testing.T) {
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

	matches := []rank.Match{{
		Item: backend.Item{
			ID:       backend.ResultID("media-player"),
			Title:    "Music Player",
			Subtitle: "Now Playing: Track 1",
			Icon:     "multimedia-audio-player",
		},
	}}

	panelRows := []backend.PanelRow{
		{Kind: "action", Label: "Play / Pause", Action: &backend.Action{Type: "exec"}},
		{Kind: backend.ControlRange, Label: "Volume", Control: &backend.Control{ID: "vol", Value: 0.75, Min: 0, Max: 1, Step: 0.05}},
		{Kind: backend.ControlToggle, Label: "Shuffle", Control: &backend.Control{ID: "shuffle", Bool: true}},
		{Kind: backend.ControlChoice, Label: "Repeat", Control: &backend.Control{ID: "repeat", Text: "All", Options: []string{"Off", "Track", "All"}}},
		{Kind: backend.ControlText, Label: "Search Track", Control: &backend.Control{ID: "q", Text: "Bohemian Rhapsody"}},
	}

	view := &fakeView{
		open:       true,
		query:      "music",
		results:    matches,
		selected:   0,
		panelOpen:  true,
		panelFocus: 1,
		panelDirty: false,
		panelRows:  panelRows,
	}

	dc, img := testContext(t, 1920, 1080)
	sr.DrawFrame(dc, view, th, cfg.Reveal)

	// Verify no pixels leak outside the 640x460 dropdown bounds centered at (1920-640)/2=640, y=120
	outsideLeaks := 0
	for y := 0; y < 1080; y += 15 {
		for x := 0; x < 1920; x += 15 {
			if x >= 640 && x <= 1280 && y >= 120 && y <= 580 {
				continue
			}
			if img.RGBAAt(x, y).A != 0 {
				outsideLeaks++
			}
		}
	}
	if outsideLeaks > 0 {
		t.Errorf("found %d painted pixels outside dropdown bounds", outsideLeaks)
	}

	// Verify that inside the dropdown box, items are actively rendered
	insidePainted := 0
	for y := 125; y < 575; y += 10 {
		for x := 645; x < 1275; x += 10 {
			if img.RGBAAt(x, y).A != 0 {
				insidePainted++
			}
		}
	}
	if insidePainted < 50 {
		t.Errorf("dropdown control rows did not paint sufficient pixels inside bounds (insidePainted=%d)", insidePainted)
	}
}

func TestDropdownSceneSelectionHighlightInset(t *testing.T) {
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

	matches := []rank.Match{{
		Item: backend.Item{
			ID:       backend.ResultID("item1"),
			Title:    "Test Item",
			Subtitle: "Subtitle",
			Icon:     "application-x-executable",
		},
	}}

	view := &fakeView{
		open:     true,
		query:    "test",
		results:  matches,
		selected: 0,
	}

	dc, img := testContext(t, 1920, 1080)
	sr.DrawFrame(dc, view, th, cfg.Reveal)

	// Window is at X=640..1280.
	// In the item list region (y ~ 200), verify that the margin at X=644
	// is painted with $slab rather than the selection fill $surface-alt.
	marginPx := img.RGBAAt(644, 200)
	selectedPx := img.RGBAAt(650, 200)

	// $surface-alt and $slab have different color values
	if marginPx == selectedPx {
		t.Errorf("selection highlight is not inset from left border: margin=%+v selected=%+v", marginPx, selectedPx)
	}

	// Right margin at X=1276 should also match $slab rather than $surface-alt
	rightMarginPx := img.RGBAAt(1276, 200)
	if rightMarginPx == selectedPx {
		t.Errorf("selection highlight is not inset from right border: rightMargin=%+v selected=%+v", rightMarginPx, selectedPx)
	}
}

func TestDropdownSceneLongItemNoOverflow(t *testing.T) {
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

	longTitle := strings.Repeat("ExtremelyLongTitleWithoutSpacesOrWithSpacesThatExceedsTheWindowWidthByALot ", 10)
	longSub := strings.Repeat("ExtremelyLongSubtitlePath/home/darkliquid/Projects/deeply/nested/directory/structure/file.txt ", 10)

	matches := []rank.Match{{
		Item: backend.Item{
			ID:       backend.ResultID("overflow-test"),
			Title:    longTitle,
			Subtitle: longSub,
			Icon:     "application-x-executable",
			Badge:    "VERY-LONG-BADGE",
			Status:   "ACTIVE",
		},
	}}

	view := &fakeView{
		open:     true,
		query:    "overflow",
		results:  matches,
		selected: 0,
	}

	dc, img := testContext(t, 1920, 1080)
	sr.DrawFrame(dc, view, th, cfg.Reveal)

	// Verify no pixels leak outside the 640x460 dropdown bounds [640..1280, 120..580]
	outsideLeaks := 0
	for y := 0; y < 1080; y += 10 {
		for x := 0; x < 1920; x += 10 {
			if x >= 640 && x <= 1280 && y >= 120 && y <= 580 {
				continue
			}
			if img.RGBAAt(x, y).A != 0 {
				outsideLeaks++
			}
		}
	}
	if outsideLeaks > 0 {
		t.Errorf("long item caused %d painted pixels outside dropdown bounds", outsideLeaks)
	}
}

func TestSceneThemeVariablesOverride(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()

	src := `
theme {
    accent: #00FFC2
    card_width: 320
    custom_label: "scene_val"
}
Window {
    Box {
        fill: $accent
        width: $card_width
        label: $custom_label
    }
}
`
	s, err := parseScene("override_test.ksl", []byte(src), nil)
	if err != nil {
		t.Fatalf("parseScene failed: %v", err)
	}

	cfg := config.Default()
	cfg.Theme = map[string]any{
		"accent":       "#08F7FE",
		"card_width":   450.0,
		"custom_label": "config_val",
	}

	sr := NewSceneRenderer(New(), cfg)
	sr.SetScene(s)

	v := &fakeView{open: true}
	rev := Reveal{YLeft: 100, YRight: 120, HLeft: 64, HRight: 80, Angle: 0.02, Shear: -0.25}
	st := sr.state(v, th, rev, cfg.Reveal)

	// Check st.Vars contains coerced values from cfg.Theme
	wantColor := eval.Color{R: 0x08, G: 0xF7, B: 0xFE, A: 0xFF}
	if gotColor, ok := st.Vars["accent"].(eval.Color); !ok || gotColor != wantColor {
		t.Errorf("st.Vars[accent] = %#v (%T), want %#v", st.Vars["accent"], st.Vars["accent"], wantColor)
	}
	if gotWidth, ok := st.Vars["card_width"].(float64); !ok || gotWidth != 450.0 {
		t.Errorf("st.Vars[card_width] = %#v, want 450.0", st.Vars["card_width"])
	}
	if gotLabel, ok := st.Vars["custom_label"].(string); !ok || gotLabel != "config_val" {
		t.Errorf("st.Vars[custom_label] = %#v, want config_val", st.Vars["custom_label"])
	}

	// Resolve the scene with st and verify properties were overridden
	frame, err := eval.Resolve(s, st, eval.WithMeasurer(themeMeasurer{th}))
	if err != nil {
		t.Fatalf("eval.Resolve failed: %v", err)
	}
	if len(frame.Nodes) == 0 || len(frame.Root.Children) == 0 {
		t.Fatal("expected resolved frame with children")
	}
	box := frame.Root.Children[0]
	if c, ok := box.Props["fill"].(eval.Color); !ok || c != wantColor {
		t.Errorf("box.Props[fill] = %#v, want %#v", box.Props["fill"], wantColor)
	}
	if w, ok := box.Props["width"].(eval.Num); !ok || w != 450 {
		t.Errorf("box.Props[width] = %#v, want Num(450)", box.Props["width"])
	}
	if l, ok := box.Props["label"].(eval.Str); !ok || l != "config_val" {
		t.Errorf("box.Props[label] = %#v, want Str(config_val)", box.Props["label"])
	}
}

func TestSceneRendererSetConfig(t *testing.T) {
	sr := NewSceneRenderer(New(), config.Default())
	sr.SetConfig(config.Config{
		Theme: map[string]any{
			"card_width": int64(480),
			"accent":     "#00FF00",
		},
	})
	vars := sr.themeVars()
	if vars == nil {
		t.Fatal("expected non-nil vars")
	}
	if w, ok := vars["card_width"].(float64); !ok || w != 480.0 {
		t.Errorf("vars[card_width] = %#v (%T), want float64(480)", vars["card_width"], vars["card_width"])
	}
	wantColor := eval.Color{R: 0x00, G: 0xFF, B: 0x00, A: 0xFF}
	if c, ok := vars["accent"].(eval.Color); !ok || c != wantColor {
		t.Errorf("vars[accent] = %#v (%T), want %#v", vars["accent"], vars["accent"], wantColor)
	}
}

func TestMultiSummonBandRandomization(t *testing.T) {
	th, err := theme.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer th.Close()

	cfg := config.Default()
	sr := NewSceneRenderer(New(), cfg)
	if !sceneDefinesReveal(sr.scene) {
		t.Fatal("expected default scene to define reveal")
	}

	dc, _ := testContext(t, 1920, 1080)
	v := &fakeView{open: false, query: "keyshell"}

	// 1. Initial closed state
	sr.DrawFrame(dc, v, th, cfg.Reveal)

	// 2. First summon (open)
	v.open = true
	sr.DrawFrame(dc, v, th, cfg.Reveal)

	sr.inner.mu.Lock()
	rev1 := sr.inner.reveal
	sr.inner.mu.Unlock()

	if rev1.HLeft == 0 && rev1.HRight == 0 {
		t.Fatalf("expected non-zero reveal geometry after first summon, got %+v", rev1)
	}

	// Another frame in the same summon must be deterministic
	sr.DrawFrame(dc, v, th, cfg.Reveal)
	sr.inner.mu.Lock()
	rev1Frame2 := sr.inner.reveal
	sr.inner.mu.Unlock()
	if rev1 != rev1Frame2 {
		t.Fatalf("expected deterministic reveal within same summon, got %+v vs %+v", rev1, rev1Frame2)
	}

	// 3. Close launcher
	v.open = false
	sr.DrawFrame(dc, v, th, cfg.Reveal)

	// 4. Second summon (open)
	v.open = true
	sr.DrawFrame(dc, v, th, cfg.Reveal)

	sr.inner.mu.Lock()
	rev2 := sr.inner.reveal
	sr.inner.mu.Unlock()

	if rev2.HLeft == 0 && rev2.HRight == 0 {
		t.Fatalf("expected non-zero reveal geometry after second summon, got %+v", rev2)
	}

	if rev1 == rev2 {
		t.Fatalf("expected different Reveal geometries across summons, got identical: %+v", rev1)
	}
	if rev1.Angle == rev2.Angle && rev1.YLeft == rev2.YLeft && rev1.YRight == rev2.YRight {
		t.Fatalf("expected different angle/offsets across summons, got %+v vs %+v", rev1, rev2)
	}
}
