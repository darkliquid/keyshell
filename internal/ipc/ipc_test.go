package ipc

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/launcher"
	"github.com/darkliquid/keyshell/internal/rank"
)

// fakeProvider satisfies backend.BackendProvider minimally for the launcher.
type fakeProvider struct{}

func (fakeProvider) Get(id string) (backend.Backend, error) {
	return nil, fmt.Errorf("not used in ipc test")
}

func TestVerbDispatch(t *testing.T) {
	l := launcher.New(fakeProvider{}, launcher.Options{RankOpts: rank.Options{}})
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
