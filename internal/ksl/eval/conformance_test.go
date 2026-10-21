package eval

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl"
)

func specTokens() map[string]Color {
	return map[string]Color{
		"slab":        {0x0B, 0x0B, 0x0F, 0xFF},
		"ink":         {0xF2, 0xF2, 0xF7, 0xFF},
		"accent":      {0xFF, 0x2E, 0x63, 0xFF},
		"accent-alt":  {0x08, 0xF7, 0xFE, 0xFF},
		"muted":       {0x5A, 0x5A, 0x66, 0xFF},
		"surface-alt": {0x12, 0x12, 0x18, 0xFF},
	}
}

func TestSpecScenesResolve(t *testing.T) {
	for _, name := range []string{"minimal.ksl", "reference.ksl", "dropdown.ksl"} {
		src, err := os.ReadFile(filepath.Join("..", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		scene, err := ksl.Parse(name, src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := ksl.Validate(scene); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		st := State{
			Width: 1280, Height: 400, Scale: 1, Summon: 1, Seed: 1,
			RandCache: &RandCache{},
			Tokens:    specTokens(),
			Reveal:    Reveal{Y: 100, H: 64, Angle: 0.05, Shear: -0.25},
		}
		f, err := Resolve(scene, st, WithMeasurer(fakeMeasurer{}))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if f.Root == nil || f.Root.Rect.W == 0 {
			t.Errorf("%s: empty frame", name)
		}
	}
}

func TestReferenceSceneResolvesWithItems(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "testdata", "reference.ksl"))
	if err != nil {
		t.Fatal(err)
	}
	scene, err := ksl.Parse("reference.ksl", src)
	if err != nil {
		t.Fatal(err)
	}
	st := State{
		Width: 1280, Height: 400, Scale: 1, Summon: 1, Seed: 1,
		RandCache: &RandCache{}, Tokens: specTokens(),
		Reveal:   Reveal{Y: 100, H: 64, Angle: 0.05, Shear: -0.25, D: "M 0 0 L 1 1"},
		Mode:     "SEARCH",
		Selected: 0,
		Results:  []Item{{ID: "a", Title: "Firefox"}, {ID: "b", Title: "Terminal"}},
	}
	f, err := Resolve(scene, st, WithMeasurer(fakeMeasurer{}))
	if err != nil {
		t.Fatal(err)
	}
	band := f.Root.Children[0]
	if band.Name != "Group" || band.ID != "band" {
		t.Fatalf("band = %+v", band)
	}
	rot, ok := band.Props["rotate"].(Length)
	if !ok || math.Abs(rot.V-2.8648) > 0.001 || rot.Unit != "deg" {
		t.Errorf("band rotate = %v", band.Props["rotate"])
	}
	// the List inside the Stack inside the Column must materialise items
	var list *ResolvedNode
	var walk func(n *ResolvedNode)
	walk = func(n *ResolvedNode) {
		if n.Name == "List" {
			list = n
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(f.Root)
	if list == nil {
		t.Fatal("no List in the frame")
	}
	if len(list.Children) != 2 {
		t.Errorf("list items = %d, want 2", len(list.Children))
	}
}
