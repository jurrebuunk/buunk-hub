// Command hub is the buunk-hub server: one binary that serves the API, the
// start page and the built frontend.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jurrebuunk/buunk-hub/internal/config"
	"github.com/jurrebuunk/buunk-hub/internal/server"
	"github.com/jurrebuunk/buunk-hub/web"
)

// version is set at build time from VERSION (see Dockerfile).
var version = "dev"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("exit", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.New(web.Dist()),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.Addr, "version", version)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	// Kubernetes sends SIGTERM and waits 30 s; finish open requests first.
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
