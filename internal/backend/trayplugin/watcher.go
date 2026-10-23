// Package trayplugin is the tray backend: a StatusNotifierItem host that lists
// applets as cards and (in a follow-on) exposes their DBusMenu as scoped child
// levels. The plugin owns the D-Bus role; the host renders schema-driven items.
package trayplugin

import (
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

// watcherPath is where the watcher object is exported.
const watcherPath = "/StatusNotifierWatcher"

// Watcher is the org.kde.StatusNotifierWatcher implementation.
type Watcher struct {
	conn *dbus.Conn

	mu        sync.Mutex
	items     []string
	isWatcher bool
}

// NewWatcher binds on the given session connection.
func NewWatcher(conn *dbus.Conn) *Watcher {
	return &Watcher{conn: conn}
}

// Serve claims the watcher role or binds as a host if another watcher exists.
func (w *Watcher) Serve() error {
	reply, err := w.conn.RequestName("org.kde.StatusNotifierWatcher", dbus.NameFlagDoNotQueue)
	if err == nil && reply == dbus.RequestNameReplyPrimaryOwner {
		w.isWatcher = true
		// We are the Watcher.
		if err := w.conn.Export(w, dbus.ObjectPath(watcherPath), "org.kde.StatusNotifierWatcher"); err != nil {
			return err
		}
		props := &watcherProperties{w: w}
		if err := w.conn.Export(props, dbus.ObjectPath(watcherPath), "org.freedesktop.DBus.Properties"); err != nil {
			return err
		}

		w.setupSignalListeners()
		w.scanBusForApplets()

		_ = w.conn.Emit(dbus.ObjectPath(watcherPath), "org.kde.StatusNotifierWatcher.StatusNotifierHostRegistered")
		return nil
	}

	// Another watcher is already running. We act as a StatusNotifierHost.
	w.setupSignalListeners()
	w.Refresh()
	return nil
}

// Refresh re-checks the existing watcher and scans the bus for applets.
func (w *Watcher) Refresh() {
	if w.conn == nil {
		return
	}
	hostObj := w.conn.Object("org.kde.StatusNotifierWatcher", dbus.ObjectPath(watcherPath))

	// Register ourselves as a host (best-effort).
	names := w.conn.Names()
	if len(names) > 0 {
		_ = hostObj.Call("org.kde.StatusNotifierWatcher.RegisterStatusNotifierHost", 0, names[0]).Err
	}

	// Fetch existing registered items from the watcher.
	registered := getRegisteredItems(hostObj)
	for _, item := range registered {
		w.add(item)
	}

	// Only scan session bus directly if no items were returned from the watcher (or we are the watcher).
	if len(registered) == 0 || w.isWatcher {
		w.scanBusForApplets()
	}
}

func getRegisteredItems(hostObj dbus.BusObject) []string {
	var variant dbus.Variant
	// Official KDE property
	if err := hostObj.Call("org.freedesktop.DBus.Properties.Get", 0, "org.kde.StatusNotifierWatcher", "RegisteredStatusNotifierItems").Store(&variant); err == nil {
		if list := toStringSlice(variant.Value()); len(list) > 0 {
			return list
		}
	}
	// Ayatana property
	if err := hostObj.Call("org.freedesktop.DBus.Properties.Get", 0, "org.ayatana.StatusNotifierWatcher", "RegisteredStatusNotifierItems").Store(&variant); err == nil {
		if list := toStringSlice(variant.Value()); len(list) > 0 {
			return list
		}
	}
	// Freedesktop property
	if err := hostObj.Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.StatusNotifierWatcher", "RegisteredStatusNotifierItems").Store(&variant); err == nil {
		if list := toStringSlice(variant.Value()); len(list) > 0 {
			return list
		}
	}
	// Fallback method call
	var methodList []string
	if err := hostObj.Call("org.kde.StatusNotifierWatcher.GetRegisteredStatusNotifierItems", 0).Store(&methodList); err == nil {
		return methodList
	}
	return nil
}

func toStringSlice(val interface{}) []string {
	if val == nil {
		return nil
	}
	switch s := val.(type) {
	case []string:
		return s
	case []interface{}:
		res := make([]string, 0, len(s))
		for _, v := range s {
			if str, ok := v.(string); ok {
				res = append(res, str)
			}
		}
		return res
	}
	return nil
}

func (w *Watcher) scanBusForApplets() {
	if w.conn == nil {
		return
	}
	dbusObj := w.conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	var names []string
	if err := dbusObj.Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err == nil {
		for _, name := range names {
			if isSNIService(name) {
				w.add(name)
			}
		}
	}
}

func isSNIService(name string) bool {
	return strings.HasPrefix(name, "org.kde.StatusNotifierItem-") ||
		strings.HasPrefix(name, "org.ayatana.NotificationItem-") ||
		strings.HasPrefix(name, "org.freedesktop.StatusNotifierItem-")
}

