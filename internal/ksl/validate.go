package ksl

import "fmt"

var primitives = map[string]bool{
	"Window": true, "Box": true, "Text": true, "Icon": true, "Cursor": true,
	"Image": true, "Pill": true, "Meter": true, "Row": true, "Column": true,
	"Grid": true, "Stack": true, "List": true, "Path": true, "Group": true,
}

var reservedNames = map[string]bool{
	"theme": true, "state": true, "transition": true, "def": true,
	"include": true, "font": true, "assets": true, "let": true,
}

// stateNames is the v1 built-in set (spec §5).
var stateNames = map[string]bool{
	"open": true, "closed": true, "focused": true, "selected": true, "searching": true,
}

var listSubBlocks = map[string]bool{
	"template": true, "empty": true, "overflowSummary": true,
}

// Validate checks the scene-level semantic rules and returns every
// violation. Syntax has already been checked by Parse.
func Validate(s *Scene) error { return ValidateWith(s) }

// ValidateWith is Validate plus host-registered node names (a renderer's
// built-in painters, such as Card, are legal node types).
func ValidateWith(s *Scene, extra ...string) error {
	v := &validator{ids: map[string]Position{}, extra: map[string]bool{}}
	for _, name := range extra {
		v.extra[name] = true
	}
	v.run(s)
	return v.errs.OrNil()
}

type validator struct {
	name  string
	ids   map[string]Position
	defs  map[string]bool
	lets  map[string]bool
	keyfs map[string]bool
	extra map[string]bool
	errs  ErrorList
}

func (v *validator) errf(pos Position, format string, args ...any) {
	v.errs = append(v.errs, Error{Name: v.name, Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

func (v *validator) run(s *Scene) {
	v.name = s.Name
	v.defs = map[string]bool{}
	v.lets = map[string]bool{}
	v.keyfs = map[string]bool{}
	var roots []*Node
	for _, it := range s.Items {
		switch x := it.(type) {
		case *Def:
			if reservedNames[x.Name] {
				v.errf(x.Pos, "component name %q is reserved", x.Name)
			}
			if v.defs[x.Name] {
				v.errf(x.Pos, "duplicate component %q", x.Name)
			}
			v.defs[x.Name] = true
		case *LetDecl:
			if x.Name == "" {
				v.errf(x.Pos, "let declaration must have a name")
			} else {
				if reservedNames[x.Name] {
					v.errf(x.Pos, "let variable name %q is reserved", x.Name)
				}
				if v.lets[x.Name] {
					v.errf(x.Pos, "duplicate let variable %q", x.Name)
				}
				v.lets[x.Name] = true
			}
		case *Keyframes:
			if v.keyfs[x.Name] {
				v.errf(x.Pos, "duplicate @keyframes %q", x.Name)
			}
			v.keyfs[x.Name] = true
			v.checkFrames(x)
		case *StateBlock:
			v.checkStateName(x)
		case *Node:
			roots = append(roots, x)
		}
	}
	if len(roots) != 1 {
		v.errf(Position{Line: 1, Col: 1}, "a scene must have exactly one top-level node, found %d", len(roots))
	} else if roots[0].Name != "Window" {
		v.errf(roots[0].Pos, "the scene root must be a Window, found %q", roots[0].Name)
	}
	for _, it := range s.Items {
		if n, ok := it.(*Node); ok {
			v.walk(n, false)
		}
	}
}

func (v *validator) walk(n *Node, inList bool) {
	if !(inList && listSubBlocks[n.Name]) && !primitives[n.Name] && !v.defs[n.Name] && !v.extra[n.Name] {
		v.errf(n.Pos, "unknown node type %q", n.Name)
	}
	if n.ID != "" {
		if prev, dup := v.ids[n.ID]; dup {
			v.errf(n.Pos, "duplicate id #%s (first used at %s)", n.ID, prev)
		} else {
			v.ids[n.ID] = n.Pos
		}
	}
	if n.Name == "List" {
		v.checkList(n)
	}
	for _, sb := range n.States {
		v.checkStateName(sb)
	}
	for _, pr := range n.Props {
		if pr.Key == "animation" {
			v.checkAnim(pr)
		}
	}
	for _, c := range n.Children {
		v.walk(c, inList || n.Name == "List")
	}
}

func (v *validator) checkList(n *Node) {
	hasItems := false
	for _, pr := range n.Props {
		if pr.Key == "items" {
			hasItems = true
		}
	}
	if !hasItems {
		v.errf(n.Pos, "List requires \"items\"")
	}
	for _, c := range n.Children {
		if !listSubBlocks[c.Name] {
			v.errf(c.Pos, "a List may only contain template/empty/overflowSummary blocks, found %q", c.Name)
		}
	}
}

func (v *validator) checkStateName(sb *StateBlock) {
	if !stateNames[sb.Name] {
		v.errf(sb.Pos, "state name %q is not one of the built-ins (open, closed, focused, selected, searching)", sb.Name)
	}
}

func (v *validator) checkFrames(kf *Keyframes) {
	prev := -1.0
	for _, f := range kf.Frames {
		if f.Percent < 0 || f.Percent > 100 {
			v.errf(f.Pos, "keyframe %g%% out of range 0..100", f.Percent)
		}
		if f.Percent <= prev {
			v.errf(f.Pos, "keyframe %g%% must be greater than the previous frame", f.Percent)
		}
		prev = f.Percent
	}
}

func (v *validator) checkAnim(pr *Prop) {
	a, ok := pr.Val.(*AnimVal)
	if !ok {
		return
	}
	for _, step := range a.Steps {
		if !v.keyfs[step.Name] {
			v.errf(pr.Pos, "animation %q has no @keyframes", step.Name)
		}
	}
}
