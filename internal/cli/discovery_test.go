package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/keyshell/internal/config"
	"github.com/darkliquid/keyshell/internal/launcher"
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
	if len(inst.Command) != 1 || inst.Command[0] != binPath {
		t.Errorf("Command = %v, want [%s]", inst.Command, binPath)
	}
}

func TestScanPluginDirsWithInfoFlag(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "script-tool")
	script := `#!/bin/sh
if [ "$1" = "--info" ]; then
    echo '{"protocol":1,"id":"script-tool","name":"Script Tool","trigger":"#","searchable":true}'
    exit 0
fi
exit 0
`
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	instances := scanPluginDirs(dir)
	if len(instances) != 1 {
		t.Fatalf("scanPluginDirs found %d instances, want 1", len(instances))
	}
	inst := instances[0]
	if inst.ID != "script-tool" {
		t.Errorf("ID = %q, want script-tool", inst.ID)
	}
	if inst.Searchable == nil || *inst.Searchable != true {
		t.Errorf("Searchable = %v, want true", inst.Searchable)
	}
	if len(inst.Command) != 1 || inst.Command[0] != binPath {
		t.Errorf("Command = %v, want [%s]", inst.Command, binPath)
	}
}

func TestNewLauncherOptionsFiltersNonSearchable(t *testing.T) {
	tBool := true
	fBool := false
	cfg := config.Config{
		Instances: []config.Instance{
			{ID: "searchable-plugin", Searchable: &tBool},
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
	var hasCalcInModes, hasSearchableInModes bool
	for _, m := range opts.Modes {
		if m == "calc-plugin" {
			hasCalcInModes = true
		}
		if m == "searchable-plugin" {
			hasSearchableInModes = true
		}
	}
	if !hasSearchableInModes {
		t.Error("opts.Modes missing searchable-plugin")
	}
	if !hasCalcInModes {
		t.Error("opts.Modes missing calc-plugin")
	}

	// Modes should start with MergedMode
	if len(opts.Modes) == 0 || opts.Modes[0] != launcher.MergedMode {
		t.Errorf("opts.Modes[0] = %v, want %v", opts.Modes, launcher.MergedMode)
	}
}

func TestScanPluginDirsSubdirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "myplugin")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(subDir, "myplugin")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifestJSON := `{"id":"myplugin","name":"My Plugin","searchable":false}`
	if err := os.WriteFile(filepath.Join(subDir, "manifest.json"), []byte(manifestJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	instances := scanPluginDirs(dir)
	if len(instances) != 1 {
		t.Fatalf("scanPluginDirs found %d instances, want 1", len(instances))
	}
	inst := instances[0]
	if inst.ID != "myplugin" {
		t.Errorf("ID = %q, want myplugin", inst.ID)
	}
	if inst.Searchable == nil || *inst.Searchable != false {
		t.Errorf("Searchable = %v, want false", inst.Searchable)
	}
	if len(inst.Command) != 1 || inst.Command[0] != binPath {
		t.Errorf("Command = %v, want [%s]", inst.Command, binPath)
	}
}

func TestScanPluginDirsSubdirectoryNonExecutableScript(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "python-plugin")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(subDir, "plugin.py")
	// chmod 0644 (not executable)
	if err := os.WriteFile(scriptPath, []byte("print('hello')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestJSON := `{"id":"py-plugin","name":"Python Plugin","command":["python3","plugin.py"],"searchable":false,"hot":true}`
	if err := os.WriteFile(filepath.Join(subDir, "manifest.json"), []byte(manifestJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	instances := scanPluginDirs(dir)
	if len(instances) != 1 {
		t.Fatalf("scanPluginDirs found %d instances, want 1", len(instances))
	}
	inst := instances[0]
	if inst.ID != "py-plugin" {
		t.Errorf("ID = %q, want py-plugin", inst.ID)
	}
	if inst.Mode != "hot" {
		t.Errorf("Mode = %q, want hot", inst.Mode)
	}
	if inst.Searchable == nil || *inst.Searchable != false {
		t.Errorf("Searchable = %v, want false", inst.Searchable)
	}
	wantCmd := []string{"python3", scriptPath}
	if len(inst.Command) != len(wantCmd) || inst.Command[0] != wantCmd[0] || inst.Command[1] != wantCmd[1] {
		t.Errorf("Command = %v, want %v", inst.Command, wantCmd)
	}
}

func TestScanPluginDirsIgnoresHiddenAndArbitraryScripts(t *testing.T) {
	dir := t.TempDir()

	// .git directory with executable
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "hook"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Subdir with non-conventional executable build.sh and no manifest
	projDir := filepath.Join(dir, "myproject")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "build.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	instances := scanPluginDirs(dir)
	if len(instances) != 0 {
		t.Fatalf("scanPluginDirs found %d instances, want 0: %v", len(instances), instances)
	}
}

func TestResolveInstanceSearchable(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "cli-probe")
	script := `#!/bin/sh
if [ "$1" = "--info" ]; then
    echo '{"protocol":1,"id":"cli-probe","name":"CLI Probe","searchable":false}'
    exit 0
fi
exit 0
`
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	inst := config.Instance{
		ID:      "cli-probe",
		Command: []string{binPath},
	}
	resolveInstanceSearchable(&inst)
	if inst.Searchable == nil || *inst.Searchable != false {
		t.Errorf("Searchable = %v, want false", inst.Searchable)
	}
}

func TestResolveInstanceSearchableMultiToken(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "script.sh")
	script := `#!/bin/sh
if [ "$1" = "--info" ]; then
    echo '{"protocol":1,"id":"multi-tool","searchable":false,"hot":true}'
    exit 0
fi
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	inst := config.Instance{
		ID:      "multi-tool",
		Command: []string{"sh", scriptPath},
	}
	resolveInstanceSearchable(&inst)
	if inst.Searchable == nil || *inst.Searchable != false {
		t.Errorf("Searchable = %v, want false", inst.Searchable)
	}
	if inst.Mode != "hot" {
		t.Errorf("Mode = %q, want hot", inst.Mode)
	}
}

func TestResolveInstanceSearchableMultiTokenManifest(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "runner.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestJSON := `{"id":"multi-manifest","searchable":false,"hot":true}`
	if err := os.WriteFile(scriptPath+".json", []byte(manifestJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	inst := config.Instance{
		ID:      "multi-manifest",
		Command: []string{"sh", scriptPath},
	}
	resolveInstanceSearchable(&inst)
	if inst.Searchable == nil || *inst.Searchable != false {
		t.Errorf("Searchable = %v, want false", inst.Searchable)
	}
	if inst.Mode != "hot" {
		t.Errorf("Mode = %q, want hot", inst.Mode)
	}
}

func TestParseHelloOrManifestRequiresID(t *testing.T) {
	// Name only, no ID: must fail
	nameOnly := []byte(`{"name":"My Plugin With Spaces"}`)
	if _, ok := parseHelloOrManifest(nameOnly); ok {
		t.Error("parseHelloOrManifest succeeded for name-only JSON, want failure")
	}

	// Valid ID with hot
	withHot := []byte(`{"id":"hot-plugin","hot":true}`)
	inst, ok := parseHelloOrManifest(withHot)
	if !ok {
		t.Fatal("parseHelloOrManifest failed for valid hot-plugin")
	}
	if inst.ID != "hot-plugin" {
		t.Errorf("ID = %q, want hot-plugin", inst.ID)
	}
	if inst.Mode != "hot" {
		t.Errorf("Mode = %q, want hot", inst.Mode)
	}
}

func TestLoadConfigAutoDiscovery(t *testing.T) {
	tmp := t.TempDir()
	pluginsDir := filepath.Join(tmp, "keyshell", "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", tmp)

	binPath := filepath.Join(pluginsDir, "discovered-tool")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifestJSON := `{"id":"discovered-tool","name":"Discovered Tool","searchable":false}`
	if err := os.WriteFile(binPath+".json", []byte(manifestJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(hostFlags{config: filepath.Join(tmp, "absent.toml")})
	if err != nil {
		t.Fatal(err)
	}

	var found *config.Instance
	for i := range cfg.Instances {
		if cfg.Instances[i].ID == "discovered-tool" {
			found = &cfg.Instances[i]
			break
		}
	}
	if found == nil {
		t.Fatal("discovered-tool not found in cfg.Instances")
	}
	if found.Searchable == nil || *found.Searchable != false {
		t.Errorf("Searchable = %v, want false", found.Searchable)
	}
}
