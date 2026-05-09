// Command oauth-ms is the connector entry point.
//
// Loads config -> builds the container -> starts the HTTP server -> waits for
// SIGINT/SIGTERM and shuts down cleanly.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/OmerSorrell/oauth-ms/cmd/dependencies"
	"github.com/OmerSorrell/oauth-ms/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	rootCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	_, c, err := dependencies.NewContainerProvider().Provide(rootCtx, cfg)
	if err != nil {
		return err
	}
	defer c.Close()

	serveErr := make(chan error, 1)
	go func() {
		c.Logger.Info("listening", "addr", c.Server.Addr, "base_url", cfg.Server.BaseURL)
		if err := c.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case <-rootCtx.Done():
		c.Logger.Info("shutdown signal received")
	case err := <-serveErr:
		return err
	}

	shutdownCtx, scancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer scancel()
	return c.Server.Shutdown(shutdownCtx)
}
