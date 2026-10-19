// Package files provides the files backend: it shells out to dsearch and parses
// its JSON result lines into items whose primary action opens the file.
package files

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/darkliquid/keyshell/internal/backend"
	"github.com/darkliquid/keyshell/internal/config"
)

// Backend runs dsearch and parses its output. The dsearch command is injectable
// so tests can point it at a stub.
type Backend struct {
	command func(args ...string) *exec.Cmd
}

// New builds the backend with the real dsearch binary.
func New(_ config.Config) (backend.Backend, error) {
	return &Backend{command: func(args ...string) *exec.Cmd {
		return exec.Command("dsearch", args...)
	}}, nil
}

// Info implements backend.Backend.
func (b *Backend) Info() backend.Info {
	return backend.Info{
		ID:         "files",
		Name:       "Files",
		Badge:      "FILES",
		Trigger:    "/",
		Searchable: true,
		Trackable:  true,
		StableIDs:  true, // full paths are stable
	}
}

// Query runs dsearch for the query text and streams parsed items.
func (b *Backend) Query(ctx context.Context, q backend.Query) (backend.Stream, error) {
	args := []string{"search", "--json", "--limit", fmt.Sprintf("%d", q.Limit)}
	if q.Text != "" {
		args = append(args, "--fuzzy", "--", q.Text)
	} else {
		args = append(args, "--sort", "mtime", "--", "*")
	}
	cmd := b.command(args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("dsearch stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("dsearch start: %w", err)
	}
	return &dsearchStream{cmd: cmd, r: bufio.NewReader(stdout)}, nil
}

func (b *Backend) Control(context.Context, backend.Change) error { return nil }
func (b *Backend) Close() error                                  { return nil }

type dsearchStream struct {
	cmd *exec.Cmd
	r   *bufio.Reader
	run bool
}

func (s *dsearchStream) Next() (backend.Item, bool, error) {
	for {
		line, err := s.r.ReadString('\n')
		if err != nil && err != io.EOF {
			_ = s.cmd.Process.Kill()
			return backend.Item{}, false, err
		}
		if line == "" && err == io.EOF {
			return backend.Item{}, false, s.finish()
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if err == io.EOF {
				return backend.Item{}, false, s.finish()
			}
			continue
		}
		var raw struct {
			Path string `json:"path"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			if err == io.EOF {
				return backend.Item{}, false, s.finish()
			}
			continue
		}
		if raw.Path == "" {
			continue
		}
		return backend.Item{
			ID:       backend.ResultID(raw.Path),
			Title:    raw.Path,
			Subtitle: raw.Type,
			Glyph:    "file",
			Actions:  []backend.Action{{Type: "open-url", Value: "file://" + raw.Path}},
		}, true, nil
	}
}

// finish waits for the process to exit. Calling it when the stream is already
// finished is harmless.
func (s *dsearchStream) finish() error {
	if !s.run {
		s.run = true
		return s.cmd.Wait()
	}
	return nil
}

func (s *dsearchStream) Close() error {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return s.cmd.Wait()
}
