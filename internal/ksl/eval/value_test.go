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
	if _, err := hexColor("#12"); err == nil {
		t.Errorf("hexColor(# 2 digits) = nil error, want failure")
	}
	if _, err := hexColor("#zz"); err == nil {
		t.Errorf("hexColor(# non-hex) = nil error, want failure")
	}
}

func TestToColorAndToValue(t *testing.T) {
	colorTests := []struct {
		input string
		want  Color
	}{
		{"#FF2E63", Color{0xFF, 0x2E, 0x63, 0xFF}},
		{"#00FFC2", Color{0x00, 0xFF, 0xC2, 0xFF}},
		{"#f2e", Color{0xFF, 0x22, 0xEE, 0xFF}},
		{"#f2e8", Color{0xFF, 0x22, 0xEE, 0x88}},
		{"#FF2E63CC", Color{0xFF, 0x2E, 0x63, 0xCC}},
		{"#cafe", Color{0xCC, 0xAA, 0xFF, 0xEE}},
		{"#feed", Color{0xFF, 0xEE, 0xEE, 0xDD}},
		{"#123456", Color{0x12, 0x34, 0x56, 0xFF}},
	}
	for _, tt := range colorTests {
		c, ok := toColor(tt.input)
		if !ok || c != tt.want {
			t.Errorf("toColor(%q) = (%v, %v), want (%v, true)", tt.input, c, ok, tt.want)
		}
		cStr, ok := toColor(Str(tt.input))
		if !ok || cStr != tt.want {
			t.Errorf("toColor(Str(%q)) = (%v, %v), want (%v, true)", tt.input, cStr, ok, tt.want)
		}
		val, ok := toValue(tt.input)
		if !ok || val != tt.want {
			t.Errorf("toValue(%q) = (%v, %v), want (%v, true)", tt.input, val, ok, tt.want)
		}
	}

	// Plain strings without '#' must evaluate to Str in toValue and fail toColor,
	// even if their content happens to consist of hex digits (e.g. "cafe", "feed", "face", "1234").
	plainStrings := []string{
		"cafe", "feed", "face", "1234", "FF2E63", "00FFC2", "f2e", "custom", "hello", "12", "zz",
	}
	for _, ps := range plainStrings {
		if c, ok := toColor(ps); ok {
			t.Errorf("toColor(%q) = (%v, %v), want false", ps, c, ok)
		}
		if c, ok := toColor(Str(ps)); ok {
			t.Errorf("toColor(Str(%q)) = (%v, %v), want false", ps, c, ok)
		}
		val, ok := toValue(ps)
		if !ok || val != Str(ps) {
			t.Errorf("toValue(%q) = (%v, %v), want (Str(%q), true)", ps, val, ok, ps)
		}
		// Existing Str values remain Str
		valStr, ok := toValue(Str(ps))
		if !ok || valStr != Str(ps) {
			t.Errorf("toValue(Str(%q)) = (%v, %v), want (Str(%q), true)", ps, valStr, ok, ps)
		}
	}
}

func TestToValuePrimitives(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  Value
	}{
		{"int", int(42), Num(42)},
		{"int64", int64(100), Num(100)},
		{"float64", float64(3.14), Num(3.14)},
		{"bool true", true, Bool(true)},
		{"bool false", false, Bool(false)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := toValue(tt.input)
			if !ok || val != tt.want {
				t.Errorf("toValue(%v) = (%v, %v), want (%v, true)", tt.input, val, ok, tt.want)
			}
		})
	}
}
