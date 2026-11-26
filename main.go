// Command hub is the buunk-hub server: one binary that serves the API, the
// start page and the built frontend.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
	"github.com/jurrebuunk/buunk-hub/internal/catalogue"
	"github.com/jurrebuunk/buunk-hub/internal/config"
	"github.com/jurrebuunk/buunk-hub/internal/db"
	"github.com/jurrebuunk/buunk-hub/internal/favicon"
	"github.com/jurrebuunk/buunk-hub/internal/kcadmin"
	"github.com/jurrebuunk/buunk-hub/internal/links"
	"github.com/jurrebuunk/buunk-hub/internal/objects"
	"github.com/jurrebuunk/buunk-hub/internal/pages"
	"github.com/jurrebuunk/buunk-hub/internal/picture"
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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	apps, err := catalogue.Load(cfg.Catalogue)
	if err != nil {
		return err
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	a, err := auth.New(ctx, auth.Config{
		PublicURL:    cfg.PublicURL,
		Issuer:       cfg.OIDCIssuer,
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		SessionKey:   cfg.SessionKey,
		Store:        auth.PGStore{DB: pool},
		Log:          log,
	})
	if err != nil {
		return err
	}
	go a.Cleanup(ctx)

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: server.New(server.Deps{
			Auth:  a,
			Apps:  apps,
			Icons: favicon.NewCache(log),
			Pictures: &picture.Service{
				Objects:   objects.New(cfg.S3Endpoint, cfg.S3Bucket, cfg.S3AccessKey, cfg.S3SecretKey),
				Keycloak:  kcadmin.New(ctx, cfg.KeycloakAdminURL, cfg.OIDCIssuer+"/protocol/openid-connect/token", cfg.OIDCClientID, cfg.OIDCClientSecret),
				Users:     auth.PGStore{DB: pool},
				PublicURL: cfg.PublicURL,
				Log:       log,
			},
			Links:    links.Store{DB: pool},
			Pages:    pages.Store{DB: pool},
			Framing:  pages.NewFraming(cfg.PublicURL, log),
			Hub:      cfg.PublicURL,
			Frontend: web.Dist(),
			Log:      log,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
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
