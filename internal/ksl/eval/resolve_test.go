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
