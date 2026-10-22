package eval

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/ksl"
)

func TestLiteralConversion(t *testing.T) {
	tests := []struct {
		src  string
		want Value
	}{
		{"24", Num(24)},
		{"12px", Length{V: 12, Unit: "px"}},
		{"40%", Length{V: 40, Unit: "%"}},
		{"250ms", Length{V: 250, Unit: "ms"}},
		{`"hi"`, Str("hi")},
		{"true", Bool(true)},
		{"accent", Kw("accent")},
		{"#FF2E63", Color{0xFF, 0x2E, 0x63, 0xFF}},
		{"#f2e", Color{0xFF, 0x22, 0xEE, 0xFF}},
		{"#f2e8", Color{0xFF, 0x22, 0xEE, 0x88}},
		{"#FF2E63CC", Color{0xFF, 0x2E, 0x63, 0xCC}},
	}
	for _, tt := range tests {
		s, err := ksl.Parse("t.ksl", []byte("Box { x: "+tt.src+" }"))
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.src, err)
		}
		got, err := literal(s.Nodes()[0].Props[0].Val)
		if err != nil {
			t.Fatalf("literal(%q): %v", tt.src, err)
		}
		if got != tt.want {
			t.Errorf("literal(%q) = %#v, want %#v", tt.src, got, tt.want)
		}
	}
}

func TestLiteralArraysAndBlocks(t *testing.T) {
	s, err := ksl.Parse("t.ksl", []byte(`Box { assets: ["a.png", "b.png"], viewport: { height: 320px } }`))
	if err != nil {
		t.Fatal(err)
	}
	arr, err := literal(s.Nodes()[0].Props[0].Val)
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := arr.(Array); !ok || len(a) != 2 || a[0] != Str("a.png") {
		t.Errorf("array = %#v", arr)
	}
	ib, err := literal(s.Nodes()[0].Props[1].Val)
	if err != nil {
		t.Fatal(err)
	}
	if b, ok := ib.(Props); !ok || len(b) != 1 || b[0].Key != "height" {
		t.Errorf("inline block = %#v", ib)
	}
}

func TestLiteralHexErrors(t *testing.T) {
	if _, err := hexColor("12"); err == nil {
		t.Errorf("hexColor(2 digits) = nil error, want failure")
	}
	if _, err := hexColor("zz"); err == nil {
		t.Errorf("hexColor(non-hex) = nil error, want failure")
	}
}
