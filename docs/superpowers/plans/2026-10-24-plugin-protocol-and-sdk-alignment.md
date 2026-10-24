# Plugin Protocol & SDK Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align keyshell's plugin wire protocol and SDK with in-process backends by adding `searchable` identity and dynamic instance configuration delivery, while adding filesystem plugin auto-discovery.

**Architecture:** Extend `proto.Hello` and `sdk.Info` with `searchable` boolean, unpack it in `plugin.Host`, and forward `Init.Config` through `sdk.Serve` via a new `sdk.Configurable` interface. Add plugin directory auto-discovery in `internal/cli` to register executables found in user and system plugin directories.

**Tech Stack:** Go standard library (`encoding/json`, `os/exec`, `path/filepath`), `github.com/BurntSushi/toml`.

---

### Task 1: `proto.Hello` Searchable Field & Wire Tests

**Files:**
- Modify: `internal/plugin/proto/proto.go:33-43`
- Test: `internal/plugin/proto/proto_test.go`

- [x] **Step 1: Write the failing tests for proto Hello serialization/deserialization with `Searchable`**

Add tests to `internal/plugin/proto/proto_test.go`:
```go
func TestHelloSearchableSerialization(t *testing.T) {
	tBool := true
	fBool := false

	tests := []struct {
		name       string
		hello      Hello
		wantSubstr string
	}{
		{
			name:       "nil searchable omitted",
			hello:      Hello{Protocol: 1, ID: "test", Searchable: nil},
			wantSubstr: `{"hello":{"protocol":1,"id":"test","name":"","badge":"","trigger":"","controls":null,"hot":false,"stable_ids":false}}`,
		},
		{
			name:       "searchable true present",
			hello:      Hello{Protocol: 1, ID: "test", Searchable: &tBool},
			wantSubstr: `"searchable":true`,
		},
		{
			name:       "searchable false present",
			hello:      Hello{Protocol: 1, ID: "test", Searchable: &fBool},
			wantSubstr: `"searchable":false`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Encode(Message{Hello: &tt.hello})
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if !strings.Contains(string(b), tt.wantSubstr) {
				t.Errorf("Encode() = %s, want to contain %s", string(b), tt.wantSubstr)
			}
			decoded, err := Decode(b)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if decoded.Hello == nil {
				t.Fatal("decoded Hello is nil")
			}
			if tt.hello.Searchable == nil {
				if decoded.Hello.Searchable != nil {
					t.Errorf("decoded Searchable = %v, want nil", *decoded.Hello.Searchable)
				}
			} else {
				if decoded.Hello.Searchable == nil || *decoded.Hello.Searchable != *tt.hello.Searchable {
					t.Errorf("decoded Searchable = %v, want %v", decoded.Hello.Searchable, *tt.hello.Searchable)
				}
			}
		})
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugin/proto -run TestHelloSearchableSerialization -v`
Expected: FAIL with `unknown field Searchable in struct literal`

- [x] **Step 3: Modify `internal/plugin/proto/proto.go` to add `Searchable *bool`**

In `internal/plugin/proto/proto.go`:
```go
// Hello is the plugin's first message (plugin → host).
type Hello struct {
	Protocol   int      `json:"protocol"`
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Badge      string   `json:"badge"`
	Trigger    string   `json:"trigger"`
	Controls   []string `json:"controls"`
	Hot        bool     `json:"hot"`
	StableIDs  bool     `json:"stable_ids"`
	Searchable *bool    `json:"searchable,omitempty"` // nil means default (true)
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plugin/proto -run TestHelloSearchableSerialization -v`
Expected: PASS

- [x] **Step 5: Run all proto tests**

Run: `go test ./internal/plugin/proto -v`
Expected: PASS

- [x] **Step 6: Commit**

```bash
git add internal/plugin/proto/proto.go internal/plugin/proto/proto_test.go
git commit -m "feat(plugin/proto): add searchable field to Hello message"
```

