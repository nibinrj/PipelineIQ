// Command pipelineiq-server is the PipelineIQ HTTP service.
// Agents upload builds to POST /api/v1/builds. P1 liveness and readiness remain.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/nibinrj/PipelineIQ/internal/api"
	"github.com/nibinrj/PipelineIQ/internal/config"
	"github.com/nibinrj/PipelineIQ/internal/db"
	"github.com/nibinrj/PipelineIQ/internal/httpserver"
	"github.com/nibinrj/PipelineIQ/internal/ingest"
)

func main() {
	if err := run(); err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("exit", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	log := newLogger(os.Stdout, cfg.LogLevel)
	slog.SetDefault(log)

	// NotifyContext is cancelled on SIGTERM (compose stop) and SIGINT (Ctrl+C).
	// defer stop() unregisters the signal handler. Same idea as a Java
	// shutdown hook, but the cancellation is a context the rest of the
	// process already knows how to watch.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.HTTPAddr, err)
	}
	log.Info("listening", "addr", ln.Addr().String(), "database_host", databaseHost(cfg.DatabaseURL))

	ingester := ingest.New(pool)
	handler := httpserver.New(log, db.NewStore(pool), cfg.ReadyTimeout, func(mux *http.ServeMux) {
		api.Register(mux, log, cfg.IngestKey, ingester)
	})
	if err := httpserver.Serve(ctx, ln, handler, cfg.ShutdownTimeout); err != nil {
		return err
	}
	log.Info("stopped")
	return nil
}

func newLogger(w *os.File, level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl}))
}

func databaseHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "unparsed"
	}
	if u.Host != "" {
		return u.Host
	}
	// libpq socket URLs put the directory in the host query, not the URL host.
	if host := u.Query().Get("host"); host != "" {
		return host
	}
	return "unparsed"
}
