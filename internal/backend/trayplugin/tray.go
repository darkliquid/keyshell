// Package trayplugin provides the tray backend. It lists registered
// StatusNotifierItem applets as cards. Reading and acting on each applet's
// DBusMenu is the recorded follow-on; the first cut surfaces the applets.
package trayplugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/darkliquid/keyshell/internal/plugin/sdk"
	"github.com/godbus/dbus/v5"
)

const itemPath = "/StatusNotifierItem"

// Backend is the tray plugin.
type Backend struct {
	watcher interface {
		Items() []string
	}
	conn *dbus.Conn
}

// New builds the tray backend over a live session connection and watcher.
func New(conn *dbus.Conn, w *Watcher) *Backend {
	return &Backend{conn: conn, watcher: w}
}

// Info implements sdk.Backend.
func (b *Backend) Info() sdk.Info {
	return sdk.Info{
		ID:         "tray",
		Name:       "Tray",
		Badge:      "TRAY",
		Trigger:    "^",
		Controls:   nil,
		Hot:        true,
		StableIDs:  true, // applet service names are stable for a session
		Searchable: true,
	}
}

// Query lists applets as items. A scoped query (a child level) returns that
// applet's menu items.
func (b *Backend) Query(_ context.Context, q sdk.Query) (sdk.Stream, error) {
	if len(q.Scope) > 0 {
		return sdk.NewStream(b.menuItems(q.Scope)...), nil
	}
	if rw, ok := b.watcher.(interface{ Refresh() }); ok {
		if len(b.watcher.Items()) == 0 {
			rw.Refresh()
		}
	}
	services := b.watcher.Items()
	items := make([]sdk.Item, 0, len(services))
	seenTitles := make(map[string]bool)
	seenOwners := make(map[string]bool)

	for _, svc := range services {
		svcName, path := splitItem(svc)
		owner := svcName
		if !strings.HasPrefix(owner, ":") && b.conn != nil {
			var busOwner string
			if err := b.conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus").Call("org.freedesktop.DBus.GetNameOwner", 0, svcName).Store(&busOwner); err == nil && busOwner != "" {
				owner = busOwner
			}
		}
		ownerKey := owner + path
		if seenOwners[ownerKey] {
			continue
		}

		title := b.title(svc)
		if title == "" {
			title = fallbackTitle(svc)
		}
		titleKey := strings.ToLower(strings.TrimSpace(title))
		if titleKey != "" && seenTitles[titleKey] {
			continue
		}

		if q.Text != "" && !containsFold(title, q.Text) {
			continue
		}

		seenOwners[ownerKey] = true
		if titleKey != "" {
			seenTitles[titleKey] = true
		}

		hasMenu := b.menuPath(svc) != ""
		var actions []sdk.Action
		if !hasMenu {
			actions = []sdk.Action{
				{Type: "plugin", Value: "activate"},
			}
		}

		icon, imagePath := b.iconAndImage(svc)
		items = append(items, sdk.Item{
			ID: svc, Title: title, Subtitle: "Tray applet",
			Icon: icon, Image: imagePath, Glyph: "⚙", HasChildren: hasMenu,
			Actions: actions,
		})
	}
	return sdk.NewStream(items...), nil
}

func (b *Backend) Control(context.Context, sdk.Change) error { return nil }

// Invoke executes an action on the applet or its DBusMenu.
func (b *Backend) Invoke(_ context.Context, i sdk.Invoke) error {
	if i.Action == "activate" && b.conn != nil {
		return b.activate(i.Item)
	}
	if i.Action != "invoke-menu" || b.conn == nil {
		return nil
	}
	service, id, ok := parseMenuItemID(i.Item)
	if !ok {
		return nil
	}
	menuPath := b.menuPath(service)
	if menuPath == "" {
		return nil
	}
	svc, _ := splitItem(service)
	obj := b.conn.Object(svc, dbus.ObjectPath(menuPath))
	// Event(id, "clicked", variant(0), timestamp=0)
	call := obj.Call(dbusMenuIface+".Event", 0, id, "clicked", dbus.MakeVariant(0), uint32(0))
	return call.Err
}

func (b *Backend) activate(service string) error {
	svc, path := splitItem(service)
	obj := b.conn.Object(svc, dbus.ObjectPath(path))
	// Try KDE Activate
	if err := obj.Call("org.kde.StatusNotifierItem.Activate", 0, int32(0), int32(0)).Err; err == nil {
		return nil
	}
	// Try ContextMenu
	if err := obj.Call("org.kde.StatusNotifierItem.ContextMenu", 0, int32(0), int32(0)).Err; err == nil {
		return nil
	}
	// Try Ayatana Activate
	if err := obj.Call("org.ayatana.StatusNotifierItem.Activate", 0, int32(0), int32(0)).Err; err == nil {
		return nil
	}
	// Try SecondaryActivate
	return obj.Call("org.kde.StatusNotifierItem.SecondaryActivate", 0, int32(0), int32(0)).Err
}

