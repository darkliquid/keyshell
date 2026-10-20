// Package render hosts the gogpu layer surface and presents the composed frame.
package render

import (
	"image"
	"time"

	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gpucontext"
)

// PresentConfig describes the overlay surface.
type PresentConfig struct {
	Width, Height int
	Namespace     string
	Reveal        config.Reveal
	// Renderer draws the frames and answers band hit tests. When nil a fresh
	// one is created, which is only useful in tests.
	Renderer *Renderer
}

// Present runs the layer-surface overlay, drawing each frame from the
// launcher. It must run on the main goroutine and blocks until the app quits.
func Present(app *gogpu.App, l *launcher.Launcher, th *theme.Theme, cfg PresentConfig) error {
	// gogpu renders on demand: nothing repaints unless a redraw is requested.
	// Every launcher change schedules one, so typing and selection are visible
	// immediately instead of waiting for the next unrelated event.
	l.OnChange(app.RequestRedraw)

	rdr := cfg.Renderer
	if rdr == nil {
		rdr = New()
	}
	p := &presenter{
		l:    l,
		th:   th,
		cfg:  cfg,
		draw: gg.NewContext(cfg.Width, cfg.Height),
		rdr:  rdr,
	}
	app = p.configure(app)

	// The cursor blinks, so while the overlay is open it needs periodic
	// redraws; when it closes the ticker stops requesting them.
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if l.IsOpen() {
				app.RequestRedraw()
			}
		}
	}()

	return app.Run()
}

type presenter struct {
	l      *launcher.Launcher
	th     *theme.Theme
	cfg    PresentConfig
	draw   *gg.Context
	rdr    *Renderer
	tex    gpucontext.Texture
	drawer gpucontext.TextureDrawer
}

func (p *presenter) configure(app *gogpu.App) *gogpu.App {
	app.OnDraw(func(dc *gogpu.Context) {
		// Clear transparent: only painted content is visible, so the surface
		// shows the desktop between and around cards.
		dc.ClearColor(gmath.Color{R: 0, G: 0, B: 0, A: 0})

		// Draw the frame into the scratch context.
		p.rdr.DrawFrame(p.draw, p.l, p.th, p.cfg.Reveal)
		img, ok := p.draw.Image().(*image.RGBA)
		if !ok {
			return
		}
		// S4: the upload path expects straight alpha.
		straight := unpremultiply(img)

		// Create a fresh texture each draw. UpdateData stores the bytes
		// verbatim while NewTextureFromRGBA converts straight alpha to the
		// premultiplied form the renderer blends with, so reusing a texture via
		// UpdateData produces black frames. Frames are drawn on change only, so
		// the allocation is not on a hot path.
		drawer := dc.AsTextureDrawer()
		tex, err := drawer.TextureCreator().NewTextureFromRGBA(p.cfg.Width, p.cfg.Height, straight.Pix)
		if err != nil {
			return
		}
		p.drawer = drawer
		p.tex = tex
		_ = p.drawer.DrawTexture(p.tex, 0, 0)
	})
	return app
}
