package sdk

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/darkliquid/keyshell/internal/plugin/proto"
)

// echoBackend answers every query with one item.
type echoBackend struct {
	gotControls []Change
}

func (e *echoBackend) Query(_ context.Context, q Query) (Stream, error) {
	return NewStream(Item{
		ID: "echo", Title: q.Text, Subtitle: "echo",
		Actions: []Action{{Type: "copy", Value: q.Text}},
	}), nil
}

func (e *echoBackend) Control(_ context.Context, c Change) error {
	e.gotControls = append(e.gotControls, c)
	return nil
}

// runServe feeds scripted stdin and returns captured stdout.
func runServe(t *testing.T, input string, backend Backend) string {
	t.Helper()
	t.Setenv("KEYSHELL_PLUGIN", "1")
	t.Setenv("KEYSHELL_PROTOCOL", "1")
	t.Setenv("KEYSHELL_INSTANCE", "test")

	old := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	defer func() { os.Stdin = old }()

	var out bytes.Buffer
	stdout := os.Stdout
	or, ow, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = ow

	done := make(chan error, 1)
	go func() {
		done <- Serve(Info{ID: "test", Name: "Test", Controls: []string{"copy"}}, backend)
	}()

	go func() {
		_, _ = w.WriteString(input)
		_ = w.Close()
	}()

	// Copy stdout pipe into out while Serve writes.
	go func() {
		_, _ = out.ReadFrom(or)
	}()

	<-done
	_ = ow.Close()
	_ = or.Close()
	os.Stdout = stdout
	return out.String()
}

func TestServeHandshakeAndQuery(t *testing.T) {
	backend := &echoBackend{}
	input := "{\"init\":{\"instance\":\"test\",\"config\":{}}}\n" +
		"{\"query\":{\"id\":1,\"text\":\"hi\",\"limit\":10,\"merged\":false}}\n"
	out := runServe(t, input, backend)

	var lines []proto.Message
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		m, err := proto.Decode(sc.Bytes())
		if err != nil {
			t.Fatalf("decode line: %v", err)
		}
		lines = append(lines, m)
	}

	// hello first
	if len(lines) < 1 || lines[0].Hello == nil {
		t.Fatal("first line must be hello")
	}
	// ready after init
	if len(lines) < 2 || lines[1].Ready == nil {
		t.Fatal("second line must be ready")
	}
	// item then done
	if len(lines) < 4 || lines[2].Item == nil || lines[3].Done == nil {
		t.Fatalf("expected item then done, got %d lines", len(lines))
	}
	if lines[2].Item.Title != "hi" {
		t.Errorf("item title = %q, want hi", lines[2].Item.Title)
	}
	if lines[3].Done.ID != 1 {
		t.Errorf("done id = %d, want 1", lines[3].Done.ID)
	}
}

func TestServeHandlesControl(t *testing.T) {
	backend := &echoBackend{}
	input := "{\"init\":{\"instance\":\"test\"}}\n" +
		"{\"control\":{\"item\":\"echo\",\"control\":\"volume\",\"value\":0.4}}\n"
	runServe(t, input, backend)
	if len(backend.gotControls) != 1 || backend.gotControls[0].Value != 0.4 {
		t.Errorf("controls = %+v, want one volume 0.4", backend.gotControls)
	}
}

// TestServeRejectsMismatchedProtocol verifies the protocol guard.
func TestServeRejectsMismatchedProtocol(t *testing.T) {
	t.Setenv("KEYSHELL_PLUGIN", "1")
	t.Setenv("KEYSHELL_PROTOCOL", "99")
	err := Serve(Info{ID: "x"}, &echoBackend{})
	if err == nil {
		t.Fatal("Serve() with wrong protocol = nil error, want error")
	}
}
