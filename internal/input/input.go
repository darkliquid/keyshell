// Package input adapts the gogpu event source to the launcher. It is the one
// place keyboard and pointer events become launcher calls, so an IME-capable
// or different backend can replace it without touching the UI.
package input

import (
	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/gogpu/gpucontext"
)

// Adapter wires an event source to the launcher.
type Adapter struct {
	l      *launcher.Launcher
	events gpucontext.EventSource
}

// New builds an adapter over the launcher and event source.
func New(l *launcher.Launcher, events gpucontext.EventSource) *Adapter {
	a := &Adapter{l: l, events: events}
	events.OnKeyPress(a.onKeyPress)
	events.OnTextInput(a.onTextInput)
	events.OnMousePress(a.onMousePress)
	return a
}

func (a *Adapter) onKeyPress(key gpucontext.Key, _ gpucontext.Modifiers) {
	switch key {
	case gpucontext.KeyEscape:
		a.l.Escape()
	case gpucontext.KeyEnter:
		a.l.Enter()
	case gpucontext.KeyBackspace:
		a.l.SetQuery(trimLast(a.l.Query()))
	case gpucontext.KeyTab:
		// mode cycling is wired by the host; launcher just changes mode
	case gpucontext.KeyLeft:
		a.l.Select(-1)
	case gpucontext.KeyRight:
		a.l.Select(1)
	}
}

func (a *Adapter) onTextInput(text string) {
	a.l.SetQuery(a.l.Query() + text)
}

func (a *Adapter) onMousePress(button gpucontext.MouseButton, x, y float64) {
	// A pointer press outside the surface is handled by focus loss at the
	// compositor level. Here we only need to commit any pending control value.
	_ = button
	_ = x
	_ = y
}

func trimLast(s string) string {
	if len(s) == 0 {
		return s
	}
	return s[:len(s)-1]
}
