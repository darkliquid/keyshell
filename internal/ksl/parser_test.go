package ksl

import (
	"errors"
	"strings"
	"testing"
)

func mustParse(t *testing.T, src string) *Scene {
	t.Helper()
	s, err := Parse("test.ksl", []byte(src))
	if err != nil {
		t.Fatalf("Parse() = %v, want nil", err)
	}
	return s
}

func TestParseNodeTree(t *testing.T) {
	s := mustParse(t, `Window {
	    Box#card { fill: $slab }
	    Column { gap: 24, align: center
	      Text { text: "hi" }
	    }
	}`)
	if len(s.Items) != 1 {
		t.Fatalf("top-level items = %d, want 1", len(s.Items))
	}
	root, ok := s.Items[0].(*Node)
	if !ok || root.Name != "Window" {
		t.Fatalf("root = %+v, want Window node", s.Items[0])
	}
	if len(root.Children) != 2 {
		t.Fatalf("root children = %d, want 2", len(root.Children))
	}
	box := root.Children[0]
	if box.ID != "card" || len(box.Props) != 1 || box.Props[0].Key != "fill" {
		t.Errorf("first child = %+v, want Box#card with fill", box)
	}
	col := root.Children[1]
	got := col.Props[0]
	sc, ok := got.Val.(*Scalar)
	if got.Key != "gap" || !ok || sc.N != 24 || sc.Unit != "" {
		t.Errorf("Column.gap = %+v, want scalar 24", got)
	}
	if len(col.Children) != 1 || col.Children[0].Name != "Text" {
		t.Errorf("Column children = %+v, want one Text", col.Children)
	}
}

func TestParseTopLevelStyleRule(t *testing.T) {
	// Grammar Rule 13: top-level block with markers is a style rule.
	s := mustParse(t, "Text.muted { color: $muted }")
	sb, ok := s.Items[0].(*StyleBlock)
	if !ok {
		t.Fatalf("item = %T, want StyleBlock", s.Items[0])
	}
	if sb.Sel.Type != "Text" || len(sb.Sel.Classes) != 1 || sb.Sel.Classes[0] != "muted" {
		t.Errorf("selector = %+v", sb.Sel)
	}
}

func TestSeparatorRules(t *testing.T) {
	// trailing comma is fine; missing separator on one line is not
	mustParse(t, "Box { fill: $slab, border: $muted, }")
	_, err := Parse("t.ksl", []byte("Box { fill: $slab border: $muted }"))
	if err == nil || !strings.Contains(err.Error(), "expected ',' or a newline") {
		t.Errorf("Parse() err = %v, want separator error", err)
	}
	_, err = Parse("t.ksl", []byte("Box { fill: $slab Text { } }"))
	if err == nil || !strings.Contains(err.Error(), "expected ',' or a newline") {
		t.Errorf("Parse() err = %v, want separator error before child", err)
	}
}

func TestParseErrorPosition(t *testing.T) {
	_, err := Parse("t.ksl", []byte("Window {\n  Box { fill  $slab }\n}"))
	var el ErrorList
	if !errors.As(err, &el) {
		t.Fatalf("Parse() err = %v, want ErrorList", err)
	}
	if el[0].Pos.Line != 2 || el[0].Pos.Col != 15 {
		t.Errorf("error position = %v, want 2:15", el[0].Pos)
	}
}
