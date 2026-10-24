package eval

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl"
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

func TestLetDeclAndBandInScene(t *testing.T) {
	src := `
theme {
    band_height: 290
    min_tilt: 5.0
    center_spread: 0.10
    taper_max: 0.15
    shear_min: 6.0
    shear_max: 14.0
}

let reveal = band($band_height, $min_tilt, $center_spread, $taper_max, $shear_min, $shear_max)

Window {
    Path { d: $reveal.d }
    Group {
        x: $reveal.frameX, y: $reveal.frameY
        width: $reveal.cw, height: $reveal.ch
        rotate: $reveal.angle
    }
}
`
	sc, err := ksl.Parse("test.ksl", []byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	st := State{Width: 1920, Height: 1080}
	frame, err := Resolve(sc, st)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(frame.Root.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(frame.Root.Children))
	}
	pathNode := frame.Root.Children[0]
	if d, ok := pathNode.Props["d"].(Str); !ok || len(d) == 0 {
		t.Errorf("expected non-empty d prop on path, got %v", pathNode.Props["d"])
	}
	groupNode := frame.Root.Children[1]
	if _, ok := groupNode.Props["rotate"]; !ok {
		t.Errorf("missing rotate prop on group")
	}
}

func TestBandHelperChangesWithSummonEpoch(t *testing.T) {
	args := []any{
		290.0, // height
		5.0,   // min_tilt
		0.10,  // spread
		0.15,  // taper
		6.0,   // shear_min
		14.0,  // shear_max
	}

	env0 := NewEnv(&State{Width: 1920, Height: 1080, Seed: 1, Summon: 0}, nil)
	val0, err := callBandHelper(env0, args)
	if err != nil {
		t.Fatalf("callBandHelper(summon 0) failed: %v", err)
	}
	m0, ok := val0.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", val0)
	}

	env1 := NewEnv(&State{Width: 1920, Height: 1080, Seed: 1, Summon: 1}, nil)
	val1, err := callBandHelper(env1, args)
	if err != nil {
		t.Fatalf("callBandHelper(summon 1) failed: %v", err)
	}
	m1, ok := val1.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", val1)
	}

	if m0["d"] == m1["d"] {
		t.Errorf("expected different 'd' across summon epochs, got identical: %v", m0["d"])
	}
	if m0["angle"] == m1["angle"] {
		t.Errorf("expected different 'angle' across summon epochs, got identical: %v", m0["angle"])
	}
}

func TestListTemplateInheritsRevealFromParent(t *testing.T) {
	src := `
let reveal = band(300.0, 5.0, 0.10, 0.15, 6.0, 14.0)

Window {
    List {
        items: $results
        template {
            Box {
                height: $reveal.ch
            }
        }
    }
}
`
	sc, err := ksl.Parse("test.ksl", []byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	st := State{
		Width:   1920,
		Height:  1080,
		Results: []Item{{ID: "1", Title: "One"}},
	}
	frame, err := Resolve(sc, st)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if len(frame.Root.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(frame.Root.Children))
	}
	listNode := frame.Root.Children[0]
	if len(listNode.Children) != 1 {
		t.Fatalf("expected 1 item rendered in List, got %d", len(listNode.Children))
	}
	boxNode := listNode.Children[0]
	hVal, ok := boxNode.Props["height"]
	if !ok {
		t.Fatalf("expected height prop on box node inside template, got props: %+v", boxNode.Props)
	}
	hNum, ok := hVal.(Num)
	if !ok {
		t.Fatalf("expected Num for height, got %T: %v", hVal, hVal)
	}
	if float64(hNum) <= 0 {
		t.Fatalf("expected positive height from $reveal.ch, got %v", hNum)
	}

	revealVal, ok := frame.Env.Get("reveal")
	if !ok {
		t.Fatal("expected $reveal in frame.Env")
	}
	chVal, err := frame.Env.Member(revealVal, "ch")
	if err != nil {
		t.Fatalf("eval Member ch on reveal failed: %v", err)
	}
	if chVal != hNum {
		t.Fatalf("template height %v does not match parent $reveal.ch %v", hNum, chVal)
	}
}
