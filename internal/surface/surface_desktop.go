//go:build darwin || windows

package surface

import (
	"errors"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

type desktopSurfaceHost struct {
	app *gogpu.App
	cfg SurfaceConfig
}

// NewHost returns the platform-appropriate SurfaceHost.
func NewHost() SurfaceHost {
	return &desktopSurfaceHost{}
}

func (h *desktopSurfaceHost) Init(cfg SurfaceConfig) error {
	if h == nil {
		return errors.New("surface: host not initialized")
	}
	h.cfg = cfg
	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle(cfg.Title).
		WithSize(cfg.Width, cfg.Height).
		WithTransparent(cfg.Transparent).
		WithFrameless(true).
		WithContinuousRender(true))

	h.app = app
	return nil
}

func (h *desktopSurfaceHost) SetVisible(open bool) error {
	if h == nil || h.app == nil {
		return nil
	}
	return nil
}

func (h *desktopSurfaceHost) SetKeyboardFocus(exclusive bool) error {
	if h == nil || h.app == nil {
		return nil
	}
	return nil
}

func (h *desktopSurfaceHost) SetInputRegion(interactive bool) error {
	if h == nil || h.app == nil {
		return nil
	}
	return nil
}

func (h *desktopSurfaceHost) RequestRedraw() {
	if h != nil && h.app != nil {
		h.app.RequestRedraw()
	}
}

func (h *desktopSurfaceHost) Run() error {
	if h == nil || h.app == nil {
		return errors.New("surface: host not initialized")
	}
	return h.app.Run()
}

func (h *desktopSurfaceHost) Quit() {
	if h != nil && h.app != nil {
		h.app.Quit()
	}
}

func (h *desktopSurfaceHost) App() *gogpu.App {
	if h == nil {
		return nil
	}
	return h.app
}

func (h *desktopSurfaceHost) EventSource() gpucontext.EventSource {
	if h != nil && h.app != nil {
		return h.app.EventSource()
	}
	return nil
}
