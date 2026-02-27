package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DoskoiYuta/portless-docker/internal/state"
)

const (
	// SelfCheckInterval is how often the proxy checks for remaining routes.
	SelfCheckInterval = 30 * time.Second
)

// Server is the portless-docker reverse proxy server.
type Server struct {
	port         int
	stateManager *state.Manager
	httpServer   *http.Server
}

// NewServer creates a new proxy server.
func NewServer(port int, sm *state.Manager) *Server {
	handler := NewHandler(sm)
	return &Server{
		port:         port,
		stateManager: sm,
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      handler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}
}

// Run starts the proxy server and blocks until shutdown.
func (s *Server) Run() error {
	// Set up signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start self-check goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.selfCheck(ctx)

	// Start serving.
	errCh := make(chan error, 1)
	go func() {
		log.Printf("Proxy listening on :%d", s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for signal or error.
	select {
	case sig := <-sigCh:
		log.Printf("Received signal %v, shutting down", sig)
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	// Graceful shutdown.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return s.httpServer.Shutdown(shutdownCtx)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// selfCheck periodically checks if there are remaining routes.
// If no routes remain, the server shuts down.
func (s *Server) selfCheck(ctx context.Context) {
	ticker := time.NewTicker(SelfCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			has, err := s.stateManager.HasRoutes()
			if err != nil {
				log.Printf("Self-check error: %v", err)
				continue
			}
			if !has {
				log.Println("No routes remaining. Proxy shutting down.")
				s.Shutdown()
				return
			}
		}
	}
}