func (w *Watcher) setupSignalListeners() {
	_ = w.conn.AddMatchSignal(dbus.WithMatchInterface("org.kde.StatusNotifierWatcher"))
	_ = w.conn.AddMatchSignal(dbus.WithMatchInterface("org.ayatana.StatusNotifierWatcher"))
	_ = w.conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.StatusNotifierWatcher"))
	_ = w.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
	)
	ch := make(chan *dbus.Signal, 32)
	w.conn.Signal(ch)
	go w.listen(ch)
}

func (w *Watcher) listen(ch <-chan *dbus.Signal) {
	for sig := range ch {
		switch {
		case strings.HasSuffix(sig.Name, ".StatusNotifierItemRegistered"):
			if len(sig.Body) > 0 {
				if service, ok := sig.Body[0].(string); ok {
					if strings.HasPrefix(service, "/") && sig.Sender != "" {
						service = sig.Sender + service
					}
					w.add(service)
				}
			}
		case strings.HasSuffix(sig.Name, ".StatusNotifierItemUnregistered"):
			if len(sig.Body) > 0 {
				if service, ok := sig.Body[0].(string); ok {
					if strings.HasPrefix(service, "/") && sig.Sender != "" {
						service = sig.Sender + service
					}
					w.remove(service)
				}
			}
		case sig.Name == "org.freedesktop.DBus.NameOwnerChanged":
			if len(sig.Body) >= 3 {
				name, ok1 := sig.Body[0].(string)
				oldOwner, ok2 := sig.Body[1].(string)
				newOwner, ok3 := sig.Body[2].(string)
				if ok1 && ok2 && ok3 && isSNIService(name) {
					if newOwner != "" && oldOwner == "" {
						w.add(name)
					} else if newOwner == "" {
						w.remove(name)
					}
				}
			}
		}
	}
}

func (w *Watcher) add(service string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, s := range w.items {
		if s == service {
			return
		}
	}
	w.items = append(w.items, service)
}

func (w *Watcher) remove(service string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := w.items[:0]
	for _, s := range w.items {
		if s != service {
			out = append(out, s)
		}
	}
	w.items = out
}

// Items returns the registered applet service names.
func (w *Watcher) Items() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, len(w.items))
	copy(out, w.items)
	return out
}

// --- Exported D-Bus methods (must have dbus-specific signatures). ---

// GetRegisteredStatusNotifierItems returns the known item service names.
func (w *Watcher) GetRegisteredStatusNotifierItems() ([]string, *dbus.Error) {
	return w.Items(), nil
}

// IsStatusNotifierHostRegistered reports whether we are registered as a host.
func (w *Watcher) IsStatusNotifierHostRegistered() (bool, *dbus.Error) {
	return true, nil
}

// ProtocolVersion is the SNI protocol version we speak.
func (w *Watcher) ProtocolVersion() (int32, *dbus.Error) {
	return 0, nil
}

// RegisterStatusNotifierItem is called by applets to register.
func (w *Watcher) RegisterStatusNotifierItem(service string, sender dbus.Sender) *dbus.Error {
	item := service
	if strings.HasPrefix(item, "/") {
		item = string(sender) + item
	}
	w.add(item)
	if err := emitRegistered(w.conn, item); err != nil {
		return &dbus.Error{Name: "org.freedesktop.DBus.Error.Failed", Body: []interface{}{err.Error()}}
	}
	return nil
}

// RegisterStatusNotifierHost is called by hosts to register with the watcher.
func (w *Watcher) RegisterStatusNotifierHost(service string) *dbus.Error {
	if w.conn != nil {
		_ = w.conn.Emit(dbus.ObjectPath(watcherPath), "org.kde.StatusNotifierWatcher.StatusNotifierHostRegistered")
	}
	return nil
}

func emitRegistered(conn *dbus.Conn, service string) error {
	return conn.Emit(dbus.ObjectPath(watcherPath), "org.kde.StatusNotifierWatcher.StatusNotifierItemRegistered", service)
}

type watcherProperties struct {
	w *Watcher
}

func (p *watcherProperties) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
	switch prop {
	case "RegisteredStatusNotifierItems":
		return dbus.MakeVariant(p.w.Items()), nil
	case "IsStatusNotifierHostRegistered":
		return dbus.MakeVariant(true), nil
	case "ProtocolVersion":
		return dbus.MakeVariant(int32(0)), nil
	}
	return dbus.Variant{}, dbus.NewError("org.freedesktop.DBus.Error.UnknownProperty", []interface{}{prop})
}

func (p *watcherProperties) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	return map[string]dbus.Variant{
		"RegisteredStatusNotifierItems":  dbus.MakeVariant(p.w.Items()),
		"IsStatusNotifierHostRegistered": dbus.MakeVariant(true),
		"ProtocolVersion":                dbus.MakeVariant(int32(0)),
	}, nil
}

func (p *watcherProperties) Set(iface, prop string, val dbus.Variant) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.PropertyReadOnly", []interface{}{prop})
}
