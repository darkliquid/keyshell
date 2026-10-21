package render

import (
	"os"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/ksl/eval"
	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/theme"
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
	if len(s.Nodes()) != 1 || s.Nodes()[0].Name != "Window" {
		t.Fatalf("fallback root = %+v", s)
	}
}

func TestLoadSceneRejectsUnknownNodes(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ui.ksl"
	os.WriteFile(path, []byte("Window { Gizmo { } }"), 0o644)
	cfg := config.Default()
	cfg.UI.Scene = path
	s, err := loadScene(cfg)
	if err != nil {
		t.Fatalf("invalid override must fall back, not fail: %v", err)
	}
	// fallback is the default scene, so no Gizmo
	if len(s.Nodes()[0].Children) == 0 || s.Nodes()[0].Children[0].Name == "Gizmo" {
		t.Errorf("invalid scene was accepted")
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
