// Package icons resolves freedesktop icon names to image files through the
// icon theme spec's directory layout, inheritance chain and size matching.
package icons

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// fixture builds an icon tree under one root and returns the base dirs to
// hand the resolver: the XDG icons root plus a pixmaps directory.
func fixture(t *testing.T) (root string, bases []string) {
	t.Helper()
	root = t.TempDir()

	writeFile(t, root, "icons/themeA/index.theme", `[Icon Theme]
Name=themeA
Inherits=hicolor
Directories=48x48/apps,64x64/apps,96x96/apps,scalable/apps

[48x48/apps]
Size=48
Type=Fixed

[64x64/apps]
Size=64
Type=Fixed

[96x96/apps]
Size=96
Type=Fixed

[scalable/apps]
Type=Scalable
MinSize=1
MaxSize=512
`)
	writePNG(t, root, "icons/themeA/48x48/apps/terminal.png", color.RGBA{R: 255, A: 255})
	writePNG(t, root, "icons/themeA/64x64/apps/terminal.png", color.RGBA{R: 255, A: 255})
	writePNG(t, root, "icons/themeA/96x96/apps/terminal.png", color.RGBA{R: 255, A: 255})
	writeFile(t, root, "icons/themeA/scalable/apps/browser.svg", "<svg/>")

	writeFile(t, root, "icons/hicolor/index.theme", `[Icon Theme]
Name=hicolor
Directories=48x48/apps

[48x48/apps]
Size=48
Type=Fixed
`)
	writePNG(t, root, "icons/hicolor/48x48/apps/editor.png", color.RGBA{G: 255, A: 255})

	writePNG(t, root, "pixmaps/legacy.png", color.RGBA{B: 255, A: 255})

	return root, []string{filepath.Join(root, "icons"), filepath.Join(root, "pixmaps")}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePNG(t *testing.T, root, rel string, c color.RGBA) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// TestLookupExactSize proves a fixed directory at the requested size wins.
func TestLookupExactSize(t *testing.T) {
	root, bases := fixture(t)
	r := NewResolver("themeA", bases...)
	path, ok := r.Lookup("terminal", 48)
	if !ok {
		t.Fatal("terminal at 48 not found")
	}
	want := filepath.Join(root, "icons/themeA/48x48/apps/terminal.png")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}

// TestLookupClosestSize proves the distance rule: asking for 40 resolves to
// the 48px icon, whose distance beats the 64px and 96px copies.
func TestLookupClosestSize(t *testing.T) {
	_, bases := fixture(t)
	r := NewResolver("themeA", bases...)
	path, ok := r.Lookup("terminal", 40)
	if !ok {
		t.Fatal("terminal at 40 not found")
	}
	if filepath.Base(filepath.Dir(filepath.Dir(path))) != "48x48" {
		t.Errorf("path = %q, want the 48px directory", path)
	}
}

// TestLookupScalable proves a scalable directory matches any size inside
// its min/max range.
func TestLookupScalable(t *testing.T) {
	root, bases := fixture(t)
	r := NewResolver("themeA", bases...)
	path, ok := r.Lookup("browser", 96)
	if !ok {
		t.Fatal("browser at 96 not found")
	}
	if path != filepath.Join(root, "icons/themeA/scalable/apps/browser.svg") {
		t.Errorf("path = %q, want the scalable copy", path)
	}
}

// TestLookupFollowsInheritance proves a miss in the active theme falls
// through to the inherited one.
func TestLookupFollowsInheritance(t *testing.T) {
	root, bases := fixture(t)
	r := NewResolver("themeA", bases...)
	path, ok := r.Lookup("editor", 48)
	if !ok {
		t.Fatal("editor not found through hicolor")
	}
	if path != filepath.Join(root, "icons/hicolor/48x48/apps/editor.png") {
		t.Errorf("path = %q, want the hicolor copy", path)
	}
}

// TestLookupUnthemedFallback proves a miss everywhere lands in the pixmap
// base directories.
func TestLookupUnthemedFallback(t *testing.T) {
	root, bases := fixture(t)
	r := NewResolver("themeA", bases...)
	path, ok := r.Lookup("legacy", 48)
	if !ok {
		t.Fatal("legacy not found in pixmaps")
	}
	if path != filepath.Join(root, "pixmaps/legacy.png") {
		t.Errorf("path = %q, want the pixmaps copy", path)
	}
}

// TestLookupMissing reports not-found for unknown names.
func TestLookupMissing(t *testing.T) {
	_, bases := fixture(t)
	r := NewResolver("themeA", bases...)
	if path, ok := r.Lookup("definitely-not-an-icon", 48); ok {
		t.Errorf("Lookup(missing) = %q, want not found", path)
	}
}

// TestLookupStripsExtension proves names carrying an image extension resolve
// without doubling the extension.
func TestLookupStripsExtension(t *testing.T) {
	root, bases := fixture(t)
	r := NewResolver("themeA", bases...)
	path, ok := r.Lookup("terminal.png", 48)
	if !ok {
		t.Fatal("terminal.png at 48 not found")
	}
	want := filepath.Join(root, "icons/themeA/48x48/apps/terminal.png")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}

// TestLookupAbsolutePathPassesThrough proves absolute icon names (some
// desktop entries carry them) are returned as-is when the file exists.
func TestLookupAbsolutePathPassesThrough(t *testing.T) {
	root, bases := fixture(t)
	r := NewResolver("themeA", bases...)
	want := filepath.Join(root, "pixmaps/legacy.png")
	path, ok := r.Lookup(want, 48)
	if !ok || path != want {
		t.Errorf("Lookup(absolute) = %q %v, want passthrough", path, ok)
	}
}

// TestDetectGTKTheme proves the theme detection order: the environment
// override, then the GTK settings files.
func TestDetectGTKTheme(t *testing.T) {
	t.Setenv("KEYSHELL_ICON_THEME", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir()))
	cfg := os.Getenv("XDG_CONFIG_HOME")
	writeFile(t, cfg, "gtk-3.0/settings.ini", "[Settings]\ngtk-icon-theme-name=Papirus\n")
	if got := Detect(); got != "Papirus" {
		t.Errorf("Detect() = %q, want Papirus from gtk-3.0 settings", got)
	}
	t.Setenv("KEYSHELL_ICON_THEME", "themeA")
	if got := Detect(); got != "themeA" {
		t.Errorf("Detect() = %q, want the environment override", got)
	}
}

