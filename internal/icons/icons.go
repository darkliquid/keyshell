// Package icons resolves freedesktop icon names to image files following the
// icon theme spec: theme directory layout, the Inherits chain with hicolor
// as the final fallback, and per-directory size matching. Raster formats are
// preferred in lookup order; the renderer decides what it can decode.
package icons

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// exts are the icon file extensions tried per directory, most useful first.
var exts = []string{"png", "webp", "svg", "xpm"}

// Resolver maps icon names to image files. Lookups are cached and the
// resolver is safe for concurrent use.
type Resolver struct {
	theme string
	bases []string

	mu    sync.Mutex
	chain []themeDir
	cache map[string]string
}

// themeDir is one installed theme: its root and the parsed index.theme.
type themeDir struct {
	root string
	dirs []subDir
}

// subDir is one directory entry from a theme's index.theme.
type subDir struct {
	name      string // relative to the theme root, e.g. "48x48/apps"
	size      int
	scale     int
	typ       string // Fixed | Threshold | Scalable
	minSize   int
	maxSize   int
	threshold int
}

// NewResolver builds a resolver for the given theme over the given base
// directories (icon roots and pixmap directories). An empty theme name
// resolves to hicolor.
func NewResolver(theme string, bases ...string) *Resolver {
	if theme == "" {
		theme = "hicolor"
	}
	return &Resolver{theme: theme, bases: bases, cache: map[string]string{}}
}

// DefaultBaseDirs returns the spec's icon search path: ~/.icons, the XDG
// data home, each XDG data dir, and /usr/share/pixmaps.
func DefaultBaseDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{filepath.Join(home, ".icons")}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	dirs = append(dirs, filepath.Join(dataHome, "icons"))
	dataDirs := os.Getenv("XDG_DATA_DIRS")
	if dataDirs == "" {
		dataDirs = "/usr/local/share:/usr/share"
	}
	for _, d := range strings.Split(dataDirs, ":") {
		if d != "" {
			dirs = append(dirs, filepath.Join(d, "icons"))
		}
	}
	return append(dirs, "/usr/share/pixmaps")
}

// Detect returns the icon theme to use: the KEYSHELL_ICON_THEME override,
// then the GTK 4 and GTK 3 settings files, then gsettings, else "" (the spec
// default, hicolor).
func Detect() string {
	if v := os.Getenv("KEYSHELL_ICON_THEME"); v != "" {
		return v
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		home, _ := os.UserHomeDir()
		cfg = filepath.Join(home, ".config")
	}
	for _, rel := range []string{"gtk-4.0/settings.ini", "gtk-3.0/settings.ini"} {
		if name := readGTKIconTheme(filepath.Join(cfg, rel)); name != "" {
			return name
		}
	}
	if name := gsettingsTheme(); name != "" {
		return name
	}
	return ""
}

var gsettingsTheme = queryGSettingsIconTheme

// queryGSettingsIconTheme checks the GNOME/XDG icon theme via gsettings.
func queryGSettingsIconTheme() string {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gsettings", "get", "org.gnome.desktop.interface", "icon-theme").Output()
	if err != nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(string(out)), "'\"")
}

// readGTKIconTheme extracts gtk-icon-theme-name from a GTK settings.ini.
func readGTKIconTheme(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if v, ok := strings.CutPrefix(line, "gtk-icon-theme-name="); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// Lookup resolves an icon name at the given pixel size. An absolute path
// passes through when the file exists, which is how some desktop entries
// name their icon.
func (r *Resolver) Lookup(name string, size int) (string, bool) {
	if strings.HasPrefix(name, "/") {
		if fileExists(name) {
			return name, true
		}
		return "", false
	}

	for _, ext := range exts {
		if strings.HasSuffix(name, "."+ext) {
			name = strings.TrimSuffix(name, "."+ext)
			break
		}
	}

	key := name + "@" + strconv.Itoa(size)
	r.mu.Lock()
	if path, ok := r.cache[key]; ok {
		r.mu.Unlock()
		return path, path != ""
	}
	chain := r.themeChainLocked()
	bases := r.bases
	r.mu.Unlock()

	for _, td := range chain {
		if path, ok := lookupInTheme(td, name, size); ok {
			r.store(key, path)
			return path, true
		}
	}
	// Unthemed fallback: files directly under the base directories.
	for _, base := range bases {
		for _, ext := range exts {
			path := filepath.Join(base, name+"."+ext)
			if fileExists(path) {
				r.store(key, path)
				return path, true
			}
		}
	}
	r.store(key, "")
	return "", false
}

func (r *Resolver) store(key, path string) {
	r.mu.Lock()
	r.cache[key] = path
	r.mu.Unlock()
}

// themeChainLocked builds the lookup order: the theme itself, its Inherits
// chain, and hicolor at the end. Cycles and repeats are visited once.
func (r *Resolver) themeChainLocked() []themeDir {
	if r.chain != nil {
		return r.chain
	}
	seen := map[string]bool{}
	var chain []themeDir
	var add func(name string)
	add = func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true

		var primaryIdx *indexTheme
		for _, base := range r.bases {
			parsed, err := parseIndexTheme(filepath.Join(base, name, "index.theme"))
			if err == nil {
				primaryIdx = &parsed
				break
			}
		}
		if primaryIdx == nil {
			return
		}

		for _, base := range r.bases {
			root := filepath.Join(base, name)
			if !dirExists(root) {
				continue
			}
			dirs := primaryIdx.dirs
			if ownIdx, err := parseIndexTheme(filepath.Join(root, "index.theme")); err == nil && len(ownIdx.dirs) > 0 {
				dirs = ownIdx.dirs
			}
			chain = append(chain, themeDir{root: root, dirs: dirs})
		}

		for _, inh := range primaryIdx.inherits {
			add(inh)
		}
	}
	add(r.theme)
	add("hicolor")
	r.chain = chain
	return chain
}

