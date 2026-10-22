package ksl

import "testing"

func TestExprPrecedence(t *testing.T) {
	tests := []struct{ src, want string }{
		{"width: $results.len * 12", "($results.len * 12)"},
		{"width: 1 + 2 * 3", "(1 + (2 * 3))"},
		{"width: (1 + 2) * 3", "((1 + 2) * 3)"},
		{`when: $open && $focused || $mode == "media"`, `(($open && $focused) || ($mode == "media"))`},
		{"y: -$reveal.h / 2", "(-$reveal.h / 2)"},
		{`text: $open ? "on" : "off"`, `$open ? "on" : "off"`},
		{"stroke: rand(-4, 4) * 1deg", "(rand(-4, 4) * 1deg)"},
		{"opacity: !$open", "!$open"},
	}
	for _, tt := range tests {
		p := mustParse(t, "Box { "+tt.src+" }")
		prop := p.Nodes()[0].Props[0]
		if got := prop.Val.String(); got != tt.want {
			t.Errorf("%s:\n got %q\nwant %q", tt.src, got, tt.want)
		}
	}
}

func TestTernaryRightAssoc(t *testing.T) {
	p := mustParse(t, "Text { text: $a ? $b ? 1 : 2 : 3 }")
	ter, ok := p.Nodes()[0].Props[0].Val.(*Ternary)
	if !ok {
		t.Fatalf("val = %T, want Ternary", p.Nodes()[0].Props[0].Val)
	}
	if _, ok := ter.Then.(*Ternary); !ok {
		t.Errorf("then = %T, want nested Ternary (right-assoc)", ter.Then)
	}
}

func TestExprMembersAndCalls(t *testing.T) {
	p := mustParse(t, `Text { text: $item.title.toUpper() }`)
	v := p.Nodes()[0].Props[0].Val
	m, ok := v.(*MethodCall)
	if !ok || m.Name != "toUpper" || len(m.Args) != 0 {
		t.Fatalf("val = %#v, want MethodCall toUpper", v)
	}
	mem, ok := m.X.(*Member)
	if !ok || mem.Name != "title" {
		t.Fatalf("target = %#v, want Member title", m.X)
	}
	r, ok := mem.X.(*Ref)
	if !ok || r.Name != "item" {
		t.Fatalf("head = %#v, want Ref item", mem.X)
	}
}

func TestExprRefPathArrayAndInline(t *testing.T) {
	p := mustParse(t, `Stack { w: #band.width, assets: ["bg.png", "t.jpg"], viewport: { height: 320px } }`)
	n := p.Nodes()[0]
	if rp, ok := n.Props[0].Val.(*RefPath); !ok || rp.ID != "band" || rp.Prop != "width" {
		t.Errorf("w = %#v, want RefPath band.width", n.Props[0].Val)
	}
	arr, ok := n.Props[1].Val.(*ArrayVal)
	if !ok || len(arr.Elems) != 2 {
		t.Fatalf("assets = %#v, want 2-element array", n.Props[1].Val)
	}
	ib, ok := n.Props[2].Val.(*InlineBlock)
	if !ok || len(ib.Props) != 1 || ib.Props[0].Key != "height" {
		t.Errorf("viewport = %#v, want inline block with height", n.Props[2].Val)
	}
}

func TestExprErrors(t *testing.T) {
	for _, src := range []string{
		"x: 1 +",       // dangling operator
		"x: (1",        // unclosed paren
		"x: #zzz",      // hash name that is neither colour nor refpath
		"x: #band.a.b", // refpath is exactly one prop
		"x: [1, 2",     // unclosed array
	} {
		if _, err := Parse("t.ksl", []byte("Box { "+src+" }")); err == nil {
			t.Errorf("Parse(%q) = nil error, want failure", src)
		}
	}
}
