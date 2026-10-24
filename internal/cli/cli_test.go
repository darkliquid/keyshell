package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/keyshell/internal/config"
)

func TestDispatchableCommands(t *testing.T) {
	for _, cmd := range []string{"run", "headless", "query", "tui"} {
		if !dispatchable(cmd) {
			t.Errorf("dispatchable(%q) = false, want true", cmd)
		}
	}
	for _, cmd := range []string{"toggle", "plugin", "version", "nope"} {
		if dispatchable(cmd) {
			t.Errorf("dispatchable(%q) = true, want false", cmd)
		}
	}
}

func TestGlobalFlagPositionals(t *testing.T) {
	rest := globalFlagPositionals([]string{"--log-level", "debug", "query", "firef", "--json"})
	if len(rest) != 3 || rest[0] != "query" || rest[1] != "firef" || rest[2] != "--json" {
		t.Fatalf("positionals = %q, want [query firef --json]", rest)
	}
	if rest := globalFlagPositionals([]string{"--config", "/tmp/x.toml"}); len(rest) != 0 {
		t.Fatalf("flags-only positionals = %q, want none", rest)
	}
}

func TestLoadConfigPluginsOverride(t *testing.T) {
	t.Run("default keeps configured instances", func(t *testing.T) {
		cfg, err := loadConfig(hostFlags{})
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Instances) == 0 {
			t.Fatal("instances = none, want the configured defaults")
		}
	})
	t.Run("none clears every instance", func(t *testing.T) {
		cfg, err := loadConfig(hostFlags{plugins: "none"})
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Instances) != 0 {
			t.Fatalf("instances = %d, want 0", len(cfg.Instances))
		}
	})
	t.Run("list overrides instances", func(t *testing.T) {
		cfg, err := loadConfig(hostFlags{plugins: " audio, media ,custom"})
		if err != nil {
			t.Fatal(err)
		}
		got := []string{}
		for _, inst := range cfg.Instances {
			got = append(got, inst.ID)
		}
		if len(got) != 3 || got[0] != "audio" || got[2] != "custom" {
			t.Fatalf("instances = %q, want audio media custom", got)
		}
	})
	t.Run("scene overrides ui.scene", func(t *testing.T) {
		cfg, err := loadConfig(hostFlags{scene: "/tmp/theme.ksl"})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.UI.Scene != "/tmp/theme.ksl" {
			t.Fatalf("ui.scene = %q", cfg.UI.Scene)
		}
	})
}

func TestDefaultConfigPathFromEnv(t *testing.T) {
	t.Setenv("KEYSHELL_CONFIG", "/opt/keyshell.toml")
	if got := defaultConfigPath(); got != "/opt/keyshell.toml" {
		t.Fatalf("config path = %q", got)
	}
	t.Setenv("KEYSHELL_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := defaultConfigPath(); got != "/xdg/keyshell/config.toml" {
		t.Fatalf("config path = %q", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	home, _ := os.UserHomeDir()
	if got := defaultConfigPath(); got != filepath.Join(home, ".config/keyshell/config.toml") {
		t.Fatalf("config path = %q", got)
	}
}

func TestLoadConfigMissingFileIsFine(t *testing.T) {
	cfg, err := loadConfig(hostFlags{config: filepath.Join(t.TempDir(), "absent.toml")})
	if err != nil {
		t.Fatalf("missing config: %v", err)
	}
	if cfg.Overlay.MaxResults != config.Default().Overlay.MaxResults {
		t.Fatalf("max results = %d, want the default", cfg.Overlay.MaxResults)
	}
}

func TestBuildProviderWithInstanceConfig(t *testing.T) {
	cfg := config.Config{
		Instances: []config.Instance{
			{
				ID:      "test-plugin",
				Command: []string{"test-cmd"},
				Config:  map[string]any{"key": "value"},
			},
		},
	}
	p := buildProvider(cfg)
	launch, ok := p.plugins["test-plugin"]
	if !ok {
		t.Fatal("expected test-plugin in plugins")
	}
	if string(launch.Config) != `{"key":"value"}` {
		t.Fatalf("expected launch.Config to be %s, got %s", `{"key":"value"}`, string(launch.Config))
	}
}