// lookupInTheme is the spec's two-pass search: an exact size match first,
// then the directory with the smallest size distance.
func lookupInTheme(td themeDir, name string, size int) (string, bool) {
	for _, d := range td.dirs {
		if !matchesSize(d, size) {
			continue
		}
		for _, ext := range exts {
			path := filepath.Join(td.root, d.name, name+"."+ext)
			if fileExists(path) {
				return path, true
			}
		}
	}
	best := ""
	bestDist := -1
	for _, d := range td.dirs {
		dist := sizeDistance(d, size)
		if dist < 0 || (bestDist >= 0 && dist >= bestDist) {
			continue
		}
		for _, ext := range exts {
			path := filepath.Join(td.root, d.name, name+"."+ext)
			if fileExists(path) {
				best, bestDist = path, dist
				break
			}
		}
	}
	if best != "" {
		return best, true
	}
	return "", false
}

// matchesSize reports whether the directory holds icons of exactly the
// requested size, per its type.
func matchesSize(d subDir, size int) bool {
	if d.scale != 1 {
		return false
	}
	switch d.typ {
	case "Fixed":
		return d.size == size
	case "Scalable":
		return d.minSize <= size && size <= d.maxSize
	default: // Threshold
		return d.size-d.threshold <= size && size <= d.size+d.threshold
	}
}

// sizeDistance is the spec's mismatch distance for a directory at the
// requested size; -1 means the directory cannot serve the size at all.
func sizeDistance(d subDir, size int) int {
	if d.scale != 1 {
		return -1
	}
	switch d.typ {
	case "Fixed":
		return abs(d.size - size)
	case "Scalable":
		if size < d.minSize {
			return d.minSize - size
		}
		if size > d.maxSize {
			return size - d.maxSize
		}
		return 0
	default: // Threshold
		if size < d.size-d.threshold {
			return d.size - d.threshold - size
		}
		if size > d.size+d.threshold {
			return size - d.size - d.threshold
		}
		return 0
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// indexTheme is the parsed subset of an index.theme file: the Inherits list
// from [Icon Theme] and one subDir per directory section.
type indexTheme struct {
	inherits []string
	dirs     []subDir
}

// parseIndexTheme reads an index.theme file. Sections whose name is a
// directory listed in [Icon Theme].Directories become subDir entries;
// missing type/size keys take the spec defaults (Threshold, threshold 2,
// scale 1, min/max equal to size).
func parseIndexTheme(path string) (indexTheme, error) {
	f, err := os.Open(path)
	if err != nil {
		return indexTheme{}, err
	}
	defer f.Close()

	var it indexTheme
	dirKeys := map[string]map[string]string{}
	dirOrder := []string{}
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";"):
			continue
		case strings.HasPrefix(line, "["):
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if section == "Icon Theme" {
			switch key {
			case "Directories":
				for _, name := range strings.Split(value, ",") {
					if name = strings.TrimSpace(name); name != "" {
						dirOrder = append(dirOrder, name)
					}
				}
			case "Inherits":
				for _, name := range strings.Split(value, ",") {
					if name = strings.TrimSpace(name); name != "" {
						it.inherits = append(it.inherits, name)
					}
				}
			}
			continue
		}
		if section == "" {
			continue
		}
		if dirKeys[section] == nil {
			dirKeys[section] = map[string]string{}
		}
		dirKeys[section][key] = value
	}
	if err := sc.Err(); err != nil {
		return indexTheme{}, err
	}

	for _, name := range dirOrder {
		keys, ok := dirKeys[name]
		if !ok {
			continue
		}
		d := subDir{
			name:      name,
			scale:     1,
			typ:       "Threshold",
			threshold: 2,
		}
		d.size = atoi(keys["Size"])
		d.minSize, d.maxSize = d.size, d.size
		if v := keys["Scale"]; v != "" {
			d.scale = atoi(v)
		}
		if v := keys["Type"]; v != "" {
			d.typ = v
		}
		if v := keys["MinSize"]; v != "" {
			d.minSize = atoi(v)
		}
		if v := keys["MaxSize"]; v != "" {
			d.maxSize = atoi(v)
		}
		if v := keys["Threshold"]; v != "" {
			d.threshold = atoi(v)
		}
		it.dirs = append(it.dirs, d)
	}
	return it, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
