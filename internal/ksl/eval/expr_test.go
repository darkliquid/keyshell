package eval

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl"
)

func evalOne(t *testing.T, st *State, src string) (Value, error) {
	t.Helper()
	s, err := ksl.Parse("t.ksl", []byte("Box { x: "+src+" }"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return NewEnv(st, nil).Eval(s.Nodes()[0].Props[0].Val)
}

func TestEvalExpr(t *testing.T) {
	st := &State{Query: "fire", Selected: 1,
		Results: []Item{{Title: "Firefox"}, {Title: "Firejail"}},
		Tokens:  map[string]Color{"accent": {0xFF, 0x2E, 0x63, 0xFF}},
		Reveal:  Reveal{H: 64, Angle: 2}}
	tests := []struct {
		src  string
		want Value
	}{
		{"$results.len * 12", Num(24)},
		{`$query + "!"`, Str("fire!")},
		{`$query + 1`, Str("fire1")}, // Rule 19: either operand a string = concat
		{"$selected == 1", Bool(true)},
		{"$results.len > 1", Bool(true)},
		{"$open ? 1 : 2", Num(2)},
		{"$reveal.h / 2", Num(32)},
		{"$accent", Color{0xFF, 0x2E, 0x63, 0xFF}},
		{"-4 * 1deg", Length{V: -4, Unit: "deg"}},
		{"2 + 3 * 4", Num(14)},
		{"(2 + 3) * 4", Num(20)},
	}
	for _, tt := range tests {
		got, err := evalOne(t, st, tt.src)
		if err != nil {
			t.Fatalf("Eval(%q): %v", tt.src, err)
		}
		if got != tt.want {
			t.Errorf("Eval(%q) = %#v, want %#v", tt.src, got, tt.want)
		}
	}
}

func TestEvalRefPath(t *testing.T) {
	s, err := ksl.Parse("t.ksl", []byte(`Box { x: #band.width }`))
	if err != nil {
		t.Fatal(err)
	}
	env := NewEnv(&State{}, nil)
	env.RefPaths["band.width"] = Num(640)
	got, err := env.Eval(s.Nodes()[0].Props[0].Val)
	if err != nil || got != Num(640) {
		t.Errorf("#band.width = %v, %v", got, err)
	}
}

func TestEvalFuncVal(t *testing.T) {
	got, err := evalOne(t, &State{Tokens: map[string]Color{"accent": {0xFF, 0, 0, 0xFF}}},
		"linear-gradient($accent, #000)")
	if err != nil {
		t.Fatal(err)
	}
	fv, ok := got.(FuncVal)
	if !ok || fv.Name != "linear-gradient" || len(fv.Args) != 2 {
		t.Fatalf("value = %#v, want linear-gradient FuncVal", got)
	}
	if fv.Args[0] != (Color{0xFF, 0, 0, 0xFF}) {
		t.Errorf("first stop = %v", fv.Args[0])
	}
}

func TestItemMethodThroughScope(t *testing.T) {
	s, err := ksl.Parse("t.ksl", []byte(`Text { text: $item.title.toUpper() }`))
	if err != nil {
		t.Fatal(err)
	}
	env := NewEnv(&State{}, nil).ItemScope(&Item{Title: "hi"})
	got, err := env.Eval(s.Nodes()[0].Props[0].Val)
	if err != nil || got != Str("HI") {
		t.Errorf("$item.title.toUpper() = %v, %v", got, err)
	}
}

func TestEvalErrors(t *testing.T) {
	st := &State{}
	for _, src := range []string{
		`$open + 1`,    // bool arithmetic
		`$nope`,        // unknown binding
		`$results.foo`, // unknown member
		`$query.nope()`,
		`!"x"`, // ! on a string
		`1 / 0`,
		`$results.len * "x"`,
		`1px + 1deg`, // mismatched units
	} {
		if _, err := evalOne(t, st, src); err == nil {
			t.Errorf("Eval(%q) = nil error, want failure", src)
		}
	}
}
