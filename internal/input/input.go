// Package input adapts the gogpu event source to the launcher. It is the one
// place keyboard and pointer events become launcher calls, so an IME-capable
// or different backend can replace it without touching the UI.
package input

import (
	"context"
	"log/slog"
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

func (a *Adapter) onKeyPress(key gpucontext.Key, mods gpucontext.Modifiers) {
	ctx, span := telemetry.StartSpan(context.Background(), "keyshell.input.key",
		trace.WithAttributes(attribute.String(telemetry.Attrs.Kind, "key"),
			attribute.String("keyshell.key", key.String())))
	defer span.End()
	telemetry.InputEvents().Add(ctx, 1,
		metric.WithAttributes(attribute.String(telemetry.Attrs.Kind, "key")))

	if a.l.PanelOpen() {
		a.panelKey(key, mods)
		return
	}

	switch key {
	case gpucontext.KeyEscape:
		slog.Debug("input key", "key", key, "action", "escape")
		a.l.Escape()
	case gpucontext.KeyEnter:
		slog.Debug("input key", "key", key, "action", "enter")
		a.l.Enter()
	case gpucontext.KeyBackspace:
		slog.Debug("input key", "key", key, "action", "backspace")
		a.setQuery(ctx, trimLast(a.l.Query()))
	case gpucontext.KeyTab:
		slog.Debug("input key", "key", key, "action", "tab")
		// mode cycling is wired by the host; launcher just changes mode
	case gpucontext.KeyLeft, gpucontext.KeyUp:
		slog.Debug("input key", "key", key, "action", "select", "delta", -1)
		a.l.Select(-1)
	case gpucontext.KeyRight, gpucontext.KeyDown:
		slog.Debug("input key", "key", key, "action", "select", "delta", 1)
		a.l.Select(1)
	default:
		slog.Debug("input key", "key", key, "action", "unhandled")
	}
}

// panelKey routes a key press while the control panel is open: up/down move
// between rows, left/right adjust the focused row (coarse with Shift), enter
// activates it, and escape/backspace/typing edit or close it.
func (a *Adapter) panelKey(key gpucontext.Key, mods gpucontext.Modifiers) {
	delta := 1
	if mods&gpucontext.ModShift != 0 {
		delta = 5
	}
	switch key {
	case gpucontext.KeyEscape:
		a.l.Escape()
	case gpucontext.KeyEnter:
		a.l.PanelActivate()
	case gpucontext.KeyBackspace:
		a.l.PanelBackspace()
	case gpucontext.KeyUp:
		a.l.PanelNavigate(-1)
	case gpucontext.KeyDown:
		a.l.PanelNavigate(1)
	case gpucontext.KeyLeft:
		a.l.PanelAdjust(-delta)
	case gpucontext.KeyRight:
		a.l.PanelAdjust(delta)
	default:
		slog.Debug("input key", "key", key, "action", "panel-unhandled")
	}
}

func (a *Adapter) onTextInput(text string) {
	ctx, span := telemetry.StartSpan(context.Background(), "keyshell.input.text",
		trace.WithAttributes(attribute.String(telemetry.Attrs.Kind, "text")))
	defer span.End()
	telemetry.InputEvents().Add(ctx, 1,
		metric.WithAttributes(attribute.String(telemetry.Attrs.Kind, "text")))
	// Log the length, not the text: a query can hold sensitive input.
	slog.Debug("input text", "chars", len(text))
	if a.l.PanelOpen() {
		a.l.PanelType(text)
		return
	}
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
	slog.Debug("input pointer", "button", button, "x", x, "y", y)
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
