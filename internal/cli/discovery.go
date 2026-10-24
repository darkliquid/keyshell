package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// resolveSubdirCommand makes relative executable/script paths in cmd absolute relative to dir.
func resolveSubdirCommand(dir string, cmd []string) []string {
	if len(cmd) == 0 {
		return nil
	}
	absDir, err := filepath.Abs(dir)
	if err == nil {
		dir = absDir
	}
	resolved := make([]string, len(cmd))
	copy(resolved, cmd)
	for i, part := range resolved {
		if filepath.IsAbs(part) {
			continue
		}
		candidate := filepath.Join(dir, part)
		if fileExists(candidate) || strings.HasPrefix(part, ".") || strings.Contains(part, string(filepath.Separator)) {
			resolved[i] = candidate
		} else if i == 0 && len(cmd) == 1 {
			if _, err := exec.LookPath(part); err != nil {
				resolved[i] = candidate
			}
		}
	}
	return resolved
}

// scanPluginDirs checks the given directories for plugin executables.
func scanPluginDirs(dirs ...string) []config.Instance {
	var instances []config.Instance
	seen := make(map[string]bool)

	for _, dir := range dirs {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			path := filepath.Join(dir, e.Name())

			if e.IsDir() {
				var manifestPath string
				if m := filepath.Join(path, "manifest.json"); fileExists(m) {
					manifestPath = m
				} else if m := filepath.Join(path, "plugin.json"); fileExists(m) {
					manifestPath = m
				}

				var inst config.Instance
				var hasValidManifest bool
				if manifestPath != "" {
					if data, err := os.ReadFile(manifestPath); err == nil {
						if parsed, ok := parseHelloOrManifest(data); ok {
							inst = parsed
							hasValidManifest = true
						}
					}
				}

				if hasValidManifest {
					if len(inst.Command) > 0 {
						inst.Command = resolveSubdirCommand(path, inst.Command)
					} else {
						bin := findSubdirBinary(path, e.Name())
						if bin != "" {
							inst.Command = []string{bin}
						}
					}
					if len(inst.Command) > 0 && inst.ID != "" && !seen[inst.ID] {
						seen[inst.ID] = true
						instances = append(instances, inst)
					}
				} else {
					// No manifest found or manifest lacked metadata
					bin := findSubdirBinary(path, e.Name())
					if bin != "" {
						probed, ok := probeCommand([]string{bin}, manifestPath)
						if ok && probed.ID != "" && !seen[probed.ID] {
							seen[probed.ID] = true
							instances = append(instances, probed)
						}
					}
				}
				continue
			}

			// Single file executable
			if strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			if isExecutable(path) {
				var manifestPath string
				if fileExists(path + ".json") {
					manifestPath = path + ".json"
				} else if ext := filepath.Ext(path); ext != "" && fileExists(strings.TrimSuffix(path, ext)+".json") {
					manifestPath = strings.TrimSuffix(path, ext) + ".json"
				}
				inst, ok := probeCommand([]string{path}, manifestPath)
				if ok && inst.ID != "" && !seen[inst.ID] {
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
		ext := strings.ToLower(filepath.Ext(p))
		return ext == ".exe" || ext == ".bat" || ext == ".cmd"
	}
	return st.Mode()&0o111 != 0
}

func findSubdirBinary(dir, name string) string {
	candidates := []string{
		filepath.Join(dir, name),
		filepath.Join(dir, "plugin"),
		filepath.Join(dir, "main"),
		filepath.Join(dir, "run"),
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			filepath.Join(dir, name+".exe"),
			filepath.Join(dir, name+".bat"),
			filepath.Join(dir, name+".cmd"),
			filepath.Join(dir, "plugin.exe"),
			filepath.Join(dir, "plugin.bat"),
			filepath.Join(dir, "plugin.cmd"),
			filepath.Join(dir, "main.exe"),
			filepath.Join(dir, "main.bat"),
			filepath.Join(dir, "main.cmd"),
			filepath.Join(dir, "run.exe"),
			filepath.Join(dir, "run.bat"),
			filepath.Join(dir, "run.cmd"),
		)
	}
	for _, c := range candidates {
		if isExecutable(c) {
			return c
		}
	}
	return ""
}

type manifestFormat struct {
	proto.Hello
	Command []string `json:"command"`
	Mode    string   `json:"mode,omitempty"`
}

func parseHelloOrManifest(data []byte) (config.Instance, bool) {
	var m manifestFormat
	if err := json.Unmarshal(data, &m); err == nil && m.ID != "" {
		mode := m.Mode
		if mode == "" && m.Hot {
			mode = "hot"
		}
		return config.Instance{
			ID:         m.ID,
			Command:    m.Command,
			Mode:       mode,
			Searchable: m.Searchable,
		}, true
	}
	var msg proto.Message
	if err := json.Unmarshal(data, &msg); err == nil && msg.Hello != nil && msg.Hello.ID != "" {
		mode := ""
		if msg.Hello.Hot {
			mode = "hot"
		}
		return config.Instance{
			ID:         msg.Hello.ID,
			Mode:       mode,
			Searchable: msg.Hello.Searchable,
		}, true
	}
	return config.Instance{}, false
}

func probePlugin(binPath, manifestPath string) (config.Instance, bool) {
	return probeCommand([]string{binPath}, manifestPath)
}

func probeCommand(command []string, manifestPath string) (config.Instance, bool) {
	// Try manifest first if available
	if manifestPath != "" {
		if data, err := os.ReadFile(manifestPath); err == nil {
			if inst, ok := parseHelloOrManifest(data); ok {
				if len(inst.Command) == 0 {
					inst.Command = command
				}
				return inst, true
			}
		}
	}

	if len(command) == 0 {
		return config.Instance{}, false
	}

	// Try running with --info
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	bin := command[0]
	args := make([]string, 0, len(command))
	args = append(args, command[1:]...)
	args = append(args, "--info")

	cmd := exec.CommandContext(ctx, bin, args...)
	if len(command) > 1 && fileExists(command[1]) {
		cmd.Dir = filepath.Dir(command[1])
	} else if fileExists(bin) {
		cmd.Dir = filepath.Dir(bin)
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err == nil {
		if inst, ok := parseHelloOrManifest(stdout.Bytes()); ok {
			if len(inst.Command) == 0 {
				inst.Command = command
			}
			return inst, true
		}
	}

	return config.Instance{}, false
}

// resolveInstanceSearchable probes an instance's command to resolve Searchable
// via manifest or --info if Searchable is unset.
func resolveInstanceSearchable(inst *config.Instance) {
	if inst.Searchable != nil || len(inst.Command) == 0 {
		return
	}

	var manifestPath string
	if len(inst.Command) > 1 {
		arg := inst.Command[1]
		if !strings.HasPrefix(arg, "-") {
			if fileExists(arg + ".json") {
				manifestPath = arg + ".json"
			} else if ext := filepath.Ext(arg); ext != "" && fileExists(strings.TrimSuffix(arg, ext)+".json") {
				manifestPath = strings.TrimSuffix(arg, ext) + ".json"
			} else if fileExists(arg) || filepath.IsAbs(arg) || strings.Contains(arg, string(filepath.Separator)) {
				dir := filepath.Dir(arg)
				if fileExists(filepath.Join(dir, "manifest.json")) {
					manifestPath = filepath.Join(dir, "manifest.json")
				} else if fileExists(filepath.Join(dir, "plugin.json")) {
					manifestPath = filepath.Join(dir, "plugin.json")
				}
			}
		}
	}
	if manifestPath == "" {
		bin := inst.Command[0]
		if fullPath, err := exec.LookPath(bin); err == nil {
			bin = fullPath
		}
		if fileExists(bin + ".json") {
			manifestPath = bin + ".json"
		} else if ext := filepath.Ext(bin); ext != "" && fileExists(strings.TrimSuffix(bin, ext)+".json") {
			manifestPath = strings.TrimSuffix(bin, ext) + ".json"
		} else if fileExists(bin) || filepath.IsAbs(bin) || strings.Contains(bin, string(filepath.Separator)) {
			dir := filepath.Dir(bin)
			if fileExists(filepath.Join(dir, "manifest.json")) {
				manifestPath = filepath.Join(dir, "manifest.json")
			} else if fileExists(filepath.Join(dir, "plugin.json")) {
				manifestPath = filepath.Join(dir, "plugin.json")
			}
		}
	}

	if probed, ok := probeCommand(inst.Command, manifestPath); ok {
		if inst.Searchable == nil && probed.Searchable != nil {
			inst.Searchable = probed.Searchable
		}
		if inst.Mode == "" && probed.Mode != "" {
			inst.Mode = probed.Mode
		}
	}
}

// ResolveInstanceSearchable probes an instance's command to resolve Searchable
// via manifest or --info if Searchable is unset.
func ResolveInstanceSearchable(inst *config.Instance) {
	resolveInstanceSearchable(inst)
}
