package render

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/darkliquid/keyshell/internal/ksl"
	"github.com/darkliquid/keyshell/internal/telemetry"
)

// watchInterval is how often the scene files are polled. A filesystem
// watcher library is not worth the dependency for a theme directory.
const watchInterval = 250 * time.Millisecond

// SceneWatcher re-parses the configured scene when its file or any include
// in its directory changes on disk, so a theme can be edited live. The load
// function is the same one that produced the initial scene, so reloads go
// through identical parsing and validation.
type SceneWatcher struct {
	// path is the scene file. An empty path watches nothing: the embedded
	// default scene cannot change.
	path string
	// load re-reads and re-parses the scene from disk.
	load func() (*ksl.Scene, error)
}

// NewSceneWatcher builds a watcher for the scene at path.
func NewSceneWatcher(path string, load func() (*ksl.Scene, error)) *SceneWatcher {
	return &SceneWatcher{path: path, load: load}
}

// Watch polls the scene files until ctx is done, calling onChange with every
// successfully parsed scene and onError with every failure. It must not run
// on the frame thread: onChange hands the scene to the renderer, which only
// swaps a pointer.
func (w *SceneWatcher) Watch(ctx context.Context, onChange func(*ksl.Scene), onError func(error)) {
	if w.path == "" || w.load == nil {
		return
	}
	dir := filepath.Dir(w.path)
	sig := w.signature(dir)
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		next := w.signature(dir)
		if next == sig {
			continue
		}
		sig = next
		s, err := w.load()
		ctx, span := telemetry.StartSpan(ctx, "keyshell.ksl.reload",
			trace.WithAttributes(attribute.String("keyshell.scene", w.path)))
		if err != nil {
			telemetry.SceneReloads().Add(ctx, 1,
				metric.WithAttributes(attribute.String(telemetry.Attrs.Outcome, "error")))
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
			onError(err)
			continue
		}
		telemetry.SceneReloads().Add(ctx, 1,
			metric.WithAttributes(attribute.String(telemetry.Attrs.Outcome, "ok")))
		span.End()
		onChange(s)
	}
}

// signature summarises the scene file and every .ksl include in its
// directory, so an edit to an included theme part reloads too.
func (w *SceneWatcher) signature(dir string) string {
	sig := ""
	entries, err := os.ReadDir(dir)
	if err != nil {
		return sig
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".ksl" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		sig += e.Name() + "|" +
			info.ModTime().String() + "|" +
			strconv.FormatInt(info.Size(), 10) + ";"
	}
	return sig
}

// DefaultOnError logs a failed reload and keeps the previous scene, which
// is the right behaviour for a live edit: a half-written theme file must
// never take the overlay down.
func DefaultOnError(err error) {
	slog.Warn("ksl: scene reload failed; keeping previous scene", "err", err)
}
