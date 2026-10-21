package eval

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl"
)

func mustScene(t *testing.T, src string) *ksl.Scene {
	t.Helper()
	s, err := ksl.Parse("t.ksl", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}

func TestLengthResolution(t *testing.T) {
	st := &State{Width: 800, Height: 600, Scale: 1.5}
	if got := resolveLength(Length{V: 40, Unit: "%"}, st, true); got != 320 {
		t.Errorf("40%% of width = %v, want 320", got)
	}
	if got := resolveLength(Length{V: 40, Unit: "%"}, st, false); got != 240 {
		t.Errorf("40%% of height = %v, want 240", got)
	}
	if got := resolveLength(Length{V: 12, Unit: "px"}, st, false); got != 18 {
		t.Errorf("12px scaled = %v, want 18", got)
	}
	if got := resolveLength(Num(8), st, false); got != 12 {
		t.Errorf("unitless length = %v, want scaled px 12", got)
	}
}

func TestColumnLayout(t *testing.T) {
	scene := mustScene(t, `Window { Column { gap: 8, align: center
	    Box { height: 40 }
	    Box { grow: 1 }
	}}`)
	st := &State{Width: 100, Height: 100, Scale: 1}
	f, err := Resolve(scene, *st, WithMeasurer(nil))
	if err != nil {
		t.Fatal(err)
	}
	col := f.Root.Children[0]
	if col.Rect.H != 100 {
		t.Errorf("column height = %v, want fill", col.Rect.H)
	}
	a, b := col.Children[0], col.Children[1]
	if a.Rect.H != 40 || a.Rect.Y != 0 {
		t.Errorf("fixed child = %+v", a.Rect)
	}
	if b.Rect.H != 52 || b.Rect.Y != 48 { // 100 - 40 - 8 gap
		t.Errorf("grow child = %+v, want y=48 h=52", b.Rect)
	}
}

func TestRowGrowAndJustify(t *testing.T) {
	scene := mustScene(t, `Window { Row { gap: 4, justify: center
	    Box { width: 20 }, Box { width: 20 }
	}}`)
	st := &State{Width: 100, Height: 100, Scale: 1}
	f, err := Resolve(scene, *st)
	if err != nil {
		t.Fatal(err)
	}
	row := f.Root.Children[0]
	a, b := row.Children[0], row.Children[1]
	if a.Rect.X != 28 || b.Rect.X != 52 { // content 44 wide, centred
		t.Errorf("justified row: %v, %v", a.Rect.X, b.Rect.X)
	}
}

func TestStackKeepsDeclarationOrderAndZ(t *testing.T) {
	scene := mustScene(t, `Window { Stack {
	    Box { width: 10, z: 2 }, Box { width: 10, z: 1 }
	} }`)
	st := &State{Width: 100, Height: 100, Scale: 1}
	f, _ := Resolve(scene, *st)
	kids := f.Root.Children[0].Children
	if len(kids) != 2 || kids[0].Z != 2 || kids[1].Z != 1 {
		t.Errorf("stack children = %+v", kids)
	}
}

func TestConstraints(t *testing.T) {
	scene := mustScene(t, `Window { Box { width: 10, minW: 30, maxH: 5, height: 50 } }`)
	st := &State{Width: 100, Height: 100, Scale: 1}
	f, _ := Resolve(scene, *st)
	box := f.Root.Children[0]
	if box.Rect.W != 30 || box.Rect.H != 5 {
		t.Errorf("constrained rect = %+v", box.Rect)
	}
}

func TestTextMeasurement(t *testing.T) {
	scene := mustScene(t, `Window { Text { text: "hi", size: 12 } }`)
	st := &State{Width: 100, Height: 100, Scale: 1}
	f, _ := Resolve(scene, *st, WithMeasurer(fakeMeasurer{}))
	text := f.Root.Children[0]
	if text.Rect.W != 24 || text.Rect.H != 12 {
		t.Errorf("text rect = %+v", text.Rect)
	}
}

func TestGridLayout(t *testing.T) {
	scene := mustScene(t, `Window { Grid { cols: 2, gap: 10
	    Box { }, Box { }, Box { }
	} }`)
	st := &State{Width: 110, Height: 100, Scale: 1}
	f, err := Resolve(scene, *st)
	if err != nil {
		t.Fatal(err)
	}
	grid := f.Root.Children[0]
	// cell width = (110-10)/2 = 50; rows = 2; cell height = (100-10)/2 = 45
	if grid.Children[0].Rect.W != 50 || grid.Children[1].Rect.X != 60 {
		t.Errorf("cells = %+v %+v", grid.Children[0].Rect, grid.Children[1].Rect)
	}
	if grid.Children[2].Rect.Y != 55 {
		t.Errorf("second row y = %v, want 55", grid.Children[2].Rect.Y)
	}
}

type fakeMeasurer struct{}

func (fakeMeasurer) TextSize(s string, size float64, _ string) (float64, float64) {
	return float64(len(s)) * size, size
}
