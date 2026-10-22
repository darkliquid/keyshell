package eval

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl"
)

func TestCascadeOrder(t *testing.T) {
	st := &State{Open: true}
	env := NewEnv(st, nil)
	scene, err := ksl.Parse("t.ksl", []byte(`
		Text#title { color: #FF2E63 }
		Text.muted { color: #5A5A66, size: 12 }
		Window { Text#title.muted { size: 48, color: #FFFFFF } }
	`))
	if err != nil {
		t.Fatal(err)
	}
	n := scene.Nodes()[0].Children[0]
	ctx := &CascadeCtx{Scene: scene}
	props, ok, err := resolveProps(n, env, ctx)
	if err != nil || !ok {
		t.Fatalf("resolveProps: %v, %v", err, ok)
	}
	// node-local beats selectors; id beats class regardless of order
	if props["color"] != (Color{0xFF, 0xFF, 0xFF, 0xFF}) {
		t.Errorf("color = %v, want node-local #FFFFFF", props["color"])
	}
	if props["size"] != Num(48) {
		t.Errorf("size = %v", props["size"])
	}
}

func TestCascadeSpecificityWithoutLocal(t *testing.T) {
	scene, _ := ksl.Parse("t.ksl", []byte(`
		Text.muted { color: #5A5A66 }
		Text#title { color: #FF2E63 }
		Window { Text#title.muted { } }
	`))
	n := scene.Nodes()[0].Children[0]
	props, _, err := resolveProps(n, NewEnv(&State{}, nil), &CascadeCtx{Scene: scene})
	if err != nil {
		t.Fatal(err)
	}
	// id selector wins over class even when declared first
	if props["color"] != (Color{0xFF, 0x2E, 0x63, 0xFF}) {
		t.Errorf("color = %v, want id result", props["color"])
	}
}

func TestNodeAndSceneStates(t *testing.T) {
	scene, err := ksl.Parse("t.ksl", []byte(`
		Window { Box#card { fill: #000000, state selected { fill: #FF0000 } } }
		state open { Box#card { opacity: 1 } }
	`))
	if err != nil {
		t.Fatal(err)
	}
	n := scene.Nodes()[0].Children[0]

	open := &State{Open: true}
	props, _, err := resolveProps(n, NewEnv(open, nil), &CascadeCtx{Scene: scene, Pseudo: map[string]bool{"selected": true}})
	if err != nil {
		t.Fatal(err)
	}
	if props["fill"] != (Color{0xFF, 0, 0, 0xFF}) {
		t.Errorf("selected fill = %v", props["fill"])
	}
	if props["opacity"] != Num(1) {
		t.Errorf("scene state opacity = %v", props["opacity"])
	}

	closed := &State{Open: false}
	props, _, err = resolveProps(n, NewEnv(closed, nil), &CascadeCtx{Scene: scene})
	if err != nil {
		t.Fatal(err)
	}
	if _, has := props["opacity"]; has {
		t.Errorf("inactive scene state leaked: %v", props)
	}
}

// TestSearchingBindingAndState proves the searching flag reaches the scene
// both ways: as a $searching when-guard binding and as a built-in state.
func TestSearchingBindingAndState(t *testing.T) {
	scene, err := ksl.Parse("t.ksl", []byte(`
		Window {
			Box#busy { when: $searching }
			Box#strip { }
		}
		state searching { Box#strip { opacity: 1 } }
	`))
	if err != nil {
		t.Fatal(err)
	}
	guarded := scene.Nodes()[0].Children[0]
	strip := scene.Nodes()[0].Children[1]

	busy := &State{Open: true, Searching: true}
	_, exists, err := resolveProps(guarded, NewEnv(busy, nil), &CascadeCtx{Scene: scene})
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("when: $searching dropped the node while searching")
	}
	props, _, err := resolveProps(strip, NewEnv(busy, nil), &CascadeCtx{Scene: scene})
	if err != nil {
		t.Fatal(err)
	}
	if props["opacity"] != Num(1) {
		t.Errorf("searching state opacity = %v, want 1", props["opacity"])
	}

	idle := &State{Open: true}
	_, exists, err = resolveProps(guarded, NewEnv(idle, nil), &CascadeCtx{Scene: scene})
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("when: $searching kept the node while idle")
	}
	props, _, err = resolveProps(strip, NewEnv(idle, nil), &CascadeCtx{Scene: scene})
	if err != nil {
		t.Fatal(err)
	}
	if _, has := props["opacity"]; has {
		t.Errorf("inactive searching state leaked: %v", props)
	}
}

func TestAttributeSelector(t *testing.T) {
	scene, _ := ksl.Parse("t.ksl", []byte(`
		Text[mode="media"] { color: #FF0000 }
		Window { Text { } }
	`))
	n := scene.Nodes()[0].Children[0]
	props, _, err := resolveProps(n, NewEnv(&State{Mode: "media"}, nil), &CascadeCtx{Scene: scene, Mode: "media"})
	if err != nil {
		t.Fatal(err)
	}
	if props["color"] != (Color{0xFF, 0, 0, 0xFF}) {
		t.Errorf("attr-matched color = %v", props["color"])
	}
	props, _, _ = resolveProps(n, NewEnv(&State{Mode: "apps"}, nil), &CascadeCtx{Scene: scene, Mode: "apps"})
	if _, has := props["color"]; has {
		t.Errorf("attr selector leaked into another mode: %v", props)
	}
}

func TestWhenGuard(t *testing.T) {
	scene, _ := ksl.Parse("t.ksl", []byte(`Window {
	    Text { text: "a", when: $mode == "media" }
	    Text { text: "b", when: $results.len > 0 }
	}`))
	st := &State{Mode: "apps", Results: []Item{{}}}
	ctx := &CascadeCtx{Scene: scene}
	_, okA, err := resolveProps(scene.Nodes()[0].Children[0], NewEnv(st, nil), ctx)
	if err != nil || okA {
		t.Errorf("mode mismatch should hide the node: ok=%v err=%v", okA, err)
	}
	_, okB, err := resolveProps(scene.Nodes()[0].Children[1], NewEnv(st, nil), ctx)
	if err != nil || !okB {
		t.Errorf("non-empty results should show the node: ok=%v err=%v", okB, err)
	}
}
