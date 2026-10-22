package eval

import (
	"sort"

	"github.com/darkliquid/keyshell/internal/ksl"
)

// CascadeCtx carries what the current frame means for matching: the
// scene's style blocks and scene-level states, the pseudo-states active
// for the node being resolved, and the current mode attribute.
type CascadeCtx struct {
	Scene  *ksl.Scene
	Pseudo map[string]bool
	Mode   string
}

// resolveProps computes one node's final property map by applying, in
// increasing specificity: style rules (type/pseudo, class, id), active
// scene-level state targets, node-level state blocks, then the node's
// own properties (Grammar Rule 13, spec §4). The bool reports whether
// the node exists at all (when + states).
func resolveProps(n *ksl.Node, env *Env, ctx *CascadeCtx) (map[string]Value, bool, error) {
	props := map[string]Value{}

	// 1. Style rules, ordered by specificity then declaration order.
	var rules []cascadeRule
	for i, sb := range ctx.Scene.Styles() {
		if selectorMatches(sb.Sel, n, ctx) {
			rules = append(rules, cascadeRule{sb, specificity(sb.Sel), i})
		}
	}
	applyRules(rules, props, env)

	// 2. Active scene-level states targeting this node.
	for _, sb := range ctx.Scene.States() {
		if !stateActive(sb.Name, env) {
			continue
		}
		rules = rules[:0]
		for i, tgt := range sb.Targets {
			if selectorMatches(tgt.Sel, n, ctx) {
				rules = append(rules, cascadeRule{tgt, specificity(tgt.Sel), i})
			}
		}
		applyRules(rules, props, env)
	}

	// 3. Node-local properties.
	if err := applyProps(props, n.Props, env); err != nil {
		return nil, false, err
	}

	// 4. Node-level state blocks (state name or item pseudo): conditional
	// overrides of the node's own properties.
	for _, sb := range n.States {
		if !stateActive(sb.Name, env) && !ctx.Pseudo[sb.Name] {
			continue
		}
		if err := applyProps(props, sb.Props, env); err != nil {
			return nil, false, err
		}
	}

	// 5. when: the guard can hide the whole subtree.
	if w, ok := props["when"]; ok {
		if !truthy(w) {
			return nil, false, nil
		}
		delete(props, "when")
	}
	return props, true, nil
}

type cascadeRule struct {
	sb    *ksl.StyleBlock
	spec  int
	order int
}

func applyRules(rules []cascadeRule, dst map[string]Value, env *Env) error {
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].spec != rules[j].spec {
			return rules[i].spec < rules[j].spec
		}
		return rules[i].order < rules[j].order
	})
	for _, r := range rules {
		if err := applyProps(dst, r.sb.Props, env); err != nil {
			return err
		}
	}
	return nil
}

func applyProps(dst map[string]Value, props []*ksl.Prop, env *Env) error {
	for _, pr := range props {
		v, err := env.Eval(pr.Val)
		if err != nil {
			return err
		}
		dst[pr.Key] = v
	}
	return nil
}

// specificity scores a selector: id (3) beats class (2) beats
// type/pseudo/attribute (1).
func specificity(sel ksl.Selector) int {
	s := 0
	if sel.Type != "" {
		s++
	}
	if sel.ID != "" {
		s += 3
	}
	s += len(sel.Classes) * 2
	s += len(sel.Pseudos)
	if sel.AttrName != "" {
		s++
	}
	return s
}

// stateActive reports whether a built-in state name holds.
func stateActive(name string, env *Env) bool {
	switch name {
	case "open":
		return env.st.Open
	case "closed":
		return !env.st.Open
	case "focused":
		return env.st.Focused
	case "searching":
		return env.st.Searching
	case "selected":
		return env.st.Selected >= 0 && env.st.Selected < len(env.st.Results)
	}
	return false
}

// selectorMatches checks type, id, classes, pseudos and the mode
// attribute (spec §4).
func selectorMatches(sel ksl.Selector, n *ksl.Node, ctx *CascadeCtx) bool {
	if sel.Type != "" && sel.Type != n.Name {
		return false
	}
	if sel.ID != "" && sel.ID != n.ID {
		return false
	}
	for _, c := range sel.Classes {
		if !hasClass(n, c) {
			return false
		}
	}
	for _, p := range sel.Pseudos {
		if !ctx.Pseudo[p] {
			return false
		}
	}
	if sel.AttrName != "" {
		if sel.AttrName != "mode" || ctx.Mode != sel.AttrVal {
			return false
		}
	}
	return true
}

func hasClass(n *ksl.Node, c string) bool {
	for _, have := range n.Classes {
		if have == c {
			return true
		}
	}
	return false
}
