// Package config defines keyshell's host configuration. Configuration is TOML,
// optional, and layered over defaults: a missing file loads fine, and any value
// a user writes overrides the corresponding default.
package config

import (
	"errors"
	"os"

	"github.com/BurntSushi/toml"
)

// Reveal holds the resampling bounds for the band's reveal effect. The values
// are the usable ranges the overlay randomises within on each summon.
type Reveal struct {
	MinTilt      float64
	MaxTilt      float64
	MinShear     float64
	MaxShear     float64
	MinEndHeight float64
	MaxEndHeight float64
}

// Anim holds the animation durations in milliseconds. keyshell's feel is short
// and mechanical; these are the knobs for it.
type Anim struct {
	ExpandMs int `toml:"expand_ms"`
	ScrollMs int `toml:"scroll_ms"`
}

// Overlay holds result and control timing limits.
type Overlay struct {
	MaxResults int
	SettleMs   int // control change settle window, optimistic echo
	IdleMs     int // idle teardown for lazy backends
}

// Drinks holds the terminal candidates for run-command, in preference order.
type Drinks struct {
	Terminal []string `toml:"terminal"`
}

// Config is the parsed host configuration.
type Config struct {
	Reveal  Reveal
	Anim    Anim
	Overlay Overlay
	Drinks  Drinks
}

// Default returns the built-in configuration. It is the zero base for any
// TOML overlay.
func Default() Config {
	return Config{
		Reveal: Reveal{
			MinTilt: -14, MaxTilt: 14,
			MinShear: -0.22, MaxShear: 0.22,
			MinEndHeight: 40, MaxEndHeight: 90,
		},
		Anim: Anim{
			ExpandMs: 90,
			ScrollMs: 70,
		},
		Overlay: Overlay{
			MaxResults: 60,
			SettleMs:   80,
			IdleMs:     5 * 60 * 1000,
		},
		Drinks: Drinks{
			Terminal: []string{"ghostty", "alacritty"},
		},
	}
}

// Load reads a TOML config file and returns the merged configuration. A missing
// path is not an error: defaults are returned. A malformed file is.
func Load(path string) (Config, error) {
	cfg := Default()

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	if _, err := toml.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
