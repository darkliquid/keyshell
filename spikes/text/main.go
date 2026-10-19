//go:build linux

// SPIKE S3 (throwaway): does gg render the display typography well enough?
//
// keyshell's look depends on one face used hard: Red Hat Display Black, italic,
// all caps, no hinting, drawn with a solid offset shadow, and often sheared with
// an affine transform. Qt does that today with Text.QtRendering, which is the
// quality bar. This spike draws the same things with gg and writes a PNG so the
// two can be compared by eye, and can optionally present the result on a layer
// surface to exercise the full path (gg -> texture -> layer surface -> screen).
//
// Run:
//
//	mise run spike:s3                     # writes out/s3-typography.png
//	go run ./spikes/text -present         # also shows it on screen for 20s
//
// The face is loaded from the system font directory; override with -face.
package main

import (
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
	canvasWidth  = 1600
	canvasHeight = 900
)

// The theme's palette, so the comparison reads in the right colours.
var (
	ink     = hex("#F2F2F7")
	slab    = hex("#0B0B0F")
	surface = hex("#121218")
	accent  = hex("#FF2E63")
	cyan    = hex("#08F7FE")
	muted   = hex("#5A5A66")
)

var defaultFace = "/usr/share/fonts/redhat/RedHatDisplay-BlackItalic.otf"

func main() {
	out := flag.String("out", "out/s3-typography.png", "where to write the rendered PNG")
	facePath := flag.String("face", defaultFace, "path to a display face")
	present := flag.Bool("present", false, "also present the render on a layer surface")
	duration := flag.Duration("duration", 20*time.Second, "how long to keep the presented surface up")
	flag.Parse()

	if _, err := os.Stat(*facePath); err != nil {
		log.Fatalf("face not found: %s (override with -face)", *facePath)
	}

	img, err := render(*facePath)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := gg.NewContextForImage(img).SavePNG(*out); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s (%dx%d), face %s\n", *out, canvasWidth, canvasHeight, filepath.Base(*facePath))

	if *present {
		presentOnLayerSurface(img, *duration)
	}
}

// render draws the typography samples that matter to this design.
func render(facePath string) (*image.RGBA, error) {
	source, err := text.NewFontSourceFromFile(facePath)
	if err != nil {
		return nil, fmt.Errorf("load face: %w", err)
	}
	defer func() { _ = source.Close() }()

	dc := gg.NewContext(canvasWidth, canvasHeight)
	dc.ClearWithColor(slab)

	// 1. Display face at title size with the theme's hard offset shadow: a
	//    solid copy, no blur, exactly as the design specifies.
	titleFace := source.Face(88)
	dc.SetFont(titleFace)
	dc.SetColor(cyan)
	dc.DrawString("KEYSHELL", 64+10, 150+10)
	dc.SetColor(ink)
	dc.DrawString("KEYSHELL", 64, 150)

	// 2. The banner lean, applied as a transform rather than by hand.
	dc.Push()
	dc.Translate(64, 260)
	dc.Shear(-0.18, 0)
	dc.SetFont(titleFace)
	dc.SetColor(accent)
	dc.DrawString("SHEARED BANNER", 0, 0)
	dc.Pop()

	// 3. Mode badges at the size the reel shows them, with the 3px border and
	//    the accent outline on the selected one.
	badgeFace := source.Face(44)
	dc.SetFont(badgeFace)
	x := 64.0
	for i, label := range []string{"APPS", "WINDOWS", "FILES", "AUDIO"} {
		boxW := badgeFace.Advance(label) + 28
		boxX, boxY, boxH := x-14, 330.0, 62.0
		dc.SetColor(surface)
		dc.DrawRectangle(boxX, boxY, boxW, boxH)
		if err := dc.Fill(); err != nil {
			return nil, err
		}
		if i == 0 {
			dc.SetColor(accent)
			dc.SetLineWidth(3)
			dc.DrawRectangle(boxX, boxY, boxW, boxH)
			if err := dc.Stroke(); err != nil {
				return nil, err
			}
			dc.SetColor(ink)
		} else {
			dc.SetColor(muted)
		}
		dc.DrawString(label, x, 375)
		x += boxW + 18
	}

	// 4. Card titles and subtitles at reel size.
	cardFace := source.Face(30)
	subFace := source.Face(18)
	dc.SetFont(cardFace)
	dc.SetColor(ink)
	dc.DrawString("GHOSTTY", 64, 470)
	dc.SetFont(subFace)
	dc.SetColor(muted)
	dc.DrawString("Terminal", 64, 498)

	dc.SetFont(cardFace)
	dc.SetColor(ink)
	dc.DrawString("FIREFOX", 470, 470)
	dc.SetFont(subFace)
	dc.SetColor(muted)
	dc.DrawString("Web Browser", 470, 498)

	// 5. The small sizes the UI leans on: key hints and metadata. This is
	//    where rasterisation quality shows.
	smallSpecs := []struct {
		size float64
		text string
	}{
		{18, "MOD+SPACE  TOGGLE"},
		{14, "Enter to launch - Escape to dismiss"},
		{11, "claude-code  ~/work/keyshell  2 hours ago"},
	}
	y := 580.0
	for _, spec := range smallSpecs {
		dc.SetFont(source.Face(spec.size))
		dc.SetColor(ink)
		dc.DrawString(spec.text, 64, y)
		dc.SetColor(muted)
		dc.DrawString(fmt.Sprintf("%.0fpx", spec.size), 1000, y)
		y += spec.size + 22
	}

	// 6. Text as paths: a sheared fill (what a card needs if no per-widget
	//    transform exists) and a stroked outline.
	pathFace := source.Face(56)
	dc.Push()
	dc.Translate(64, 720)
	dc.Shear(-0.22, 0)
	dc.SetFont(pathFace)
	dc.SetColor(cyan)
	if err := dc.FillPath(dc.TextPath("PATH SHEAR", 0, 0)); err != nil {
		return nil, err
	}
	dc.Pop()

	dc.SetFont(pathFace)
	dc.SetColor(accent)
	dc.SetLineWidth(2)
	if err := dc.StrokePath(dc.TextPath("OUTLINE", 700, 720)); err != nil {
		return nil, err
	}

	rgba, ok := dc.Image().(*image.RGBA)
	if !ok {
		converted := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
		draw.Draw(converted, converted.Bounds(), dc.Image(), image.Point{}, draw.Src)
		return converted, nil
	}
	return rgba, nil
}

