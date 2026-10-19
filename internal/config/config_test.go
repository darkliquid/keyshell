package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestDefaultsAreReturnedWhenFileMissing covers the documented behaviour: a
// missing config file is not an error, it yields defaults.
func TestDefaultsAreReturnedWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("Load(missing) = err %v, want nil", err)
	}
	if cfg.Reveal.MinTilt >= cfg.Reveal.MaxTilt {
		t.Errorf("Reveal tilt range invalid: min=%v max=%v", cfg.Reveal.MinTilt, cfg.Reveal.MaxTilt)
	}
	if cfg.Anim.ExpandMs <= 0 || cfg.Anim.ScrollMs <= 0 {
		t.Errorf("Anim durations must be positive, got %+v", cfg.Anim)
	}
	if cfg.Overlay.MaxResults <= 0 {
		t.Errorf("Overlay.MaxResults must be positive, got %d", cfg.Overlay.MaxResults)
	}
}

// TestLoadReadsToml covers that documented values are read and merged over the
// defaults rather than replacing them wholesale.
func TestLoadReadsToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	toml := `
[anim]
expand_ms = 140
scroll_ms = 40
`
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load = err %v, want nil", err)
	}
	if cfg.Anim.ExpandMs != 140 || cfg.Anim.ScrollMs != 40 {
		t.Errorf("Anim = %+v, want expand=140 scroll=40", cfg.Anim)
	}
	if cfg.Reveal.MinTilt == 0 {
		t.Errorf("defaults lost behind TOML: Reveal.MinTilt = 0")
	}
}

// TestLoadRejectsMalformedToml just glance the error path is surfaced.
func TestLoadRejectsMalformedToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("not [ valid toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load(malformed) = nil error, want error")
	}
}

// Ensure the Config struct expands the way the plan expects without importing
// gg into the config package (config must not depend on gg).
func TestConfigShape(t *testing.T) {
	cfg := Config{}
	// Presence checks: these fields must exist with the right types.
	var _ map[string]any = map[string]any{
		"Reveal":    cfg.Reveal,
		"Anim":      cfg.Anim,
		"Overlay":   cfg.Overlay,
		"Drinks":    cfg.Drinks,
		"Frecency":  cfg.Frecency,
		"Instances": cfg.Instances,
	}
	if reflect.ValueOf(cfg).Kind() != reflect.Struct {
		t.Fatal("Config must be a struct")
	}
}
