package ipc

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/darkliquid/keyshell/internal/rank"
)

// recordingBackend captures the controls it receives.
type recordingBackend struct {
	source   string
	controls []backend.Change
}

func (r *recordingBackend) Info() backend.Info {
	return backend.Info{ID: r.source}
}
func (r *recordingBackend) Query(_ context.Context, _ backend.Query) (backend.Stream, error) {
	return &nilStream{}, nil
}
func (r *recordingBackend) Control(_ context.Context, c backend.Change) error {
	r.controls = append(r.controls, c)
	return nil
}
func (r *recordingBackend) Close() error { return nil }

type nilStream struct{}

func (n *nilStream) Next() (backend.Item, bool, error) { return backend.Item{}, false, nil }
func (n *nilStream) Close() error                      { return nil }

// fakeProvider serves named recording backends.
type fakeProvider struct {
	backends map[string]*recordingBackend
}

func (p *fakeProvider) Get(id string) (backend.Backend, error) {
	if b, ok := p.backends[id]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("unknown backend %q", id)
}

func TestControlVerbsRouteToBackends(t *testing.T) {
	audio := &recordingBackend{source: "audio"}
	media := &recordingBackend{source: "media"}
	prov := &fakeProvider{backends: map[string]*recordingBackend{"audio": audio, "media": media}}
	l := launcher.New(prov, launcher.Options{RankOpts: rank.Options{}})
	h := New(l)

	h.dispatch("volumeUp")
	h.dispatch("toggleMute")
	h.dispatch("playPause")
	h.dispatch("nextTrack")

	if len(audio.controls) != 2 || audio.controls[0].Control != "volume-delta" || audio.controls[0].Value != 0.05 {
		t.Errorf("audio controls = %+v, want volume-delta then mute", audio.controls)
	}
	if audio.controls[1].Control != "mute" {
		t.Errorf("audio control 2 = %+v, want mute", audio.controls[1])
	}
	if len(media.controls) != 2 || media.controls[0].Text != "play-pause" || media.controls[1].Text != "next" {
		t.Errorf("media controls = %+v, want play-pause then next", media.controls)
	}
}

func TestVerbDispatch(t *testing.T) {
	audio := &recordingBackend{source: "audio"}
	media := &recordingBackend{source: "media"}
	prov := &fakeProvider{backends: map[string]*recordingBackend{"audio": audio, "media": media}}
	l := launcher.New(prov, launcher.Options{RankOpts: rank.Options{}})
	h := New(l)

	dir := t.TempDir()
	sock := filepath.Join(dir, "test.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go h.Serve(ln)

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("toggle\n")); err != nil {
		t.Fatal(err)
	}
	// Wait for the handler goroutine to dispatch.
	for i := 0; i < 50 && !l.IsOpen(); i++ {
		time.Sleep(2 * time.Millisecond)
	}
	if !l.IsOpen() {
		t.Error("launcher did not open after toggle verb")
	}
}
