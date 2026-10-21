package eval

import "testing"

func TestBindings(t *testing.T) {
	st := State{
		Open: true, Query: "fire", Mode: "apps",
		Results: []Item{{ID: "a", Title: "Firefox"}, {ID: "b", Title: "Firejail"}},
		Tokens:  map[string]Color{"accent": {0xFF, 0x2E, 0x63, 0xFF}},
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
		"angle": Num(0.05), "shear": Num(-0.3), "cardShear": Num(0.3), "d": Str("M 0 0 L 1 1"),
	} {
		got, err := env.member(rev, name)
		if err != nil || got != want {
			t.Errorf("$reveal.%s = %v, %v; want %v", name, got, err, want)
		}
	}
}

func TestItemScope(t *testing.T) {
	st := State{Results: []Item{{ID: "a", Title: "Firefox"}}}
	env := NewEnv(&st, nil)
	inner := env.ItemScope(&st.Results[0])
	if got, ok := inner.lookupRef("item"); !ok || got != (ItemVal{&st.Results[0]}) {
		t.Errorf("$item = %v, %v", got, ok)
	}
	// parent bindings stay visible through the item scope
	if got, ok := inner.lookupRef("open"); !ok || got != Bool(false) {
		t.Errorf("$open through scope = %v, %v", got, ok)
	}
}
