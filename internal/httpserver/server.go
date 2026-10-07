// Package httpserver is the net/http front of pipelineiq-server.
// Liveness and readiness live here. Ingest routes are registered by the caller.
package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Pinger is the only database method this package needs. An interface here is
// the same reason a Java test uses a fake: the handler tests must not open Postgres.
type Pinger interface {
	Ping(ctx context.Context) error
}

// New returns the request router. register, if set, adds routes such as ingest.
// Method and path are part of the pattern, so POST /healthz is rejected by the
// mux with 405. That is the Go 1.22 ServeMux, not a framework.
func New(log *slog.Logger, pinger Pinger, readyTimeout time.Duration, register ...func(*http.ServeMux)) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	if readyTimeout <= 0 {
		readyTimeout = 2 * time.Second
	}
	s := &handler{log: log, pinger: pinger, readyTimeout: readyTimeout}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	for _, fn := range register {
		if fn != nil {
			fn(mux)
		}
	}
	return mux
}

type handler struct {
	log          *slog.Logger
	pinger       Pinger
	readyTimeout time.Duration
}

func (h *handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.readyTimeout)
	defer cancel()
	if err := h.pinger.Ping(ctx); err != nil {
		// Log the cause. Do not put the driver error in the response: it can
		// include a host name or a statement.
		h.log.Error("readiness check failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// The status is already sent. Nothing useful to write after that.
		return
	}
}

// Serve accepts on ln until ctx is cancelled, then shuts down within
// shutdownTimeout. SIGTERM handling lives in main; this function is what it calls.
func Serve(ctx context.Context, ln net.Listener, h http.Handler, shutdownTimeout time.Duration) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return <-errCh
	}
}
