// Package surface provides an abstraction over windowing and layer-shell
// presentation surfaces across operating systems.
package surface

import (
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

// SurfaceConfig describes the window or layer surface to initialize.
type SurfaceConfig struct {
	Width       int
	Height      int
	Title       string
	Namespace   string
	Transparent bool
}

// SurfaceHost abstracts the presentation surface across operating systems.
type SurfaceHost interface {
	// Init initializes the presentation surface with the given configuration.
	Init(cfg SurfaceConfig) error

	// SetVisible controls the visibility of the surface.
	SetVisible(open bool) error

	// SetKeyboardFocus requests keyboard focus from the windowing environment or layer-shell compositor.
	SetKeyboardFocus(exclusive bool) error

	// SetInputRegion configures whether pointer and touch input regions are interactive.
	SetInputRegion(interactive bool) error

	// RequestRedraw schedules a redraw of the surface.
	RequestRedraw()

	// Run starts the surface event loop. It returns an error if the host is uninitialized or encounters a runtime error.
	Run() error

	// Quit terminates the surface event loop.
	Quit()

	// App returns the underlying gogpu App instance, or nil if uninitialized.
	App() *gogpu.App

	// EventSource returns the event source for receiving input events, or nil if uninitialized.
	EventSource() gpucontext.EventSource
}
