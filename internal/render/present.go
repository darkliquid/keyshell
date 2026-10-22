// Package render hosts the gogpu layer surface and presents the composed frame.
package render

import (
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
		l:   l,
		th:  th,
		cfg: cfg,
		rdr: rdr,
		// gg draws into a buffer we own, so a frame never has to be copied out
		// of the context before upload.
		premul: make([]byte, cfg.Width*cfg.Height*4),
	}
	p.pixmap = gg.NewPixmapFromBuffer(p.premul, cfg.Width, cfg.Height)
	p.draw = gg.NewContextForPixmap(p.pixmap)
	app = p.configure(app)

	return app.Run()
}

type presenter struct {
	l       *launcher.Launcher
	th      *theme.Theme
	cfg     PresentConfig
	draw    *gg.Context
	pixmap  *gg.Pixmap
	premul  []byte
	rdr     *Renderer
	tex     gpucontext.Texture
	updater gpucontext.TextureUpdater
	drawer  gpucontext.TextureDrawer
}

func (p *presenter) configure(app *gogpu.App) *gogpu.App {
	app.OnDraw(func(dc *gogpu.Context) {
		// Clear transparent: only painted content is visible, so the surface
		// shows the desktop between and around cards.
		dc.ClearColor(gmath.Color{R: 0, G: 0, B: 0, A: 0})
		if !p.l.IsOpen() {
			// Continuous rendering keeps the loop at vsync; a hidden overlay
			// only needs the cleared frame, not a composed one.
			return
		}

		// Draw into our own buffer and clip it to the slab in place. gg's output
		// is already premultiplied, which is the layout UpdateData expects, so
		// later frames only upload bytes.
		p.rdr.DrawFrame(p.draw, p.l, p.th, p.cfg.Reveal)
		img := p.pixmap.ImageView()
		p.rdr.MaskFrame(img)
		// The control panel is an overlay painted *after* the band mask, so it
		// is not clipped to the band and may extend beyond its trapezoid.
		if p.rdr.scene != nil {
			p.rdr.scene.DrawPanelOverlay(p.draw, p.th)
		}

		drawer := dc.AsTextureDrawer()
		p.drawer = drawer
		if p.tex == nil {
			// NewTextureFromRGBA is the only constructor and it premultiplies,
			// so the first frame goes through it and then the texture is reused.
			p.tex = nil
			tex, err := drawer.TextureCreator().NewTextureFromRGBA(p.cfg.Width, p.cfg.Height, unpremultiplied(img))
			if err != nil {
				return
			}
			p.tex = tex
			p.updater, _ = tex.(gpucontext.TextureUpdater)
		} else if p.updater != nil {
			if err := p.updater.UpdateData(img.Pix); err != nil {
				p.tex = nil
				return
			}
		}
		_ = p.drawer.DrawTexture(p.tex, 0, 0)
	})
	return app
}
