//go:build !darwin && !windows

package surface

import (
	"errors"
	"os"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

type linuxSurfaceHost struct {
	app *gogpu.App
	cfg SurfaceConfig
}

// NewHost returns the platform-appropriate SurfaceHost.
func NewHost() SurfaceHost {
	return &linuxSurfaceHost{}
}

func (h *linuxSurfaceHost) Init(cfg SurfaceConfig) error {
	if h == nil {
		return errors.New("surface: host not initialized")
	}
	h.cfg = cfg
	if os.Getenv("GOGPU_WAYLAND_FRAME_CALLBACK") == "" {
		_ = os.Setenv("GOGPU_WAYLAND_FRAME_CALLBACK", "0")
	}

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle(cfg.Title).
		WithSize(cfg.Width, cfg.Height).
		WithTransparent(cfg.Transparent).
		WithContinuousRender(true).
		WithLayerShell(gogpu.LayerShellConfig{
			Namespace:     cfg.Namespace,
			Layer:         gogpu.LayerShellOverlay,
			Anchor:        gogpu.LayerShellAnchorTop | gogpu.LayerShellAnchorBottom | gogpu.LayerShellAnchorLeft | gogpu.LayerShellAnchorRight,
			Width:         int32(cfg.Width),
			Height:        int32(cfg.Height),
			Keyboard:      gogpu.LayerShellKeyboardOnDemand,
			ExclusiveZone: -1,
		}))

	h.app = app
	return nil
}

func (h *linuxSurfaceHost) SetVisible(open bool) error {
	if h == nil || h.app == nil {
		return nil
	}
	return nil
}

func (h *linuxSurfaceHost) SetKeyboardFocus(exclusive bool) error {
	if h == nil || h.app == nil {
		return nil
	}
	mode := gogpu.LayerShellKeyboardNone
	if exclusive {
		mode = gogpu.LayerShellKeyboardExclusive
	}
	return h.app.SetLayerKeyboardInteractivity(mode)
}

func (h *linuxSurfaceHost) SetInputRegion(interactive bool) error {
	if h == nil || h.app == nil {
		return nil
	}
	return h.app.SetInputRegion(interactive)
}

func (h *linuxSurfaceHost) RequestRedraw() {
	if h != nil && h.app != nil {
		h.app.RequestRedraw()
	}
}

func (h *linuxSurfaceHost) Run() error {
	if h == nil || h.app == nil {
		return errors.New("surface: host not initialized")
	}
	return h.app.Run()
}

func (h *linuxSurfaceHost) Quit() {
	if h != nil && h.app != nil {
		h.app.Quit()
	}
}

func (h *linuxSurfaceHost) App() *gogpu.App {
	if h == nil {
		return nil
	}
	return h.app
}

func (h *linuxSurfaceHost) EventSource() gpucontext.EventSource {
	if h != nil && h.app != nil {
		return h.app.EventSource()
	}
	return nil
}
