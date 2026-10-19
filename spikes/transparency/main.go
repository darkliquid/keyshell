//go:build linux

// SPIKE S4 (throwaway): does a layer-shell surface composite per-pixel alpha?
//
// A launcher overlay is not a solid rectangle: it wants a dimmed scrim over the
// desktop, cards with alpha, soft edges and text that is not fully opaque. If a
// layer surface can only ever be an opaque surface, the whole visual language
// changes, so this spike answers three things at once:
//
//   - does the compositor blend the surface with the desktop beneath it
//     (a fully transparent region must show the desktop unchanged)?
//   - is alpha blended correctly, or does the path lose it somewhere between gg
//     drawing, the texture upload and the swapchain (probed with 50% white, 50%
//     black and a 70% card, where a premultiplication mistake is obvious)?
//   - is the opaque case still exactly opaque (a control)?
//
// The spike draws its composition with gg - the same way the real overlay will,
// since gg is where the design lives - uploads it as a texture and presents it
// on a transparent layer surface. It prints its layout as JSON so the checker
// knows where to sample.
//
// Run:
//
//	mise run spike:s4                 # 20s, transparent surface
//	mise run spike:s4:verify          # capture desktop + overlay and compare
//	go run ./spikes/transparency -transparent=false   # control: opaque surface
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gogpu/gg"
	"github.com/gogpu/gg/text"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gpucontext"
)

const (
	width  = 800
	height = 450
)

// Palette, matching the theme.
var (
	ink     = gg.RGBA{R: 0xF2 / 255.0, G: 0xF2 / 255.0, B: 0xF7 / 255.0, A: 1}
	surface = gg.RGBA{R: 0x12 / 255.0, G: 0x12 / 255.0, B: 0x18 / 255.0, A: 1}
	accent  = gg.RGBA{R: 0xFF / 255.0, G: 0x2E / 255.0, B: 0x63 / 255.0, A: 1}
	cyan    = gg.RGBA{R: 0x08 / 255.0, G: 0xF7 / 255.0, B: 0xFE / 255.0, A: 1}
)

// sample describes one probe point the checker asserts against.
type sample struct {
	Name  string   `json:"name"`
	X     int      `json:"x"` // screen coordinates: the surface is anchored at 0,0
	Y     int      `json:"y"`
	Kind  string   `json:"kind"`            // desktop, opaque or blend
	Fill  [3]uint8 `json:"fill,omitempty"`  // colour drawn there, if any
	Alpha float64  `json:"alpha,omitempty"` // source alpha drawn there, if any
	Note  string   `json:"note,omitempty"`
}

var defaultFace = "/usr/share/fonts/redhat/RedHatDisplay-BlackItalic.otf"

func main() {
	out := flag.String("out", "out/s4-transparency.png", "where to write the composed PNG")
	unpremultiply := flag.Bool("unpremultiply", false,
		"convert the gg image from premultiplied to straight alpha before upload")
	facePath := flag.String("face", defaultFace, "display face to use")
	transparent := flag.Bool("transparent", true, "request a premultiplied-alpha swapchain")
	duration := flag.Duration("duration", 20*time.Second, "how long to keep the surface up")
	namespace := flag.String("namespace", "keyshell-spike-s4", "layer surface namespace")
	flag.Parse()

	img, layout, err := compose(*facePath)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := gg.NewContextForImage(img).SavePNG(*out); err != nil {
		log.Fatal(err)
	}

	// The layout goes to stdout as JSON so verify.py samples the right pixels.
	layoutJSON, err := json.Marshal(layout)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s (%dx%d)\n", *out, width, height)
	fmt.Printf("S4-LAYOUT %s\n", layoutJSON)

	if *unpremultiply {
		img = toStraightAlpha(img)
		fmt.Println("uploading straight-alpha data (unpremultiplied)")
	} else {
		fmt.Println("uploading the image as-is (premultiplied, Go image.RGBA convention)")
	}

	present(img, *transparent, *namespace, *duration)
}

