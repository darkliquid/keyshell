package render

import (
	"os"
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