func fallbackTitle(service string) string {
	svc, _ := splitItem(service)
	for _, prefix := range []string{
		"org.kde.StatusNotifierItem-",
		"org.ayatana.NotificationItem-",
		"org.freedesktop.StatusNotifierItem-",
	} {
		if strings.HasPrefix(svc, prefix) {
			clean := strings.TrimPrefix(svc, prefix)
			if idx := strings.LastIndexByte(clean, '-'); idx > 0 {
				allDigits := true
				for _, c := range clean[idx+1:] {
					if c < '0' || c > '9' {
						allDigits = false
						break
					}
				}
				if allDigits {
					clean = clean[:idx]
				}
			}
			if len(clean) > 0 {
				return strings.ToUpper(clean[:1]) + clean[1:]
			}
		}
	}
	return svc
}

// parseMenuItemID recovers service and menu id from "menu:<service>:<id>".
func parseMenuItemID(item string) (string, int32, bool) {
	rest := item
	if len(rest) < 5 || rest[:5] != "menu:" {
		return "", 0, false
	}
	rest = rest[5:]
	colon := strings.LastIndexByte(rest, ':')
	if colon < 0 {
		return "", 0, false
	}
	service := rest[:colon]
	idPart := rest[colon+1:]
	var id int32
	for _, c := range idPart {
		if c < '0' || c > '9' {
			return "", 0, false
		}
		id = id*10 + int32(c-'0')
	}
	return service, id, true
}

func splitItem(item string) (string, string) {
	idx := strings.IndexByte(item, '/')
	if idx < 0 {
		return item, itemPath
	}
	return item[:idx], item[idx:]
}

func getSNIProperty(obj dbus.BusObject, prop string) (dbus.Variant, error) {
	interfaces := []string{
		"org.kde.StatusNotifierItem",
		"org.ayatana.StatusNotifierItem",
		"org.freedesktop.StatusNotifierItem",
	}
	var lastErr error
	for _, iface := range interfaces {
		var v dbus.Variant
		err := obj.Call("org.freedesktop.DBus.Properties.Get", 0, iface, prop).Store(&v)
		if err == nil {
			return v, nil
		}
		lastErr = err
	}
	return dbus.Variant{}, lastErr
}

// title reads the applet's Title or Id property, best-effort.
func (b *Backend) title(service string) string {
	if b.conn == nil {
		return ""
	}
	svc, path := splitItem(service)
	obj := b.conn.Object(svc, dbus.ObjectPath(path))
	if v, err := getSNIProperty(obj, "Title"); err == nil {
		if s, ok := v.Value().(string); ok && s != "" {
			return s
		}
	}
	if v, err := getSNIProperty(obj, "Id"); err == nil {
		if s, ok := v.Value().(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// iconAndImage resolves icon name or decodes pixmap into an image path.
func (b *Backend) iconAndImage(service string) (icon string, imagePath string) {
	if b.conn == nil {
		return "application-x-executable", ""
	}
	svc, path := splitItem(service)
	obj := b.conn.Object(svc, dbus.ObjectPath(path))

	// 1. Check IconPixmap first (most accurate for Electron / Qt applets).
	if pmVariant, err := getSNIProperty(obj, "IconPixmap"); err == nil {
		pixmaps := extractPixmaps(pmVariant)
		if best := bestPixmap(pixmaps); best != nil {
			img := pixmapToImage(*best)
			if p := savePixmapImage(service, img); p != "" {
				return "", p
			}
		}
	}

	// 2. Check IconName.
	var iconName string
	if nameVariant, err := getSNIProperty(obj, "IconName"); err == nil {
		if s, ok := nameVariant.Value().(string); ok {
			iconName = s
		}
	}

	if iconName != "" {
		if strings.HasPrefix(iconName, "/") && fileExists(iconName) {
			return "", iconName
		}
		if themeVariant, err := getSNIProperty(obj, "IconThemePath"); err == nil {
			if themePath, ok := themeVariant.Value().(string); ok && themePath != "" {
				if full := findIconInPath(themePath, iconName); full != "" {
					return "", full
				}
			}
		}
		return iconName, ""
	}

	return "application-x-executable", ""
}

type rawPixmap struct {
	width  int
	height int
	data   []byte
}

func extractPixmaps(v interface{}) []rawPixmap {
	if v == nil {
		return nil
	}
	if variant, ok := v.(dbus.Variant); ok {
		v = variant.Value()
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	var list []rawPixmap
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i).Interface()
		if variant, ok := elem.(dbus.Variant); ok {
			elem = variant.Value()
		}
		pm := parseOnePixmap(elem)
		if pm.width > 0 && pm.height > 0 && len(pm.data) >= pm.width*pm.height*4 {
			list = append(list, pm)
		}
	}
	return list
}

func parseOnePixmap(elem interface{}) rawPixmap {
	if s, ok := elem.([]interface{}); ok && len(s) >= 3 {
		return rawPixmap{width: toInt(s[0]), height: toInt(s[1]), data: toBytes(s[2])}
	}
	ev := reflect.ValueOf(elem)
	if ev.Kind() == reflect.Struct && ev.NumField() >= 3 {
		return rawPixmap{
			width:  toInt(ev.Field(0).Interface()),
			height: toInt(ev.Field(1).Interface()),
			data:   toBytes(ev.Field(2).Interface()),
		}
	}
	return rawPixmap{}
}

func toInt(v interface{}) int {
	if variant, ok := v.(dbus.Variant); ok {
		v = variant.Value()
	}
	switch n := v.(type) {
	case int32:
		return int(n)
	case int:
		return n
	case uint32:
		return int(n)
	case int64:
		return int(n)
	}
	return 0
}

func toBytes(v interface{}) []byte {
	if variant, ok := v.(dbus.Variant); ok {
		v = variant.Value()
	}
	switch b := v.(type) {
	case []byte:
		return b
	}
	return nil
}

func bestPixmap(list []rawPixmap) *rawPixmap {
	if len(list) == 0 {
		return nil
	}
	best := &list[0]
	for i := range list {
		p := &list[i]
		if best.width < 32 && p.width > best.width {
			best = p
			continue
		}
		if best.width > 64 && p.width >= 32 && p.width < best.width {
			best = p
			continue
		}
		if best.width >= 32 && best.width <= 64 && p.width >= 32 && p.width <= 64 {
			if math.Abs(float64(p.width-48)) < math.Abs(float64(best.width-48)) {
				best = p
			}
		}
	}
	return best
}

func pixmapToImage(pm rawPixmap) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, pm.width, pm.height))
	n := pm.width * pm.height
	for i := 0; i < n; i++ {
		srcIdx := i * 4
		dstIdx := i * 4
		a := pm.data[srcIdx]
		r := pm.data[srcIdx+1]
		g := pm.data[srcIdx+2]
		b := pm.data[srcIdx+3]
		img.Pix[dstIdx] = r
		img.Pix[dstIdx+1] = g
		img.Pix[dstIdx+2] = b
		img.Pix[dstIdx+3] = a
	}
	return img
}

