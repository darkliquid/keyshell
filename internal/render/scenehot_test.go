package render

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/ksl"
)

// TestSceneWatcherReloadsOnChange writes a scene, starts the watcher and
// edits the file, expecting a re-parsed scene on the callback.
func TestSceneWatcherReloadsOnChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui.ksl")
	if err := os.WriteFile(path, []byte("Window { Box { } }"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.UI.Scene = path

	var mu sync.Mutex
	loaded := 0
	w := NewSceneWatcher(path, func() (*ksl.Scene, error) { return LoadScene(cfg) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Watch(ctx, func(*ksl.Scene) {
		mu.Lock()
		loaded++
		mu.Unlock()
	}, DefaultOnError)

	// Wait past the initial signature, then edit.
	time.Sleep(watchInterval + 100*time.Millisecond)
	mu.Lock()
	if loaded != 0 {
		t.Fatalf("reloads before edit = %d, want 0", loaded)
	}
	mu.Unlock()
	if err := os.WriteFile(path, []byte("Window {\n Box { }\n Box { }\n}"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := loaded
		mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("scene edit did not trigger a reload")
}

// TestSceneWatcherReportsParseErrors edits the scene into something broken
// and expects the error callback, not a silent keep.
func TestSceneWatcherReportsParseErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui.ksl")
	if err := os.WriteFile(path, []byte("Window { Box { } }"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.UI.Scene = path

	errs := make(chan error, 1)
	w := NewSceneWatcher(path, func() (*ksl.Scene, error) { return LoadScene(cfg) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Watch(ctx, func(*ksl.Scene) {
		t.Error("broken scene must not reach onChange")
	}, func(err error) { errs <- err })

	time.Sleep(watchInterval + 100*time.Millisecond)
	if err := os.WriteFile(path, []byte("Window { Box {"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errs:
		if err == nil {
			t.Fatal("error callback received a nil error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("broken scene did not trigger the error callback")
	}
}

// TestSetSceneSwapsFrames proves a hot scene swap is visible to the next
// frame without rebuilding the renderer.
func TestSetSceneSwapsFrames(t *testing.T) {
	r := NewConfigured(config.Default())
	before := r.scene.scene
	r.SetScene(mustParseScene(t, "Window {\n Box { }\n Box { }\n}"))
	after, _ := r.scene.snapshot()
	if after == before {
		t.Fatal("SetScene left the scene pointer unchanged")
	}
	if len(after.Nodes()[0].Children) != 2 {
		t.Fatalf("swapped scene nodes = %d children, want 2", len(after.Nodes()[0].Children))
	}
}
