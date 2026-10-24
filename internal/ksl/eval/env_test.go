package eval

import "testing"

func TestBindings(t *testing.T) {
	st := State{
		Open: true, Query: "fire", Mode: "apps",
		Results: []Item{{ID: "a", Title: "Firefox"}, {ID: "b", Title: "Firejail"}},
		Tokens:  map[string]Color{"accent": {0xFF, 0x2E, 0x63, 0xFF}},
		Width:   1920, Height: 1080,
	}
	env := NewEnv(&st, nil)
	for _, tc := range []struct {
		ref  string
		want Value
	}{
		{"open", Bool(true)},
		{"focused", Bool(false)},
		{"query", Str("fire")},
		{"mode", Str("apps")},
		{"selected", Num(0)},
		{"typing", Num(0)},
		{"accent", Color{0xFF, 0x2E, 0x63, 0xFF}},
		{"active", ItemVal{&st.Results[0]}},
		{"width", Num(1920)},
		{"height", Num(1080)},
	} {
		got, ok := env.lookupRef(tc.ref)
		if !ok || got != tc.want {
			t.Errorf("lookupRef(%q) = %#v, %v; want %#v", tc.ref, got, ok, tc.want)
		}
	}
	if _, ok := env.lookupRef("nope"); ok {
		t.Errorf("unknown binding resolved")
	}
}

func TestMembersAndMethods(t *testing.T) {
	st := State{Results: []Item{{Title: "Firefox", StatusTone: "warn"}}}
	env := NewEnv(&st, nil)
	if got, err := env.member(env.results(), "len"); err != nil || got != Num(1) {
		t.Errorf("$results.len = %v, %v", got, err)
	}
	item := ItemVal{&st.Results[0]}
	if got, err := env.member(item, "title"); err != nil || got != Str("Firefox") {
		t.Errorf("$item.title = %v, %v", got, err)
	}
	if got, err := env.member(item, "statusTone"); err != nil || got != Str("warn") {
		t.Errorf("$item.statusTone = %v, %v", got, err)
	}
	if got, err := env.call(Str("enter"), "toUpper", nil); err != nil || got != Str("ENTER") {
		t.Errorf("toUpper = %v, %v", got, err)
	}
	if got, err := env.call(Str("ENTER"), "toLower", nil); err != nil || got != Str("enter") {
		t.Errorf("toLower = %v, %v", got, err)
	}
	if _, err := env.member(Str("x"), "bogus"); err == nil {
		t.Errorf("unknown member = nil error, want failure")
	}
	if _, err := env.call(Num(1), "toUpper", nil); err == nil {
		t.Errorf("toUpper on a number = nil error, want failure")
	}
}

func TestRevealMembers(t *testing.T) {
	st := &State{Reveal: Reveal{Y: 100, H: 64, HLeft: 60, HRight: 70, Angle: 0.05, Shear: -0.3, CardShear: 0.3, D: "M 0 0 L 1 1"}}
	env := NewEnv(st, nil)
	rev, _ := env.lookupRef("reveal")
	for name, want := range map[string]Value{
		"y": Num(100), "h": Num(64), "hLeft": Num(60), "hRight": Num(70),
		"angle": Length{V: 0.05 * 180 / 3.141592653589793, Unit: "deg"}, "shear": Num(-0.3), "cardShear": Num(0.3), "d": Str("M 0 0 L 1 1"),
	} {
		got, err := env.member(rev, name)
		if err != nil || got != want {
			t.Errorf("$reveal.%s = %v, %v; want %v", name, got, err, want)
		}
	}
}

func TestPanelBindings(t *testing.T) {
	st := &State{Panel: Panel{Open: true, Focus: 1, Dirty: false, Rows: []Item{
		{Kind: "action", Title: "RUN"},
		{Kind: "range", Title: "Volume", Ratio: 0.5},
	}}}
	env := NewEnv(st, nil)
	p, ok := env.lookupRef("panel")
	if !ok {
		t.Fatal("$panel missing")
	}
	if got, err := env.member(p, "open"); err != nil || got != Bool(true) {
		t.Errorf("$panel.open = %v, %v; want true", got, err)
	}
	if got, err := env.member(p, "focus"); err != nil || got != Num(1) {
		t.Errorf("$panel.focus = %v, %v; want 1", got, err)
	}
	if got, err := env.member(p, "dirty"); err != nil || got != Bool(false) {
		t.Errorf("$panel.dirty = %v, %v; want false", got, err)
	}
	rows, err := env.member(p, "rows")
	if err != nil {
		t.Fatalf("$panel.rows = err %v", err)
	}
	if got, err := env.member(rows, "len"); err != nil || got != Num(2) {
		t.Errorf("$panel.rows.len = %v, %v; want 2", got, err)
	}
}

func TestOrderedControls(t *testing.T) {
	st := &State{Results: []Item{{ID: "a", Controls: []ItemControl{
		{ID: "volume", Kind: "range", Label: "Volume", Ratio: 0.5},
		{ID: "mute", Kind: "toggle", Label: "Mute", On: true},
	}}}}
	env := NewEnv(st, nil)
	inner := env.ItemScope(&st.Results[0])
	item := ItemVal{&st.Results[0]}
	if got, err := env.member(item, "volume"); err != nil {
		t.Errorf("$item.volume = %v, %v", got, err)
	} else if cv, ok := got.(ControlVal); !ok || cv.Ratio != 0.5 || cv.Kind != "range" {
		t.Errorf("$item.volume = %#v", got)
	}
	if got, err := env.member(item, "mute"); err != nil {
		t.Errorf("$item.mute = %v, %v", got, err)
	} else if cv, ok := got.(ControlVal); !ok || !cv.On {
		t.Errorf("$item.mute = %#v", got)
	}
	if _, err := env.member(item, "nope"); err == nil {
		t.Error("$item.nope = nil error, want failure")
	}
	_ = inner
}

func TestItemScope(t *testing.T) {
	st := State{Results: []Item{{ID: "a", Title: "Firefox"}}}
	env := NewEnv(&st, nil)
	env.Set("parentVar", "fromParent")
	inner := env.ItemScope(&st.Results[0])
	if got, ok := inner.lookupRef("item"); !ok || got != (ItemVal{&st.Results[0]}) {
		t.Errorf("$item = %v, %v", got, ok)
	}
	// parent bindings stay visible through the item scope
	if got, ok := inner.lookupRef("open"); !ok || got != Bool(false) {
		t.Errorf("$open through scope = %v, %v", got, ok)
	}
	if got, ok := inner.Get("parentVar"); !ok || got != Str("fromParent") {
		t.Errorf("parentVar through scope = %v, %v", got, ok)
	}
	// Setting variable in inner scope must not mutate parent
	inner.Set("innerVar", "local")
	if _, ok := env.Get("innerVar"); ok {
		t.Error("innerVar leaked into parent scope")
	}
	if got, ok := inner.Get("innerVar"); !ok || got != Str("local") {
		t.Errorf("innerVar in inner scope = %v, %v", got, ok)
	}
}
