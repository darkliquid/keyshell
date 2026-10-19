// Package render hosts the gogpu layer surface and presents the composed frame.
package render

import (
	"image"

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
}

// Present runs the layer-surface overlay, drawing each frame from the
// launcher. It must run on the main goroutine and blocks until the app quits.
func Present(app *gogpu.App, l *launcher.Launcher, th *theme.Theme, cfg PresentConfig) error {
	p := &presenter{
		l:    l,
		th:   th,
		cfg:  cfg,
		draw: gg.NewContext(cfg.Width, cfg.Height),
		rdr:  New(),
	}
	app = p.configure(app)
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

		if p.tex == nil {
			p.drawer = dc.AsTextureDrawer()
			tex, err := p.drawer.TextureCreator().NewTextureFromRGBA(p.cfg.Width, p.cfg.Height, straight.Pix)
			if err != nil {
				return
			}
			p.tex = tex
		} else if u, ok := p.tex.(gpucontext.TextureUpdater); ok {
			_ = u.UpdateData(straight.Pix)
		}
		_ = p.drawer.DrawTexture(p.tex, 0, 0)
	})
	return app
}
