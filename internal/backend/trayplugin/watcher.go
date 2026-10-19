// Package trayplugin is the tray backend: a StatusNotifierItem host that lists
// applets as cards and (in a follow-on) exposes their DBusMenu as scoped child
// levels. The plugin owns the D-Bus role; the host renders schema-driven items.
package trayplugin

import (
	"sync"

	"github.com/godbus/dbus/v5"
)

// watcherPath is where the watcher object is exported.
const watcherPath = "/StatusNotifierWatcher"

// Watcher is the org.kde.StatusNotifierWatcher implementation.
type Watcher struct {
	conn *dbus.Conn

	mu    sync.Mutex
	items []string
}

// NewWatcher binds on the given session connection.
func NewWatcher(conn *dbus.Conn) *Watcher {
	return &Watcher{conn: conn}
}

// Serve requests the watcher name, exports the object, and starts listening for
// item (un)registration. It is best-effort: any failure leaves an empty tray.
func (w *Watcher) Serve() error {
	if _, err := w.conn.RequestName("org.kde.StatusNotifierWatcher", dbus.NameFlagDoNotQueue); err != nil {
		return err
	}
	if err := w.conn.Export(w, dbus.ObjectPath(watcherPath), "org.kde.StatusNotifierWatcher"); err != nil {
		return err
	}
	if err := w.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.kde.StatusNotifierWatcher"),
	); err != nil {
		return err
	}
	ch := make(chan *dbus.Signal, 16)
	w.conn.Signal(ch)
	go w.listen(ch)
	return nil
}

func (w *Watcher) listen(ch <-chan *dbus.Signal) {
	for sig := range ch {
		if len(sig.Body) == 0 {
			continue
		}
		service, ok := sig.Body[0].(string)
		if !ok {
			continue
		}
		switch sig.Name {
		case "org.kde.StatusNotifierWatcher.StatusNotifierItemRegistered":
			w.add(service)
		case "org.kde.StatusNotifierWatcher.StatusNotifierItemUnregistered":
			w.remove(service)
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
func (w *Watcher) RegisterStatusNotifierItem(service string) *dbus.Error {
	w.add(service)
	if err := emitRegistered(w.conn, service); err != nil {
		return &dbus.Error{Name: "org.freedesktop.DBus.Error.Failed", Body: []interface{}{err.Error()}}
	}
	return nil
}

func emitRegistered(conn *dbus.Conn, service string) error {
	return conn.Emit(dbus.ObjectPath(watcherPath), "org.kde.StatusNotifierWatcher.StatusNotifierItemRegistered", service)
}
