package render

import (
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/rank"
	"github.com/darkliquid/keyshell/internal/theme"
	"github.com/gogpu/gg"
)

func benchView() *fakeView {
	results := make([]rank.Match, 0, 60)
	titles := []string{"Ghostty", "Firefox", "Speakers", "Wired connection 1", "Cool Song", "Calculator", "System Monitor", "Files"}
	for i := 0; i < 60; i++ {
		title := titles[i%len(titles)]
		results = append(results, rank.Match{Item: backend.Item{
			Title: title, Subtitle: "a subtitle for the card", Icon: "application-x-executable",
			Badge: "APP", Status: "OPEN",
			Controls: []backend.Control{{ID: "volume", Kind: backend.ControlRange, Value: 0.5, Min: 0, Max: 1, Primary: true}},
			Actions:  []backend.Action{{Type: "exec", Value: title}},
		}})
	}
	return &fakeView{open: true, results: results}
}

// BenchmarkDrawFrame measures composition alone, without the presenter's copy,
// mask and alpha conversion.
func BenchmarkDrawFrame(b *testing.B) {
	th, err := theme.Load()
	if err != nil {
		b.Fatal(err)
	}
	defer th.Close()
	view := benchView()
	dc := gg.NewContext(3840, 1080)
	r := New()
	r.wasOpen = true
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.DrawFrame(dc, view, th, config.Default().Reveal)
	}
}

// BenchmarkFramePipeline measures what the presenter does per frame: compose
// into a buffer we own, then mask it to the band in place.
func BenchmarkFramePipeline(b *testing.B) {
	th, err := theme.Load()
	if err != nil {
		b.Fatal(err)
	}
	defer th.Close()
	view := benchView()
	premul := make([]byte, 3840*1080*4)
	pm := gg.NewPixmapFromBuffer(premul, 3840, 1080)
	dc := gg.NewContextForPixmap(pm)
	r := New()
	r.wasOpen = true
	img := pm.ImageView()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.DrawFrame(dc, view, th, config.Default().Reveal)
		r.MaskFrame(img)
	}
}
