package ksl

import (
	"errors"
	"testing"
)

func TestPositionString(t *testing.T) {
	p := Position{Offset: 42, Line: 3, Col: 14}
	if got, want := p.String(), "3:14"; got != want {
		t.Errorf("Position.String() = %q, want %q", got, want)
	}
}

func TestErrorFormat(t *testing.T) {
	e := Error{Name: "ui.ksl", Pos: Position{Line: 3, Col: 14}, Msg: "expected ',' or a newline"}
	want := "ui.ksl:3:14: expected ',' or a newline"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrorList(t *testing.T) {
	var empty ErrorList
	if err := empty.OrNil(); err != nil {
		t.Errorf("OrNil() on empty list = %v, want nil", err)
	}
	l := ErrorList{
		{Name: "a.ksl", Pos: Position{Line: 1}, Msg: "first"},
		{Name: "a.ksl", Pos: Position{Line: 9}, Msg: "second"},
	}
	if err := l.OrNil(); err == nil {
		t.Fatal("OrNil() on non-empty list = nil, want error")
	}
	want := "a.ksl:1:0: first (and 1 more errors)"
	if got := l.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	var el ErrorList
	if !errors.As(l.OrNil(), &el) { // the returned error IS the list
		t.Errorf("OrNil() should return the ErrorList itself")
	}
}