---

### Task 2: `sdk.Info` Searchable, `sdk.Configurable`, and `--info` Support

**Files:**
- Modify: `internal/plugin/sdk/sdk.go`
- Test: `internal/plugin/sdk/sdk_test.go`
- Modify: `internal/backend/audioplugin/audio.go:34-43`
- Modify: `internal/backend/clipplugin/clip.go:46-55`
- Modify: `internal/backend/mediaplugin/media.go:30-38`
- Modify: `internal/backend/netplugin/net.go:56-65`
- Modify: `internal/backend/powerplugin/power.go:33-41`
- Modify: `internal/backend/trayplugin/tray.go:39-48`

- [x] **Step 1: Write the failing tests for `sdk.Configurable`, `Searchable` propagation, and `--info` flag**

In `internal/plugin/sdk/sdk_test.go`, add:
```go
type configurableBackend struct {
	echoBackend
	gotInstance string
	gotConfig   []byte
	confErr     error
}

func (c *configurableBackend) Configure(instance string, cfg []byte) error {
	c.gotInstance = instance
	c.gotConfig = cfg
	return c.confErr
}

func TestServeDeliversConfiguration(t *testing.T) {
	backend := &configurableBackend{}
	input := "{\"init\":{\"instance\":\"custom-sink\",\"config\":{\"rate\":44100}}}\n"
	runServe(t, input, backend)

	if backend.gotInstance != "custom-sink" {
		t.Errorf("gotInstance = %q, want custom-sink", backend.gotInstance)
	}
	wantCfg := "{\"rate\":44100}"
	if string(backend.gotConfig) != wantCfg {
		t.Errorf("gotConfig = %q, want %q", string(backend.gotConfig), wantCfg)
	}
}

func TestServePropagatesSearchable(t *testing.T) {
	backend := &echoBackend{}
	input := "{\"init\":{\"instance\":\"test\"}}\n"

	t.Setenv("KEYSHELL_PLUGIN", "1")
	t.Setenv("KEYSHELL_PROTOCOL", "1")
	t.Setenv("KEYSHELL_INSTANCE", "test")

	oldIn := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldIn }()

	oldOut := os.Stdout
	or, ow, _ := os.Pipe()
	os.Stdout = ow
	defer func() { os.Stdout = oldOut }()

	done := make(chan error, 1)
	go func() {
		// Non-searchable plugin
		done <- Serve(Info{ID: "calc", Searchable: false}, backend)
	}()

	go func() {
		_, _ = w.WriteString(input)
		_ = w.Close()
	}()

	var out bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		_, _ = out.ReadFrom(or)
		close(readDone)
	}()

	<-done
	_ = ow.Close()
	<-readDone
	_ = or.Close()

	sc := bufio.NewScanner(strings.NewReader(out.String()))
	if !sc.Scan() {
		t.Fatal("expected hello line")
	}
	m, err := proto.Decode(sc.Bytes())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if m.Hello == nil || m.Hello.Searchable == nil || *m.Hello.Searchable != false {
		t.Fatalf("expected Hello.Searchable = false, got %+v", m.Hello)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugin/sdk -run "TestServeDeliversConfiguration|TestServePropagatesSearchable" -v`
Expected: FAIL with `unknown field Searchable in struct literal` / `configurableBackend does not implement Backend`

- [x] **Step 3: Update `internal/plugin/sdk/sdk.go`**

In `internal/plugin/sdk/sdk.go`:
1. Add `Searchable bool` to `InfoShape`:
```go
// InfoShape is the plugin's static identity.
type InfoShape struct {
	ID         string
	Name       string
	Badge      string
	Trigger    string
	Controls   []string
	Hot        bool
	StableIDs  bool
	Searchable bool // default true when not set or explicitly true
}
```

2. Add `Configurable` interface:
```go
// Configurable is implemented by plugins that accept dynamic instance configuration.
type Configurable interface {
	Configure(instance string, config []byte) error
}
```