// TestDetectFallsBackToGSettings proves a dconf-managed theme is detected
// when no GTK settings.ini exists.
func TestDetectFallsBackToGSettings(t *testing.T) {
	t.Setenv("KEYSHELL_ICON_THEME", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	old := gsettingsTheme
	t.Cleanup(func() { gsettingsTheme = old })
	gsettingsTheme = func() string { return "breeze" }
	if got := Detect(); got != "breeze" {
		t.Errorf("Detect() = %q, want breeze from gsettings", got)
	}
}

// TestDetectWithoutSettings falls back to the spec default.
func TestDetectWithoutSettings(t *testing.T) {
	t.Setenv("KEYSHELL_ICON_THEME", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir()))
	old := gsettingsTheme
	t.Cleanup(func() { gsettingsTheme = old })
	gsettingsTheme = func() string { return "" }
	if got := Detect(); got != "" {
		t.Errorf("Detect() = %q, want empty (hicolor fallback)", got)
	}
}

// TestDefaultBaseDirs proves the XDG search order: user icons, data icons,
// then the system directories and pixmaps.
func TestDefaultBaseDirs(t *testing.T) {
	home := t.TempDir()
	data := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_DATA_DIRS", "/usr/local/share:/usr/share")
	dirs := DefaultBaseDirs()
	want := []string{
		filepath.Join(home, ".icons"),
		filepath.Join(data, "icons"),
		"/usr/local/share/icons",
		"/usr/share/icons",
		"/usr/share/pixmaps",
	}
	if len(dirs) != len(want) {
		t.Fatalf("dirs = %q, want %q", dirs, want)
	}
	for i := range want {
		if dirs[i] != want[i] {
			t.Errorf("dirs[%d] = %q, want %q", i, dirs[i], want[i])
		}
	}
}

// TestLookupIsRepeatable proves the name cache returns the same answer
// without rescanning.
func TestLookupIsRepeatable(t *testing.T) {
	_, bases := fixture(t)
	r := NewResolver("themeA", bases...)
	first, ok1 := r.Lookup("terminal", 48)
	second, ok2 := r.Lookup("terminal", 48)
	if !ok1 || !ok2 || first != second {
		t.Errorf("Lookup repeated = %q %v, %q %v", first, ok1, second, ok2)
	}
}

// TestLookupAcrossMultipleBases proves that icons for the same theme spread
// across multiple base directories (e.g. system + flatpak/local) are both found.
func TestLookupAcrossMultipleBases(t *testing.T) {
	base1 := t.TempDir()
	base2 := t.TempDir()

	writeFile(t, base1, "themeA/index.theme", `[Icon Theme]
Name=themeA
Directories=48x48/apps

[48x48/apps]
Size=48
Type=Fixed
`)
	writePNG(t, base1, "themeA/48x48/apps/app1.png", color.RGBA{R: 255, A: 255})
	writePNG(t, base2, "themeA/48x48/apps/app2.png", color.RGBA{G: 255, A: 255})

	r := NewResolver("themeA", base1, base2)
	p1, ok1 := r.Lookup("app1", 48)
	if !ok1 || p1 != filepath.Join(base1, "themeA/48x48/apps/app1.png") {
		t.Errorf("Lookup(app1) = %q %v, want app1 from base1", p1, ok1)
	}
	p2, ok2 := r.Lookup("app2", 48)
	if !ok2 || p2 != filepath.Join(base2, "themeA/48x48/apps/app2.png") {
		t.Errorf("Lookup(app2) = %q %v, want app2 from base2", p2, ok2)
	}
}

// TestSmokeSystemIcons inspects lookup against whatever icon themes are
// installed on the host system without asserting specific paths.
func TestSmokeSystemIcons(t *testing.T) {
	theme := Detect()
	r := NewResolver(theme, DefaultBaseDirs()...)
	t.Logf("detected theme: %s", r.theme)
	for _, name := range []string{"btop", "com.mitchellh.ghostty", "google-chrome", "Alacritty"} {
		p, ok := r.Lookup(name, 48)
		t.Logf("icon %-22s -> %v (%s)", name, ok, p)
	}
}
