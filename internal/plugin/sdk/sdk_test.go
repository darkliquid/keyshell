package sdk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type configurableBackend struct {
	echoBackend
	gotInstance     string
	gotConfig       []byte
	configureErr    error
	configureCalled bool
}

func (c *configurableBackend) Configure(instance string, config []byte) error {
	c.configureCalled = true
	c.gotInstance = instance
	c.gotConfig = config
	return c.configureErr
}

// runServe feeds scripted stdin and returns captured stdout.
func runServe(t *testing.T, input string, backend Backend) string {
	t.Helper()
	out, err := runServeWithInfo(t, Info{ID: "test", Name: "Test", Controls: []string{"copy"}}, backend, input)
	if err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
	return out
}

func runServeWithInfo(t *testing.T, info Info, backend Backend, input string) (string, error) {
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
	defer r.Close()
	defer w.Close()

	var out bytes.Buffer
	stdout := os.Stdout
	or, ow, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = ow
	defer func() { os.Stdout = stdout }()
	defer or.Close()
	defer ow.Close()

	done := make(chan error, 1)
	go func() {
		done <- Serve(info, backend)
	}()

	go func() {
		_, _ = w.WriteString(input)
		_ = w.Close()
	}()

	readDone := make(chan struct{})
	// Copy stdout pipe into out while Serve writes.
	go func() {
		_, _ = out.ReadFrom(or)
		close(readDone)
	}()

	serveErr := <-done
	_ = ow.Close()
	<-readDone
	return out.String(), serveErr
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

func TestConfigurableDelivery(t *testing.T) {
	backend := &configurableBackend{}
	input := "{\"init\":{\"instance\":\"custom-inst\",\"config\":{\"key\":\"value\"}}}\n"
	_ = runServe(t, input, backend)

	if !backend.configureCalled {
		t.Fatal("expected Configure to be called")
	}
	if backend.gotInstance != "custom-inst" {
		t.Errorf("instance = %q, want custom-inst", backend.gotInstance)
	}
	wantConfig := `{"key":"value"}`
	if string(backend.gotConfig) != wantConfig {
		t.Errorf("config = %s, want %s", string(backend.gotConfig), wantConfig)
	}
}

func TestConfigurableNotCalledWhenConfigOmittedOrNull(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "no config field",
			input: "{\"init\":{\"instance\":\"custom-inst\"}}\n",
		},
		{
			name:  "config is null",
			input: "{\"init\":{\"instance\":\"custom-inst\",\"config\":null}}\n",
		},
		{
			name:  "config is whitespace null",
			input: "{\"init\":{\"instance\":\"custom-inst\",\"config\": null }}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &configurableBackend{}
			_ = runServe(t, tt.input, backend)
			if backend.configureCalled {
				t.Errorf("Configure was called unexpectedly with config: %q", string(backend.gotConfig))
			}
		})
	}
}
func TestConfigurableError(t *testing.T) {
	backend := &configurableBackend{
		configureErr: errors.New("boom"),
	}
	input := "{\"init\":{\"instance\":\"test\",\"config\":{\"invalid\":true}}}\n"
	_, err := runServeWithInfo(t, Info{ID: "test"}, backend, input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	wantErr := "sdk: configure: boom"
	if err.Error() != wantErr {
		t.Errorf("err = %q, want %q", err.Error(), wantErr)
	}
}

func TestSearchablePropagation(t *testing.T) {
	tests := []struct {
		name       string
		searchable bool
	}{
		{name: "searchable true", searchable: true},
		{name: "searchable false", searchable: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &echoBackend{}
			input := "{\"init\":{\"instance\":\"test\",\"config\":{}}}\n"
			out, err := runServeWithInfo(t, Info{ID: "test", Searchable: tt.searchable}, backend, input)
			if err != nil {
				t.Fatalf("Serve error: %v", err)
			}
			sc := bufio.NewScanner(strings.NewReader(out))
			if !sc.Scan() {
				t.Fatal("expected hello message")
			}
			m, err := proto.Decode(sc.Bytes())
			if err != nil {
				t.Fatalf("decode hello: %v", err)
			}
			if m.Hello == nil {
				t.Fatal("first message was not hello")
			}
			if m.Hello.Searchable == nil {
				t.Fatal("expected Hello.Searchable to be set, got nil")
			}
			if *m.Hello.Searchable != tt.searchable {
				t.Errorf("Hello.Searchable = %v, want %v", *m.Hello.Searchable, tt.searchable)
			}
		})
	}
}
func TestInfoFlag(t *testing.T) {
	tests := []struct {
		name       string
		searchable bool
	}{
		{name: "searchable true", searchable: true},
		{name: "searchable false", searchable: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			os.Args = []string{"keyshell-plugin", "--info"}
			defer func() { os.Args = oldArgs }()

			stdout := os.Stdout
			or, ow, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdout = ow
			defer func() { os.Stdout = stdout }()
			defer or.Close()
			defer ow.Close()

			info := Info{
				ID:         "plug-id",
				Name:       "Plug Name",
				Badge:      "PLUG",
				Trigger:    ":p",
				Controls:   []string{"copy"},
				Hot:        true,
				StableIDs:  true,
				Searchable: tt.searchable,
			}

			serveErr := Serve(info, &echoBackend{})
			_ = ow.Close()

			if serveErr != nil {
				t.Fatalf("Serve returned error: %v", serveErr)
			}

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(or)

			var hello proto.Hello
			if err := json.Unmarshal(buf.Bytes(), &hello); err != nil {
				t.Fatalf("failed to unmarshal --info output %q: %v", buf.String(), err)
			}

			if hello.Protocol != proto.Protocol {
				t.Errorf("Protocol = %d, want %d", hello.Protocol, proto.Protocol)
			}
			if hello.ID != "plug-id" {
				t.Errorf("ID = %q, want plug-id", hello.ID)
			}
			if hello.Name != "Plug Name" {
				t.Errorf("Name = %q, want Plug Name", hello.Name)
			}
			if hello.Badge != "PLUG" {
				t.Errorf("Badge = %q, want PLUG", hello.Badge)
			}
			if hello.Trigger != ":p" {
				t.Errorf("Trigger = %q, want :p", hello.Trigger)
			}
			if len(hello.Controls) != 1 || hello.Controls[0] != "copy" {
				t.Errorf("Controls = %+v, want [copy]", hello.Controls)
			}
			if !hello.Hot {
				t.Errorf("Hot = false, want true")
			}
			if !hello.StableIDs {
				t.Errorf("StableIDs = false, want true")
			}
			if hello.Searchable == nil || *hello.Searchable != tt.searchable {
				t.Errorf("Searchable = %v, want %v", hello.Searchable, tt.searchable)
			}
		})
	}
}