3. Update `sdk.Serve`:
At the start of `Serve`, handle `--info`:
```go
func Serve(info Info, backend Backend) error {
	if slices.Contains(os.Args, "--info") {
		searchable := info.Searchable
		h := proto.Hello{
			Protocol:   proto.Protocol,
			ID:         info.ID,
			Name:       info.Name,
			Badge:      info.Badge,
			Trigger:    info.Trigger,
			Controls:   info.Controls,
			Hot:        info.Hot,
			StableIDs:  info.StableIDs,
			Searchable: &searchable,
		}
		b, err := json.Marshal(h)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	if os.Getenv("KEYSHELL_PLUGIN") == "" {
		return errors.New("sdk: not running as a plugin (KEYSHELL_PLUGIN not set)")
	}
	if got := os.Getenv("KEYSHELL_PROTOCOL"); got != fmt.Sprintf("%d", proto.Protocol) {
		return fmt.Errorf("sdk: protocol mismatch: have %s want %d", got, proto.Protocol)
	}

	w := bufio.NewWriter(os.Stdout)
	write := func(m proto.Message) error {
		b, err := proto.Encode(m)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			return err
		}
		return w.Flush()
	}

	// 1. Hello.
	searchable := info.Searchable
	if err := write(proto.Message{Hello: &proto.Hello{
		Protocol:   proto.Protocol,
		ID:         info.ID,
		Name:       info.Name,
		Badge:      info.Badge,
		Trigger:    info.Trigger,
		Controls:   info.Controls,
		Hot:        info.Hot,
		StableIDs:  info.StableIDs,
		Searchable: &searchable,
	}}); err != nil {
		return err
	}

	// 2. Wait for init, reply ready.
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), protoLineCap)
	if !sc.Scan() {
		return errors.New("sdk: connection closed before init")
	}
	msg, err := proto.Decode(sc.Bytes())
	if err != nil || msg.Init == nil {
		return errors.New("sdk: expected init")
	}
	if conf, ok := backend.(Configurable); ok && len(msg.Init.Config) > 0 {
		if err := conf.Configure(msg.Init.Instance, msg.Init.Config); err != nil {
			return fmt.Errorf("sdk: configure: %w", err)
		}
	}
	if err := write(proto.Message{Ready: &proto.Ready{}}); err != nil {
		return err
	}
```
(Include `slices` in imports).

4. Update `Info()` in the 6 first-party plugins to declare `Searchable: true`:
- `internal/backend/audioplugin/audio.go`: add `Searchable: true`
- `internal/backend/clipplugin/clip.go`: add `Searchable: true`
- `internal/backend/mediaplugin/media.go`: add `Searchable: true`
- `internal/backend/netplugin/net.go`: add `Searchable: true`
- `internal/backend/powerplugin/power.go`: add `Searchable: true`
- `internal/backend/trayplugin/tray.go`: add `Searchable: true`

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plugin/sdk -run "TestServeDeliversConfiguration|TestServePropagatesSearchable" -v`
Expected: PASS

- [x] **Step 5: Run all backend plugin tests**

Run: `go test ./internal/backend/... ./internal/plugin/sdk -v`
Expected: PASS

- [x] **Step 6: Commit**

```bash
git add internal/plugin/sdk internal/backend/audioplugin internal/backend/clipplugin internal/backend/mediaplugin internal/backend/netplugin internal/backend/powerplugin internal/backend/trayplugin
git commit -m "feat(plugin/sdk): add Searchable and Configurable to SDK"
```

---

### Task 3: `plugin.Host` Searchable Unpacking & Init Config Forwarding

**Files:**
- Modify: `internal/plugin/host.go:30-40,130-140,157-170`
- Test: `internal/plugin/host_test.go`

- [x] **Step 1: Write the failing tests for `Host` Searchable unpacking and Init config forwarding**

In `internal/plugin/host_test.go`:
```go
func TestHostUnpacksSearchable(t *testing.T) {
	// fake_plugin.py omits searchable in hello -> host must default to true
	h := newFakeHost(t)
	if !h.Info().Searchable {
		t.Errorf("Info().Searchable = false, want true (default when omitted)")
	}
}