// compose draws the alpha probes on a fully transparent canvas.
func compose(facePath string) (*image.RGBA, []sample, error) {
	source, err := text.NewFontSourceFromFile(facePath)
	if err != nil {
		return nil, nil, fmt.Errorf("load face: %w", err)
	}
	defer func() { _ = source.Close() }()

	dc := gg.NewContext(width, height)
	dc.ClearWithColor(gg.RGBA{R: 0, G: 0, B: 0, A: 0}) // transparent

	// 1. Opaque cyan rectangle: the control. If this is not exactly cyan on
	//    screen, nothing else here is trustworthy.
	dc.SetColor(cyan)
	dc.DrawRectangle(40, 40, 160, 160)
	if err := dc.Fill(); err != nil {
		return nil, nil, err
	}

	// 2. White at 50%: correct premultiplied blending gives (desktop+255)/2.
	//    If the buffer holds straight alpha but is read as premultiplied, this
	//    clips towards white instead.
	dc.SetRGBA(1, 1, 1, 0.5)
	dc.DrawRectangle(240, 40, 160, 160)
	if err := dc.Fill(); err != nil {
		return nil, nil, err
	}

	// 3. Black at 50%: the same probe in the other direction - misuse of
	//    premultiplication here goes to black rather than half the desktop.
	dc.SetRGBA(0, 0, 0, 0.5)
	dc.DrawRectangle(440, 40, 160, 160)
	if err := dc.Fill(); err != nil {
		return nil, nil, err
	}

	// 4. The card case: the surface colour at 70% with a thick accent border.
	dc.SetRGBA(surface.R, surface.G, surface.B, 0.7)
	dc.DrawRectangle(40, 260, 360, 140)
	if err := dc.Fill(); err != nil {
		return nil, nil, err
	}
	dc.SetColor(accent)
	dc.SetLineWidth(3)
	dc.DrawRectangle(40, 260, 360, 140)
	if err := dc.Stroke(); err != nil {
		return nil, nil, err
	}

	// 5. Semi-transparent text: alpha must survive the glyph rasteriser too.
	dc.SetFont(source.Face(48))
	dc.SetRGBA(ink.R, ink.G, ink.B, 0.6)
	dc.DrawString("TRANSPARENT", 420, 300)

	rgba, ok := dc.Image().(*image.RGBA)
	if !ok {
		converted := image.NewRGBA(image.Rect(0, 0, width, height))
		draw.Draw(converted, converted.Bounds(), dc.Image(), image.Point{}, draw.Src)
		rgba = converted
	}

	layout := []sample{
		{Name: "transparent", X: 700, Y: 100, Kind: "desktop", Note: "must equal the desktop captured without the overlay"},
		{Name: "transparent2", X: 100, Y: 430, Kind: "desktop", Note: "second transparent probe"},
		{Name: "opaque-cyan", X: 120, Y: 120, Kind: "opaque", Fill: [3]uint8{0x08, 0xF7, 0xFE}, Alpha: 1, Note: "must equal the fill colour exactly"},
		{Name: "white-50", X: 320, Y: 120, Kind: "blend", Fill: [3]uint8{255, 255, 255}, Alpha: 0.5, Note: "must be half way to white from the desktop"},
		{Name: "black-50", X: 520, Y: 120, Kind: "blend", Fill: [3]uint8{0, 0, 0}, Alpha: 0.5, Note: "must be half way to black from the desktop"},
		{Name: "card-70", X: 200, Y: 330, Kind: "blend", Fill: [3]uint8{0x12, 0x12, 0x18}, Alpha: 0.7, Note: "the card case"},
	}
	return rgba, layout, nil
}

// toStraightAlpha converts a premultiplied RGBA image - what Go's image.RGBA and
// therefore gg produce - into straight (non-premultiplied) RGBA.
//
// The spike needs this because NewTextureFromRGBA documents only "RGBA, 8 bits
// per channel" and behaves as if the data is straight alpha: semi-transparent
// pixels then have their colour multiplied by alpha a second time and come out
// too dark, while fully opaque and fully transparent pixels are unaffected. See
// verify.py for the arithmetic the checker proves.
func toStraightAlpha(img *image.RGBA) *image.RGBA {
	out := image.NewRGBA(img.Bounds())
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			i := img.PixOffset(x, y)
			a := img.Pix[i+3]
			if a == 0 || a == 255 {
				out.Pix[i] = img.Pix[i]
				out.Pix[i+1] = img.Pix[i+1]
				out.Pix[i+2] = img.Pix[i+2]
				out.Pix[i+3] = a
				continue
			}
			out.Pix[i] = uint8(min(255, int(img.Pix[i])*255/int(a)))
			out.Pix[i+1] = uint8(min(255, int(img.Pix[i+1])*255/int(a)))
			out.Pix[i+2] = uint8(min(255, int(img.Pix[i+2])*255/int(a)))
			out.Pix[i+3] = a
		}
	}
	return out
}

// present uploads the composition and shows it on a layer surface anchored at
// the output's top-left, so screen coordinates equal canvas coordinates.
func present(img *image.RGBA, transparent bool, namespace string, duration time.Duration) {
	var (
		texture gpucontext.Texture
		drawer  gpucontext.TextureDrawer
	)

	cfg := gogpu.DefaultConfig().
		WithTitle("keyshell S4 transparency").
		WithSize(width, height).
		WithTransparent(transparent).
		WithLayerShell(gogpu.LayerShellConfig{
			Namespace:     namespace,
			Layer:         gogpu.LayerShellTop,
			Anchor:        gogpu.LayerShellAnchorLeft | gogpu.LayerShellAnchorTop,
			Width:         width,
			Height:        height,
			Keyboard:      gogpu.LayerShellKeyboardNone,
			ExclusiveZone: -1,
		})

	app := gogpu.NewApp(cfg)

	app.OnDraw(func(dc *gogpu.Context) {
		// Clear to a colour with zero alpha: anything not drawn must be
		// invisible, not black. This is the other half of the question.
		dc.ClearColor(gmath.Color{R: 0, G: 0, B: 0, A: 0})
		if texture == nil {
			drawer = dc.AsTextureDrawer()
			tex, err := drawer.TextureCreator().NewTextureFromRGBA(width, height, img.Pix)
			if err != nil {
				log.Println("texture upload failed:", err)
				return
			}
			texture = tex
		}
		if err := drawer.DrawTexture(texture, 0, 0); err != nil {
			log.Println("draw texture failed:", err)
		}
	})

	go func() {
		time.Sleep(duration)
		app.Quit()
	}()

	fmt.Printf("presenting %dx%d at the output's top-left, namespace %s, transparent=%v, for %s\n",
		width, height, namespace, transparent, duration)
	if err := app.Run(); err != nil {
		log.Println("presentation failed:", err)
	}
}