// presentOnLayerSurface uploads the render and shows it on a layer surface so
// the composited result can be screenshotted. This is the path a real overlay
// would use: gg draws, the texture is uploaded, gogpu presents it.
func presentOnLayerSurface(img *image.RGBA, duration time.Duration) {
	const (
		width  = 800
		height = 450
	)
	// Nearest-neighbour downscale: the spike only needs the content on screen,
	// not a correct resampler.
	scaled := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			scaled.Set(x, y, img.At(x*canvasWidth/width, y*canvasHeight/height))
		}
	}

	var (
		texture gpucontext.Texture
		drawer  gpucontext.TextureDrawer
	)

	app := gogpu.NewApp(gogpu.DefaultConfig().
		WithTitle("keyshell S3 typography").
		WithSize(width, height).
		WithLayerShell(gogpu.LayerShellConfig{
			Namespace:     "keyshell-spike-s3",
			Layer:         gogpu.LayerShellTop,
			Anchor:        gogpu.LayerShellAnchorLeft | gogpu.LayerShellAnchorBottom,
			Width:         width,
			Height:        height,
			Keyboard:      gogpu.LayerShellKeyboardNone,
			ExclusiveZone: -1,
		}))

	app.OnDraw(func(dc *gogpu.Context) {
		dc.ClearColor(gmath.Color{R: 0.04, G: 0.04, B: 0.06, A: 1})
		if texture == nil {
			drawer = dc.AsTextureDrawer()
			tex, err := drawer.TextureCreator().NewTextureFromRGBA(width, height, scaled.Pix)
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

	fmt.Printf("presenting on a layer surface (%dx%d, namespace keyshell-spike-s3) for %s\n", width, height, duration)
	if err := app.Run(); err != nil {
		log.Println("presentation failed:", err)
	}
}

// hex parses "#RRGGBB" into a gg colour.
func hex(s string) gg.RGBA {
	var r, g, b uint8
	if _, err := fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b); err != nil {
		log.Fatalf("bad hex colour %q: %v", s, err)
	}
	return gg.RGBA{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: 1}
}