func TestHostForwardsConfig(t *testing.T) {
	// Launch with config payload
	cfgJSON := []byte(`{"rate":48000}`)
	h := &Host{cfg: Launch{
		Instance: "fake-configured",
		Command:  []string{"python3", fakePluginPath(t)},
		Config:   cfgJSON,
	}}
	if err := h.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Close()
	// Handshake succeeds without error
	if !h.Alive() {
		t.Fatal("expected host to be alive")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugin -run "TestHostUnpacksSearchable|TestHostForwardsConfig" -v`
Expected: FAIL with `unknown field Config in Launch`

- [x] **Step 3: Modify `internal/plugin/host.go`**

1. In `Launch` struct:
```go
// Launch describes how to start an instance.
type Launch struct {
	Instance string
	Command  []string
	Env      []string
	Config   json.RawMessage
}
```
2. In `startLocked()`:
```go
	// Send init.
	if err := h.writeLocked(proto.Message{Init: &proto.Init{
		Instance: h.cfg.Instance,
		Config:   h.cfg.Config,
	}}); err != nil {
		_ = h.kill()
		return err
	}
```
3. In `Info()`:
```go
// Info implements backend.Backend from the handshake hello.
func (h *Host) Info() backend.Info {
	h.mu.Lock()
	defer h.mu.Unlock()
	searchable := true
	if h.hello.Searchable != nil {
		searchable = *h.hello.Searchable
	}
	return backend.Info{
		ID:         h.hello.ID,
		Name:       h.hello.Name,
		Badge:      h.hello.Badge,
		Trigger:    h.hello.Trigger,
		Searchable: searchable,
		Trackable:  h.hello.StableIDs,
		StableIDs:  h.hello.StableIDs,
	}
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plugin -run "TestHostUnpacksSearchable|TestHostForwardsConfig" -v`
Expected: PASS

- [x] **Step 5: Run all plugin package tests**

Run: `go test ./internal/plugin -v`
Expected: PASS

- [x] **Step 6: Commit**

```bash
git add internal/plugin/host.go internal/plugin/host_test.go
git commit -m "feat(plugin/host): unpack Hello.Searchable and forward Launch.Config in Init"
```

---

### Task 4: `config.Instance` Extensions & Host Launch Wiring

**Files:**
- Modify: `internal/config/config.go:70-78`
- Test: `internal/config/config_test.go`
- Modify: `internal/cli/host.go:169-182`

- [x] **Step 1: Write failing test for config parsing with `searchable` and `config`**

In `internal/config/config_test.go`:
```go
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
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestConfigInstanceSearchableAndConfig -v`
Expected: FAIL with `unknown field Searchable / Config`

- [x] **Step 3: Modify `internal/config/config.go` and `internal/cli/host.go`**

In `internal/config/config.go`:
```go
// Instance is one configured plugin instance.
type Instance struct {
	ID         string   `toml:"id"`
	Command    []string `toml:"command"`
	Mode       string   `toml:"mode"` // auto|hot|lazy
	NoReap     bool     `toml:"noreap"`
	Searchable *bool    `toml:"searchable,omitempty"`
	Config     any      `toml:"config"`
}
```

In `internal/cli/host.go` `buildProvider`:
```go
	for _, inst := range cfg.Instances {
		cmd := inst.Command
		if len(cmd) == 0 {
			cmd = []string{self, "plugin", inst.ID}
		}
		var cfgBytes json.RawMessage
		if inst.Config != nil {
			if b, err := json.Marshal(inst.Config); err == nil {
				cfgBytes = b
			}
		}
		plugins[inst.ID] = plugin.Launch{
			Instance: inst.ID,
			Command:  cmd,
			Config:   cfgBytes,
		}
	}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config ./internal/cli -v`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go internal/cli/host.go
git commit -m "feat(config): add searchable and config fields to Instance"
```

---

### Task 5: Plugin Auto-Discovery & Manifest/CLI Scanner

**Files:**
- Create: `internal/cli/discovery.go`
- Create: `internal/cli/discovery_test.go`
- Modify: `internal/cli/host.go`
- Modify: `internal/cli/run.go:196-213`

- [x] **Step 1: Write failing test for `scanPluginDirs` and `newLauncherOptions` with non-searchable instances**

Create `internal/cli/discovery_test.go`:
```go
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/keyshell/internal/config"
)

func TestScanPluginDirsWithManifest(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "tool")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	manifestJSON := `{"id":"tool","name":"Tool","trigger":"@","searchable":false}`
	if err := os.WriteFile(filepath.Join(dir, "tool.json"), []byte(manifestJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	instances := scanPluginDirs(dir)
	if len(instances) != 1 {
		t.Fatalf("scanPluginDirs found %d instances, want 1", len(instances))
	}
	inst := instances[0]
	if inst.ID != "tool" {
		t.Errorf("ID = %q, want tool", inst.ID)
	}
	if inst.Searchable == nil || *inst.Searchable != false {
		t.Errorf("Searchable = %v, want false", inst.Searchable)
	}
}

func TestNewLauncherOptionsFiltersNonSearchable(t *testing.T) {
	fBool := false
	cfg := config.Config{
		Instances: []Instance{
			{ID: "searchable-plugin"},
			{ID: "calc-plugin", Searchable: &fBool},
		},
	}
	opts := newLauncherOptions(cfg)

	// Searchable should contain searchable-plugin, but NOT calc-plugin
	var hasSearchable, hasCalcInSearchable bool
	for _, s := range opts.Searchable {
		if s == "searchable-plugin" {
			hasSearchable = true
		}
		if s == "calc-plugin" {
			hasCalcInSearchable = true
		}
	}
	if !hasSearchable {
		t.Error("opts.Searchable missing searchable-plugin")
	}
	if hasCalcInSearchable {
		t.Error("opts.Searchable unexpectedly contains calc-plugin")
	}

	// Modes should contain both
	var hasCalcInModes bool
	for _, m := range opts.Modes {
		if m == "calc-plugin" {
			hasCalcInModes = true
		}
	}
	if !hasCalcInModes {
		t.Error("opts.Modes missing calc-plugin")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run "TestScanPluginDirsWithManifest|TestNewLauncherOptionsFiltersNonSearchable" -v`
Expected: FAIL with `undefined: scanPluginDirs`

- [x] **Step 3: Implement `internal/cli/discovery.go` and update `newLauncherOptions`**

Create `internal/cli/discovery.go`:
```go
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/plugin/proto"
)

// defaultPluginDirs returns the OS-specific paths to scan for plugins.
func defaultPluginDirs() []string {
	var dirs []string
	switch runtime.GOOS {
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, "Library", "Application Support", "keyshell", "plugins"))
		}
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			dirs = append(dirs, filepath.Join(appData, "keyshell", "plugins"))
		}
	default: // Linux and other Unixes
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			dirs = append(dirs, filepath.Join(xdg, "keyshell", "plugins"))
		} else if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, ".config", "keyshell", "plugins"))
		}
		dirs = append(dirs, "/usr/lib/keyshell/plugins")
	}
	return dirs
}

// scanPluginDirs checks the given directories for plugin executables.
func scanPluginDirs(dirs ...string) []config.Instance {
	var instances []config.Instance
	seen := make(map[string]bool)

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			info, err := e.Info()
			if err != nil {
				continue
			}

			// Subdirectory plugin with manifest or binary
			if e.IsDir() {
				// Check for <subdir>/manifest.json or <subdir>/plugin.json
				var manifestPath string
				if m := filepath.Join(path, "manifest.json"); fileExists(m) {
					manifestPath = m
				} else if m := filepath.Join(path, "plugin.json"); fileExists(m) {
					manifestPath = m
				}

				// Check for binary inside subdir matching subdir name or "plugin"
				bin := filepath.Join(path, e.Name())
				if !isExecutable(bin) {
					bin = filepath.Join(path, "plugin")
				}
				if isExecutable(bin) {
					inst, ok := probePlugin(bin, manifestPath)
					if ok && !seen[inst.ID] {
						seen[inst.ID] = true
						instances = append(instances, inst)
					}
				}
				continue
			}

			// Single file executable
			if isExecutable(path) {
				manifestPath := path + ".json"
				if !fileExists(manifestPath) {
					manifestPath = ""
				}
				inst, ok := probePlugin(path, manifestPath)
				if ok && !seen[inst.ID] {
					seen[inst.ID] = true
					instances = append(instances, inst)
				}
			}
		}
	}
	return instances
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func isExecutable(p string) bool {
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		ext := filepath.Ext(p)
		return ext == ".exe" || ext == ".bat" || ext == ".cmd"
	}
	return st.Mode()&0o111 != 0
}

func probePlugin(binPath, manifestPath string) (config.Instance, bool) {
	// Try manifest first if available
	if manifestPath != "" {
		if data, err := os.ReadFile(manifestPath); err == nil {
			var h proto.Hello
			if err := json.Unmarshal(data, &h); err == nil && h.ID != "" {
				return config.Instance{
					ID:         h.ID,
					Command:    []string{binPath},
					Searchable: h.Searchable,
				}, true
			}
		}
	}

	// Try running with --info
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "--info")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err == nil {
		var h proto.Hello
		if err := json.Unmarshal(stdout.Bytes(), &h); err == nil && h.ID != "" {
			return config.Instance{
				ID:         h.ID,
				Command:    []string{binPath},
				Searchable: h.Searchable,
			}, true
		}
	}

	return config.Instance{}, false
}
```

In `internal/cli/host.go` update `loadConfig`:
```go
func loadConfig(hf hostFlags) (config.Config, error) {
	cfg, err := config.Load(hf.config)
	if err != nil {
		return cfg, err
	}
	switch {
	case strings.EqualFold(hf.plugins, "none") || hf.plugins == "-":
		cfg.Instances = nil
		return cfg, nil
	case hf.plugins != "":
		cfg.Instances = nil
		for _, id := range strings.Split(hf.plugins, ",") {
			if id = strings.TrimSpace(id); id != "" {
				cfg.Instances = append(cfg.Instances, config.Instance{ID: id, Mode: "hot"})
			}
		}
	default:
		// Auto-discover plugins from user and system plugin directories
		discovered := scanPluginDirs(defaultPluginDirs()...)
		existing := make(map[string]bool, len(cfg.Instances))
		for _, inst := range cfg.Instances {
			existing[inst.ID] = true
		}
		for _, inst := range discovered {
			if !existing[inst.ID] {
				cfg.Instances = append(cfg.Instances, inst)
			}
		}
	}
	if hf.scene != "" {
		cfg.UI.Scene = hf.scene
	}
	return cfg, nil
}
```

In `internal/cli/run.go` update `newLauncherOptions`:
```go
func newLauncherOptions(cfg config.Config) launcher.Options {
	searchable := []string{"apps", "files", "windows"}
	var nonSearchable []string
	for _, inst := range cfg.Instances {
		if inst.Searchable != nil && !*inst.Searchable {
			nonSearchable = append(nonSearchable, inst.ID)
		} else {
			searchable = append(searchable, inst.ID)
		}
	}
	// Tab walks the merged search, then the searchable modes, then the
	// trigger-only in-process backends (calc, run) and any non-searchable
	// plugins so every mode is reachable from the keyboard.
	modes := append([]string{launcher.MergedMode}, searchable...)
	modes = append(modes, "calc", "run")
	modes = append(modes, nonSearchable...)
	return launcher.Options{
		SettleMs:   settleDuration(cfg.Overlay.SettleMs),
		RankOpts:   rank.Options{Limit: cfg.Overlay.MaxResults},
		Searchable: searchable,
		Modes:      modes,
	}
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run "TestScanPluginDirsWithManifest|TestNewLauncherOptionsFiltersNonSearchable" -v`
Expected: PASS

- [x] **Step 5: Run all CLI tests**

Run: `go test ./internal/cli -v`
Expected: PASS

- [x] **Step 6: Commit**

```bash
git add internal/cli/discovery.go internal/cli/discovery_test.go internal/cli/host.go internal/cli/run.go
git commit -m "feat(cli): add plugin auto-discovery and searchable filtering in launcher options"
```

---

### Task 6: End-to-End Verification Test for Non-Searchable Trigger-Only Plugin in Launcher

**Files:**
- Modify: `internal/launcher/launcher_test.go`

- [x] **Step 1: Write the end-to-end launcher test**

In `internal/launcher/launcher_test.go`:
```go
func TestLauncherExternalPluginNonSearchable(t *testing.T) {
	// Represents an external plugin declaring Searchable: false and Trigger: "="
	extPlugin := &listBackend{
		id:            "extcalc",
		trigger:       "=",
		nonSearchable: true,
		items:         []backend.Item{{ID: "res", Title: "42", Subtitle: "External Calculator"}},
	}
	appsB := &listBackend{
		id:    "apps",
		items: []backend.Item{{ID: "app1", Title: "Terminal"}},
	}

	prov := &fakeProvider{backends: map[string]backend.Backend{
		"extcalc": extPlugin,
		"apps":    appsB,
	}}

	// In keyshell, non-searchable plugins are in Modes but NOT Searchable
	l := New(prov, Options{
		Searchable: []string{"apps"},
		Modes:      []string{MergedMode, "apps", "extcalc"},
		RankOpts:   rank.Options{Limit: 10},
		SettleMs:   20 * time.Millisecond,
	})
	l.Open()

	// 1. In merged search, querying should NOT query extcalc or return its items
	l.SetQuery("42")
	results := l.Results()
	for _, m := range results {
		if m.Source == "extcalc" || m.Item.ID == "res" {
			t.Fatalf("merged search returned result from non-searchable plugin: %+v", m)
		}
	}

	// 2. Typing the trigger prefix "=" should switch mode to extcalc and return its items
	l.SetQuery("= 21 * 2")
	if l.Mode() != "extcalc" {
		t.Fatalf("Mode after trigger prefix = %q, want extcalc", l.Mode())
	}
	calcResults := l.Results()
	if len(calcResults) == 0 {
		t.Fatal("expected results after trigger prefix, got none")
	}
	if calcResults[0].Item.Title != "42" {
		t.Errorf("expected Title = 42, got %q", calcResults[0].Item.Title)
	}
}
```

- [x] **Step 2: Run test to verify it passes**

Run: `go test ./internal/launcher -run TestLauncherExternalPluginNonSearchable -v`
Expected: PASS

- [x] **Step 3: Run all tests in the repository**

Run: `go test ./...`
Expected: PASS across all packages

- [x] **Step 4: Commit**

```bash
git add internal/launcher/launcher_test.go
git commit -m "test(launcher): verify external plugin with searchable false is excluded from merged search and active on trigger"
```

---

### Task 7: Full Verification and Cleanup

**Files:**
- Modify: any if needed

- [x] **Step 1: Run full test suite**

Run: `go test -race ./...`
Expected: PASS

- [x] **Step 2: Verify git status is clean**

Run: `git status`
Expected: Clean working tree
