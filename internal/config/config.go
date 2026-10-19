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

// FrecencyBucket is one age/weight pair in the frecency score.
type FrecencyBucket struct {
	MaxAgeMs int64 `toml:"max_age_ms"`
	Weight   int   `toml:"weight"`
}

// Frecency holds the ranking-history parameters.
type Frecency struct {
	Buckets    []FrecencyBucket `toml:"buckets"`
	CountCap   int              `toml:"count_cap"`
	MaxAgeDays int              `toml:"max_age_days"`
	FlushMs    int              `toml:"flush_debounce_ms"`
}

// Instance is one configured plugin instance.
type Instance struct {
	ID      string   `toml:"id"`
	Command []string `toml:"command"`
	Mode    string   `toml:"mode"` // auto|hot|lazy
	NoReap  bool     `toml:"noreap"`
}

// Config is the parsed host configuration.
type Config struct {
	Reveal    Reveal
	Anim      Anim
	Overlay   Overlay
	Drinks    Drinks
	Frecency  Frecency
	Instances []Instance `toml:"instance"`
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
		Frecency: Frecency{
			Buckets: []FrecencyBucket{
				{MaxAgeMs: 3600000, Weight: 4},
				{MaxAgeMs: 86400000, Weight: 3},
				{MaxAgeMs: 604800000, Weight: 2},
				{MaxAgeMs: 2592000000, Weight: 1},
			},
			CountCap:   10,
			MaxAgeDays: 90,
			FlushMs:    2000,
		},
		// First-party plugins run as subprocesses by default. With an empty
		// command, the host resolves each to `<self> plugin <id>`.
		Instances: []Instance{
			{ID: "audio", Mode: "hot"},
			{ID: "media", Mode: "hot"},
			{ID: "network", Mode: "hot"},
			{ID: "power", Mode: "hot"},
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
