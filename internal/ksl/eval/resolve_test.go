package eval

import "testing"

func TestListMaterialisation(t *testing.T) {
	scene := mustScene(t, `
		def Row { Box { Text { text: $item.title } } }
		Window { List {
		    items: $results, selected: $selected
		    cap: 2
		    template { Row { } }
		    empty { Text { text: "none" } }
		}}`)
	st := State{Width: 100, Height: 100, Scale: 1,
		Results:  []Item{{ID: "a", Title: "Firefox"}, {ID: "b", Title: "Firejail"}, {ID: "c", Title: "Fira"}},
		Selected: 1}
	f, err := Resolve(scene, st)
	if err != nil {
		t.Fatal(err)
	}
	list := f.Root.Children[0]
	if len(list.Children) != 2 { // cap=2, not 3
		t.Fatalf("list children = %d, want 2", len(list.Children))
	}
	sel := list.Children[1]
	// selected item's template sees its own $item
	txt := sel.Children[0].Children[0]
	if txt.Props["text"] != Str("Firejail") {
		t.Errorf("selected template text = %v", txt.Props["text"])
	}
}

func TestListEmpty(t *testing.T) {
	scene := mustScene(t, `Window { List { items: $results, empty { Text { text: "none" } } } }`)
	f, err := Resolve(scene, State{Width: 100, Height: 100, Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	list := f.Root.Children[0]
	if len(list.Children) != 1 || list.Children[0].Props["text"] != Str("none") {
		t.Errorf("empty list = %+v", list.Children)
	}
}

func TestListPositionsItems(t *testing.T) {
	scene := mustScene(t, `Window { List {
	    items: $results, gap: 4
	    orientation: horizontal
	    template { Box { width: 20, height: 10 } }
	} }`)
	st := State{Width: 100, Height: 100, Scale: 1, Results: []Item{{ID: "a"}, {ID: "b"}}}
	f, err := Resolve(scene, st)
	if err != nil {
		t.Fatal(err)
	}
	list := f.Root.Children[0]
	if len(list.Children) != 2 {
		t.Fatalf("items = %d", len(list.Children))
	}
	if list.Children[0].Rect.X != 0 || list.Children[1].Rect.X != 24 {
		t.Errorf("item x = %v, %v; want 0, 24", list.Children[0].Rect.X, list.Children[1].Rect.X)
	}
}

func TestListGroupChildrenStayInLocalCoordsAndMarkSelected(t *testing.T) {
	scene := mustScene(t, `Window { List {
	    items: $results, selected: $selected, gap: 8
	    orientation: horizontal
	    template { Group#card { width: 100, height: 50, Card { } } }
	} }`)
	st := State{
		Width:    400,
		Height:   100,
		Scale:    1,
		Results:  []Item{{ID: "a"}, {ID: "b"}},
		Selected: 1,
	}
	f, err := Resolve(scene, st)
	if err != nil {
		t.Fatal(err)
	}
	list := f.Root.Children[0]
	g0, g1 := list.Children[0], list.Children[1]
	if g0.Rect.X != 0 || g1.Rect.X != 108 {
		t.Fatalf("group x = %v, %v; want 0, 108", g0.Rect.X, g1.Rect.X)
	}
	if g1.Children[0].Rect.X != 0 || g1.Children[0].Rect.Y != 0 {
		t.Errorf("card inside group was translated to %+v, want local (0,0)", g1.Children[0].Rect)
	}
	if g0.Selected || g0.Children[0].Selected {
		t.Errorf("unselected group/card marked selected")
	}
	if !g1.Selected || !g1.Children[0].Selected {
		t.Errorf("selected group/card not marked selected")
	}
}

func TestListCycleScrollCenterFillsViewportEdgeToEdge(t *testing.T) {
	scene := mustScene(t, `Window { List {
	    items: $results, selected: $selected, gap: 16
	    orientation: horizontal, scroll: center, overflow: cycle
	    template { Group#card { width: 360, height: 120, Card { } } }
	} }`)
	st := State{
		Width:    1600,
		Height:   200,
		Scale:    1,
		Results:  []Item{{ID: "a", Title: "Alpha"}, {ID: "b", Title: "Beta"}, {ID: "c", Title: "Gamma"}},
		Selected: 1,
	}
	f, err := Resolve(scene, st)
	if err != nil {
		t.Fatal(err)
	}
	list := f.Root.Children[0]
	if len(list.Children) < 5 {
		t.Fatalf("cycle list children = %d, want >= 5 to span 1600px viewport", len(list.Children))
	}
	first := list.Children[0]
	last := list.Children[len(list.Children)-1]
	if first.Rect.X > 0 {
		t.Errorf("first card X = %v, want <= 0 to cover left edge", first.Rect.X)
	}
	if last.Rect.X+last.Rect.W < 1600 {
		t.Errorf("last card right = %v, want >= 1600 to cover right edge", last.Rect.X+last.Rect.W)
	}
	selectedCount := 0
	for i, ch := range list.Children {
		if i > 0 {
			prev := list.Children[i-1]
			gap := ch.Rect.X - (prev.Rect.X + prev.Rect.W)
			if gap != 16 {
				t.Errorf("gap between slot %d and %d = %v, want 16", i-1, i, gap)
			}
		}
		if ch.Selected {
			selectedCount++
			center := ch.Rect.X + ch.Rect.W/2
			if center != 800 {
				t.Errorf("selected card center = %v, want 800", center)
			}
			if ch.Item == nil || ch.Item.ID != "b" {
				t.Errorf("selected card item = %+v, want b", ch.Item)
			}
		}
	}
	if selectedCount != 1 {
		t.Errorf("selectedCount = %d, want 1", selectedCount)
	}
}

func TestListScrollRevealVertical(t *testing.T) {
	scene := mustScene(t, `Window { List {
	    x: 0, y: 10, width: 200, height: 100
	    items: $results, selected: $selected, gap: 0
	    orientation: vertical, scroll: reveal, overflow: clamp
	    template { Box { width: 200, height: 20 } }
	} }`)
	// 10 items, 20px each = 200px total content in a 100px high viewport at y=10.
	items := make([]Item, 10)
	for i := range items {
		items[i] = Item{ID: string(rune('a' + i))}
	}

	// Case 1: selected = 0 (top). First item should be at y=10.
	st := State{Width: 200, Height: 200, Scale: 1, Results: items, Selected: 0}
	f, err := Resolve(scene, st)
	if err != nil {
		t.Fatal(err)
	}
	list := f.Root.Children[0]
	if len(list.Children) != 10 {
		t.Fatalf("children len = %d, want 10", len(list.Children))
	}
	if list.Children[0].Rect.Y != 10 {
		t.Errorf("item 0 Y = %v, want 10", list.Children[0].Rect.Y)
	}

	// Case 2: selected = 5 (item at [100, 120] relative to list Y=10, so unshifted is [110, 130]).
	// Viewport is [10, 110]. To reveal item 5, its bottom must align with viewport bottom (110).
	st.Selected = 5
	f, err = Resolve(scene, st)
	if err != nil {
		t.Fatal(err)
	}
	list = f.Root.Children[0]
	selNode := list.Children[5]
	if selNode.Rect.Y < 10 || selNode.Rect.Y+selNode.Rect.H > 110 {
		t.Errorf("item 5 Rect = %+v, not within viewport [10, 110]", selNode.Rect)
	}
	if selNode.Rect.Y+selNode.Rect.H != 110 {
		t.Errorf("item 5 bottom = %v, want 110", selNode.Rect.Y+selNode.Rect.H)
	}
}

func TestRandInSceneIsSummonScoped(t *testing.T) {
	scene := mustScene(t, `Window { Group#band { rotate: rand(-4, 4) * 1deg } }`)
	st := State{Width: 100, Height: 100, Scale: 1, Summon: 1, Seed: 42, RandCache: &RandCache{}}
	f1, err := Resolve(scene, st)
	if err != nil {
		t.Fatal(err)
	}
	f2, _ := Resolve(scene, st)
	st.Summon = 2
	f3, _ := Resolve(scene, st)
	r1 := f1.Root.Children[0].Props["rotate"]
	r2 := f2.Root.Children[0].Props["rotate"]
	r3 := f3.Root.Children[0].Props["rotate"]
	if r1 != r2 {
		t.Errorf("rand within summon: %v vs %v", r1, r2)
	}
	if r1 == r3 {
		t.Errorf("rand across summons: %v vs %v", r1, r3)
	}
}
