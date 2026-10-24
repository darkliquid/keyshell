package ksl

import "testing"

func TestParseTheme(t *testing.T) {
	s := mustParse(t, `theme {
	    ink: #EAEAEA
	    font { display: "RedHatDisplay", ui: "Inter" }
	    assets: ["bg.png", "texture.jpg"]
	}`)
	th := s.ThemeBlocks()[0]
	if len(th.Tokens) != 1 || th.Tokens[0].Key != "ink" {
		t.Errorf("tokens = %+v, want one ink token", th.Tokens)
	}
	if len(th.Fonts) != 2 || th.Fonts[0].Key != "display" {
		t.Errorf("fonts = %+v, want display and ui", th.Fonts)
	}
	if len(th.Assets) != 2 {
		t.Errorf("assets = %v, want 2 entries", th.Assets)
	}
}

func TestParseStatesBothForms(t *testing.T) {
	s := mustParse(t, `Window {
	    Box#card { fill: $slab
	        state selected { fill: $accent }
	    }
	}
	state open {
	    Box#card { opacity: 1 }
	}`)
	root := s.Nodes()[0]
	box := root.Children[0]
	if len(box.States) != 1 || box.States[0].Name != "selected" || len(box.States[0].Props) != 1 {
		t.Errorf("node state = %+v, want selected with one prop", box.States)
	}
	scene := s.States()[0]
	if scene.Name != "open" || len(scene.Targets) != 1 || scene.Targets[0].Sel.ID != "card" {
		t.Errorf("scene state = %+v, want open targeting #card", scene)
	}
	if len(scene.Props) != 0 {
		t.Errorf("scene state must not hold props directly, got %+v", scene.Props)
	}
}

func TestParseTransitionAndTiming(t *testing.T) {
	s := mustParse(t, `transition open { rotate: 250ms ease-out, opacity: 180ms, x: 300ms cubic-bezier(0.25, 0.1, 0.25, 1) }`)
	tr := s.Transitions()[0]
	if tr.State != "open" || len(tr.Props) != 3 {
		t.Fatalf("transition = %+v", tr)
	}
	if tr.Props[0].Dur.String() != "250ms" || tr.Props[0].Easing.String() != "ease-out" {
		t.Errorf("rotate timing = %+v", tr.Props[0])
	}
	if tr.Props[1].Easing != nil {
		t.Errorf("opacity easing = %v, want nil", tr.Props[1].Easing)
	}
	if tr.Props[2].Easing.String() != "cubic-bezier(0.25, 0.1, 0.25, 1)" {
		t.Errorf("x easing = %v", tr.Props[2].Easing.String())
	}
}

func TestParseKeyframes(t *testing.T) {
	s := mustParse(t, `@keyframes blink {
	    0% { opacity: 1 }
	    50% { opacity: 0 }
	    100% { opacity: 1 }
	}`)
	kf := s.Keyframes()[0]
	if kf.Name != "blink" || len(kf.Frames) != 3 || kf.Frames[0].Percent != 0 || kf.Frames[2].Percent != 100 {
		t.Errorf("keyframes = %+v", kf)
	}
}

func TestParseDefAndAnimation(t *testing.T) {
	s := mustParse(t, `
@keyframes breathe { 0% { opacity: 0.4 } 100% { opacity: 1 } }
def Card { Box { fill: $slab } }
Window {
    Card#x { }
    Meter#progress { animation: breathe 2s ease-in-out infinite alternate }
}
`)
	d := s.Defs()[0]
	if d.Name != "Card" || len(d.Children) != 1 {
		t.Errorf("def = %+v", d)
	}
	w := s.Nodes()[0]
	if w.Name != "Window" {
		t.Fatalf("root = %+v, want Window", w)
	}
	m := w.Children[1]
	av, ok := m.Props[0].Val.(*AnimVal)
	if !ok || len(av.Steps) != 1 {
		t.Fatalf("animation = %#v, want AnimVal", m.Props[0].Val)
	}
	st := av.Steps[0]
	if st.Name != "breathe" || st.Dur == nil || st.Dur.String() != "2s" ||
		st.Easing.String() != "ease-in-out" || st.Iteration.String() != "infinite" ||
		st.Direction != "alternate" {
		t.Errorf("anim step = %+v", st)
	}
}

func TestParseThemeVariousTypes(t *testing.T) {
	src := `theme {
		accent: #00FFC2
		band_height: 320
		min_tilt: 6.5
		title: "Keyshell"
		enabled: true
	}`
	s := mustParse(t, src)
	th := s.ThemeBlocks()[0]
	if len(th.Tokens) != 5 {
		t.Fatalf("tokens len = %d, want 5", len(th.Tokens))
	}
	expected := []string{"accent", "band_height", "min_tilt", "title", "enabled"}
	for i, exp := range expected {
		if th.Tokens[i].Key != exp {
			t.Errorf("token %d key = %q, want %q", i, th.Tokens[i].Key, exp)
		}
	}
}
