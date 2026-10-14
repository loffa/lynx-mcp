package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"lynx-mcp/internal/config"
)

// Server exposes the Lynx service through MCP over HTTP.
type Server struct {
	Service mcpService
}

// Open serves HTTP requests until the context is canceled or the server fails.
func (s *Server) Open(ctx context.Context, cfg config.HTTPConfig) error {
	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           s.routes(),
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	shutdown := make(chan error, 1)

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			if closeErr := httpServer.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("failed to close HTTP server: %w", closeErr))
			}

			shutdown <- fmt.Errorf("failed to shut down HTTP server: %w", err)

			return
		}

		shutdown <- nil
	}()

	err := httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return <-shutdown
	}

	return fmt.Errorf("failed to start HTTP server connections: %w", err)
}

func (s *Server) routes() http.Handler {
	routes := chi.NewRouter()
	routes.Use(middleware.Logger)

	routes.Handle("/mcp", s.mcpHandler())

	return routes
}
