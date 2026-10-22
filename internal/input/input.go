// Package input adapts the gogpu event source to the launcher. It is the one
// place keyboard and pointer events become launcher calls, so an IME-capable
// or different backend can replace it without touching the UI.
package input

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/darkliquid/keyshell/internal/telemetry"
	"github.com/gogpu/gpucontext"
)

// Adapter wires an event source to the launcher.
type Adapter struct {
	l       *launcher.Launcher
	events  gpucontext.EventSource
	onPress func(x, y float64)

	// Typing updates the query text at once but only searches after the keys
	// settle, because a merged search blocks for the length of a plugin round
	// trip and would stutter the overlay on every keystroke.
	searchTimer *time.Timer
}

// searchSettle is how long typing must pause before the merged search runs.
const searchSettle = 80 * time.Millisecond

// New builds an adapter over the launcher and event source.
func New(l *launcher.Launcher, events gpucontext.EventSource) *Adapter {
	a := &Adapter{l: l, events: events}
	events.OnKeyPress(a.onKeyPress)
	events.OnTextInput(a.onTextInput)
	events.OnMousePress(a.onMousePress)
	return a
}

func (a *Adapter) onKeyPress(key gpucontext.Key, _ gpucontext.Modifiers) {
	ctx, span := telemetry.StartSpan(context.Background(), "keyshell.input.key",
		trace.WithAttributes(attribute.String(telemetry.Attrs.Kind, "key"),
			attribute.String("keyshell.key", key.String())))
	defer span.End()
	telemetry.InputEvents().Add(ctx, 1,
		metric.WithAttributes(attribute.String(telemetry.Attrs.Kind, "key")))

	switch key {
	case gpucontext.KeyEscape:
		a.l.Escape()
	case gpucontext.KeyEnter:
		a.l.Enter()
	case gpucontext.KeyBackspace:
		a.setQuery(ctx, trimLast(a.l.Query()))
	case gpucontext.KeyTab:
		// mode cycling is wired by the host; launcher just changes mode
	case gpucontext.KeyLeft:
		a.l.Select(-1)
	case gpucontext.KeyRight:
		a.l.Select(1)
	}
}

func (a *Adapter) onTextInput(text string) {
	ctx, span := telemetry.StartSpan(context.Background(), "keyshell.input.text",
		trace.WithAttributes(attribute.String(telemetry.Attrs.Kind, "text")))
	defer span.End()
	telemetry.InputEvents().Add(ctx, 1,
		metric.WithAttributes(attribute.String(telemetry.Attrs.Kind, "text")))
	a.setQuery(ctx, a.l.Query()+text)
}

// setQuery shows the text now and searches once typing pauses. The event's
// trace context rides along, so the query span it triggers is a child of the
// keystroke span.
func (a *Adapter) setQuery(ctx context.Context, text string) {
	a.l.SetQueryInput(text)
	if a.searchTimer != nil {
		a.searchTimer.Stop()
	}
	a.searchTimer = time.AfterFunc(searchSettle, func() {
		if ctx != nil {
			a.l.SetQueryContext(ctx, text)
			return
		}
		a.l.SetQuery(text)
	})
}

// OnPress registers a callback for pointer presses in surface coordinates.
func (a *Adapter) OnPress(fn func(x, y float64)) {
	a.onPress = fn
}

func (a *Adapter) onMousePress(button gpucontext.MouseButton, x, y float64) {
	if button != gpucontext.MouseButtonLeft || a.onPress == nil {
		return
	}
	a.onPress(x, y)
}

func trimLast(s string) string {
	if len(s) == 0 {
		return s
	}
	return s[:len(s)-1]
}
