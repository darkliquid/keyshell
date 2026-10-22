// Package logging configures the process-wide structured logger. The CLI
// installs a slog handler early, so every package can log through the slog
// default logger with consistent levels and formatting.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Options describes the logger configuration.
type Options struct {
	// Level is one of debug, info, warn, error. Unknown values fall back
	// to info.
	Level string
	// Format is text (default) or json.
	Format string
}

// Setup installs the configured logger as the slog default and returns it.
// Output goes to stderr, so stdout stays usable for command output.
func Setup(opts Options) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(opts.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	hopts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if strings.EqualFold(opts.Format, "json") {
		h = slog.NewJSONHandler(os.Stderr, hopts)
	} else {
		h = slog.NewTextHandler(os.Stderr, hopts)
	}
	lg := slog.New(h)
	slog.SetDefault(lg)
	return lg
}

// DefaultOptions resolves the log options from the environment, so spawned
// subprocesses inherit the host's logging posture. Flags override these.
func DefaultOptions() Options {
	return Options{
		Level:  os.Getenv("KEYSHELL_LOG_LEVEL"),
		Format: os.Getenv("KEYSHELL_LOG_FORMAT"),
	}
}