func savePixmapImage(service string, img image.Image) string {
	dir := filepath.Join(os.TempDir(), "keyshell-tray")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return ""
	}
	h := sha256.New()
	if rgba, ok := img.(*image.RGBA); ok {
		h.Write(rgba.Pix)
	}
	hash := hex.EncodeToString(h.Sum(nil))[:12]
	cleanSvc := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, service)
	path := filepath.Join(dir, cleanSvc+"-"+hash+".png")
	if fileExists(path) {
		return path
	}
	f, err := os.Create(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return ""
	}
	return path
}

func findIconInPath(dir, name string) string {
	exts := []string{"", ".png", ".svg", ".xpm"}
	for _, ext := range exts {
		candidate := filepath.Join(dir, name+ext)
		if fileExists(candidate) {
			return candidate
		}
	}
	for _, sub := range []string{"icons", "hicolor/48x48/apps", "hicolor/32x32/apps", "hicolor/scalable/apps"} {
		for _, ext := range exts {
			candidate := filepath.Join(dir, sub, name+ext)
			if fileExists(candidate) {
				return candidate
			}
		}
	}
	return ""
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// menuPath reads the applet's Menu property.
func (b *Backend) menuPath(service string) string {
	if b.conn == nil {
		return ""
	}
	svc, path := splitItem(service)
	obj := b.conn.Object(svc, dbus.ObjectPath(path))
	if v, err := getSNIProperty(obj, "Menu"); err == nil {
		var p string
		if m, ok := v.Value().(dbus.ObjectPath); ok {
			p = string(m)
		} else if s, ok := v.Value().(string); ok {
			p = s
		}
		if p != "" && p != "/" && p != "/NO_DBUSMENU" {
			return p
		}
	}
	// Fallback: check if the object itself or /MenuBar or /StatusNotifierMenu implements com.canonical.dbusmenu
	for _, candidate := range []string{path, "/MenuBar", "/StatusNotifierMenu"} {
		menuObj := b.conn.Object(svc, dbus.ObjectPath(candidate))
		var version uint32
		if err := menuObj.Call("org.freedesktop.DBus.Properties.Get", 0, dbusMenuIface, "Version").Store(&version); err == nil {
			return candidate
		}
	}
	return ""
}

func containsFold(hay, needle string) bool {
	return indexFold(hay, needle) >= 0
}

func indexFold(hay, needle string) int {
	lh := toLower(hay)
	ln := toLower(needle)
	for i := 0; i+len(ln) <= len(lh); i++ {
		if lh[i:i+len(ln)] == ln {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
