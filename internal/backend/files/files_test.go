package files

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/keyshell/internal/backend"
)

// stubDsearch writes a shell script that echoes fixed dsearch-shaped JSON, one
// object per line, and returns its path.
func stubDsearch(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "dsearch-stub")
	content := "#!/bin/sh\n" + script
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestQueryParsesDsearchLines proves a stub dsearch's JSON lines become items.
func TestQueryParsesDsearchLines(t *testing.T) {
	stub := stubDsearch(t, `echo '{"path":"/home/u/a.txt","type":"file"}'
echo '{"path":"/home/u/b.txt","type":"file"}'
`)
	b := &Backend{command: func(args ...string) *exec.Cmd {
		return exec.Command(stub, args...)
	}}

	stream, err := b.Query(context.Background(), backend.Query{Text: "a", Limit: 10})
	if err != nil {
		t.Fatalf("Query = err %v, want nil", err)
	}
	defer stream.Close()

	got := []string{}
	for {
		it, more, err := stream.Next()
		if err != nil {
			t.Fatalf("Next = err %v, want nil", err)
		}
		if !more {
			break
		}
		got = append(got, string(it.ID))
	}
	if len(got) != 2 || got[0] != "/home/u/a.txt" || got[1] != "/home/u/b.txt" {
		t.Errorf("ids = %v, want [a.txt b.txt]", got)
	}
}

// TestQueryToleratesGarbage validates that malformed lines are skipped without
// aborting the stream.
func TestQueryToleratesGarbage(t *testing.T) {
	stub := stubDsearch(t, `echo 'not json'
echo '{"path":"/ok.txt"}'
`)
	b := &Backend{command: func(args ...string) *exec.Cmd {
		return exec.Command(stub, args...)
	}}
	stream, _ := b.Query(context.Background(), backend.Query{Text: "", Limit: 10})
	defer stream.Close()

	got := []string{}
	for {
		it, more, err := stream.Next()
		if err != nil {
			t.Fatalf("Next = err %v, want nil", err)
		}
		if !more {
			break
		}
		got = append(got, string(it.ID))
	}
	if len(got) != 1 || got[0] != "/ok.txt" {
		t.Errorf("ids = %v, want only [/ok.txt]", got)
	}
}

// TestQueryEmptyTextUsesSortFlag checks the no-text path builds the right arg
// set by exercising the stub with a marker in the arg list.
func TestQueryEmptyTextUsesSortFlag(t *testing.T) {
	stub := stubDsearch(t, `echo '{"path":"/mtime.txt"}'
`)
	var args []string
	b := &Backend{command: func(a ...string) *exec.Cmd {
		args = a
		return exec.Command(stub, a...)
	}}
	stream, err := b.Query(context.Background(), backend.Query{Text: "", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	stream.Close()

	joined := ""
	for _, a := range args {
		joined += a + " "
	}
	if !contains(joined, "--sort") {
		t.Errorf("args = %q, want --sort for empty query", joined)
	}
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && strings.Contains(hay, needle)
}
