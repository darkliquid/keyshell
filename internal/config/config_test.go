package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestCommandsReadFromToml proves the [commands] table configures the host
// actions instead of the built-in executables.
func TestCommandsReadFromToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	toml := `
[commands]
copy = ["wl-copy", "--type", "text/plain"]
open_url = ["firefox", "--new-window"]
focus_window = ["swaymsg", "[con_id={id}]", "focus"]
terminal = ["foot", "kitty"]
`
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load = err %v", err)
	}
	if !reflect.DeepEqual(cfg.Commands.Copy, []string{"wl-copy", "--type", "text/plain"}) {
		t.Errorf("Commands.Copy = %v", cfg.Commands.Copy)
	}
	if !reflect.DeepEqual(cfg.Commands.OpenURL, []string{"firefox", "--new-window"}) {
		t.Errorf("Commands.OpenURL = %v", cfg.Commands.OpenURL)
	}
	if !reflect.DeepEqual(cfg.Commands.FocusWindow, []string{"swaymsg", "[con_id={id}]", "focus"}) {
		t.Errorf("Commands.FocusWindow = %v", cfg.Commands.FocusWindow)
	}
	if !reflect.DeepEqual(cfg.Commands.Terminal, []string{"foot", "kitty"}) {
		t.Errorf("Commands.Terminal = %v", cfg.Commands.Terminal)
	}
}

// TestDefaultsAreReturnedWhenFileMissing covers the documented behaviour: a
// missing config file is not an error, it yields defaults.
func TestDefaultsAreReturnedWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("Load(missing) = err %v, want nil", err)
	}
	if cfg.Reveal.ShearMinDeg >= cfg.Reveal.ShearMaxDeg {
		t.Errorf("Reveal shear range invalid: min=%v max=%v", cfg.Reveal.ShearMinDeg, cfg.Reveal.ShearMaxDeg)
	}
	if cfg.Reveal.BandHeightPx <= 0 {
		t.Errorf("Reveal.BandHeightPx must be positive, got %v", cfg.Reveal.BandHeightPx)
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
	if cfg.Reveal.BandHeightPx == 0 {
		t.Errorf("defaults lost behind TOML: Reveal.BandHeightPx = 0")
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
		"Commands":  cfg.Commands,
		"Frecency":  cfg.Frecency,
		"Instances": cfg.Instances,
	}
	if reflect.ValueOf(cfg).Kind() != reflect.Struct {
		t.Fatal("Config must be a struct")
	}
}

func TestConfigInstanceSearchableAndConfig(t *testing.T) {
	tomlStr := `
[[instance]]
id = "my-calc"
command = ["keyshell-calc"]
searchable = false
[instance.config]
precision = 10
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(tomlStr), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var found *Instance
	for i := range cfg.Instances {
		if cfg.Instances[i].ID == "my-calc" {
			found = &cfg.Instances[i]
			break
		}
	}
	if found == nil {
		t.Fatal("my-calc instance not found")
	}
	if found.Searchable == nil || *found.Searchable != false {
		t.Errorf("Searchable = %v, want false", found.Searchable)
	}
	cfgMap, ok := found.Config.(map[string]any)
	if !ok || cfgMap["precision"] != int64(10) {
		t.Errorf("Config = %+v, want precision: 10", found.Config)
	}
}
