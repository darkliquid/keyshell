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

func TestRowIntrinsicLeafAndNestedSizing(t *testing.T) {
	scene := mustScene(t, `Window {
		Row { height: 48, gap: 12, align: center
			Box { width: 12 }
			Pill { text: $mode, tone: accent }
			Row { gap: 2, align: center
				Text { text: $query, size: 20 }
				Cursor { width: 3, height: 24 }
			}
			Box { grow: 1 }
			Text { text: $results.len, size: 12 }
			Box { width: 12 }
		}
	}`)
	st := State{
		Width:   400,
		Height:  100,
		Scale:   1,
		Mode:    "SEARCH",
		Query:   "fi",
		Results: []Item{{ID: "1"}, {ID: "2"}, {ID: "3"}},
	}
	f, err := Resolve(scene, st, WithMeasurer(fakeMeasurer{}))
	if err != nil {
		t.Fatal(err)
	}
	row := f.Root.Children[0]
	pill := row.Children[1]
	// fakeMeasurer: "SEARCH" at 12px = 6*12 = 72 + 16 pad = 88, height = 28, centered in 48 -> Y = 10
	if pill.Rect.W != 88 || pill.Rect.H != 28 || pill.Rect.Y != 10 {
		t.Errorf("pill rect = %+v, want W=88 H=28 Y=10", pill.Rect)
	}
	queryGroup := row.Children[2]
	qText := queryGroup.Children[0]
	cursor := queryGroup.Children[1]
	// "fi" at 20px = 2*20 = 40; cursor should be at qText.X + 40 + 2
	if qText.Rect.W != 40 || qText.Rect.H != 20 || qText.Rect.Y != 14 {
		t.Errorf("query text rect = %+v, want W=40 H=20 Y=14", qText.Rect)
	}
	if cursor.Rect.X != qText.Rect.X+42 || cursor.Rect.Y != 12 {
		t.Errorf("cursor rect = %+v, want X=%v Y=12", cursor.Rect, qText.Rect.X+42)
	}
	countText := row.Children[4]
	// $results.len is Num(3) -> "3" at 12px = 12 wide, placed before right margin (12 gap + 12 box = 24 from right edge -> X = 400 - 24 - 12 = 364)
	if countText.Rect.W != 12 || countText.Rect.X != 364 || countText.Rect.Y != 18 {
		t.Errorf("count text rect = %+v, want X=364 Y=18 W=12", countText.Rect)
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

func TestNestedContainerIntrinsicSizingAndRelativeOffsets(t *testing.T) {
	// Nested Box inside Box with x and y offset
	scene := mustScene(t, `Window {
		Box { x: 50, y: 40, width: 200, height: 100
			Box { x: 10, y: 15, width: 80, height: 30 }
		}
	}`)
	st := &State{Width: 500, Height: 500, Scale: 1}
	f, err := Resolve(scene, *st)
	if err != nil {
		t.Fatal(err)
	}
	parent := f.Root.Children[0]
	child := parent.Children[0]
	if parent.Rect.X != 50 || parent.Rect.Y != 40 {
		t.Errorf("parent = %+v, want X=50 Y=40", parent.Rect)
	}
	if child.Rect.X != 60 || child.Rect.Y != 55 {
		t.Errorf("child = %+v, want X=60 Y=55 (relative to parent)", child.Rect)
	}

	// Column inside Row: Column takes max width of children and sum of heights
	sceneColInRow := mustScene(t, `Window {
		Row { height: 60, align: center
			Column { gap: 4
				Text { text: "Title", size: 14 }
				Text { text: "Longer Subtitle", size: 10 }
			}
		}
	}`)
	fCol, err := Resolve(sceneColInRow, *st, WithMeasurer(fakeMeasurer{}))
	if err != nil {
		t.Fatal(err)
	}
	row := fCol.Root.Children[0]
	col := row.Children[0]
	// fakeMeasurer: "Title" at 14px = 5*14 = 70. "Longer Subtitle" at 10px = 15*10 = 150.
	// Max width = 150.
	if col.Rect.W != 150 {
		t.Errorf("column width = %v, want 150", col.Rect.W)
	}
	// Heights: 14 + 10 + 4 gap = 28. Centered in 60 -> (60 - 28)/2 = 16.
	if col.Rect.H != 28 || col.Rect.Y != 16 {
		t.Errorf("column height = %v y = %v, want H=28 Y=16", col.Rect.H, col.Rect.Y)
	}
}

type fakeMeasurer struct{}

func (fakeMeasurer) TextSize(s string, size float64, _ string) (float64, float64) {
	return float64(len(s)) * size, size
}
