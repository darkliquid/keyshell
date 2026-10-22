package ksl

import (
	"strings"
	"testing"
)

func validateErrs(t *testing.T, src string) ErrorList {
	t.Helper()
	s, err := Parse("t.ksl", []byte(src))
	if err != nil {
		t.Fatalf("Parse() = %v", err)
	}
	el, ok := Validate(s).(ErrorList)
	if !ok && Validate(s) != nil {
		t.Fatalf("Validate() = %T, want ErrorList or nil", Validate(s))
	}
	return el
}

func wantMsg(t *testing.T, el ErrorList, sub string) {
	t.Helper()
	for _, e := range el {
		if strings.Contains(e.Msg, sub) {
			return
		}
	}
	t.Errorf("no error containing %q in %v", sub, el)
}

func TestValidateSceneRoot(t *testing.T) {
	wantMsg(t, validateErrs(t, "Box { }"), "root must be a Window")
	wantMsg(t, validateErrs(t, "Window { }, Window { }"), "exactly one top-level node")
	if el := validateErrs(t, "Window { }"); len(el) != 0 {
		t.Errorf("valid scene produced %v", el)
	}
}

func TestValidateNodeNames(t *testing.T) {
	wantMsg(t, validateErrs(t, "Window { Gizmo { } }"), `unknown node type "Gizmo"`)
	wantMsg(t, validateErrs(t, "def theme { Box { } }\nWindow { }"), "reserved")
	wantMsg(t, validateErrs(t, "Window { List { } }"), `List requires "items"`)
	wantMsg(t, validateErrs(t, `Window { List { items: $results, template { Box { } }, Box { } } }`), "template/empty/overflowSummary")
}

func TestValidateIDsAndStates(t *testing.T) {
	wantMsg(t, validateErrs(t, "Window { Box#a { }, Box#a { } }"), "duplicate id")
	wantMsg(t, validateErrs(t, "Window { Box { state hovered { fill: $accent } } }"), "state name")
	wantMsg(t, validateErrs(t, "state hovered { Box { opacity: 1 } }\nWindow { }"), "state name")
}

func TestValidateKeyframesAndAnimations(t *testing.T) {
	wantMsg(t, validateErrs(t, `
		@keyframes k { 50% { opacity: 1 } 0% { opacity: 0 } }
		Window { }`), "greater than the previous frame")
	wantMsg(t, validateErrs(t, `
		Window { Meter#m { animation: blink 800ms linear infinite } }`), `no @keyframes`)
	if el := validateErrs(t, `
		@keyframes blink { 0% { opacity: 1 } 100% { opacity: 0 } }
		Window { Meter#m { animation: blink 800ms linear infinite } }`); len(el) != 0 {
		t.Errorf("valid animation produced %v", el)
	}
}
