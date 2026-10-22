// Package debugsrv serves pprof profiles and expvar runtime metrics over a
// local HTTP endpoint for development and profiling.
package debugsrv

import (
	"errors"
	_ "expvar" // registers /debug/vars on DefaultServeMux
	"log/slog"
	"net/http"
	_ "net/http/pprof" // registers the pprof handlers on DefaultServeMux
	"time"
)

// Serve starts the debug endpoint on addr. It returns immediately; a non-nil
// error means the listener could not be opened. Close the returned server to
// stop it.
func Serve(addr string, lg *slog.Logger) (*http.Server, error) {
	if addr == "" {
		return nil, errors.New("debug server: empty address")
	}
	srv := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			lg.Error("debug server failed", "err", err)
		}
	}()
	lg.Info("debug server listening", "addr", addr)
	return srv, nil
}
