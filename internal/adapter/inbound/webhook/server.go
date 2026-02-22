package webhook

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jonny/opsai-bot/internal/adapter/inbound/webhook/middleware"
)

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// Server wraps an HTTP server with graceful shutdown support.
type Server struct {
	cfg     ServerConfig
	handler *Handler
	logger  *log.Logger
	srv     *http.Server
}

// NewServer creates a new Server with the given config and webhook handler.
func NewServer(cfg ServerConfig, handler *Handler) *Server {
	return &Server{
		cfg:     cfg,
		handler: handler,
		logger:  log.Default(),
	}
}

// NewServerWithLogger creates a new Server with a custom logger.
func NewServerWithLogger(cfg ServerConfig, handler *Handler, logger *log.Logger) *Server {
	return &Server{
		cfg:     cfg,
		handler: handler,
		logger:  logger,
	}
}

// SetupRoutes builds and returns an http.Handler with all middleware applied.
// Route layout:
//
//	GET  /health        - Health check
//	POST /webhook       - Main webhook receiver
func (s *Server) SetupRoutes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", HealthHandler())
	mux.Handle("/webhook", s.handler)

	// Apply middleware stack (outermost = first to execute):
	//   BodyReader -> Logging -> RateLimit
	var h http.Handler = mux
	h = middleware.NewRateLimiter(120)(h)
	h = middleware.NewLoggingMiddleware(s.logger)(h)
	h = middleware.BodyReader(h)

	return h
}

// Start starts the HTTP server and blocks until ctx is cancelled, then performs
// a graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	s.srv = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.Port),
		Handler:      s.SetupRoutes(),
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Printf("webhook server listening on :%d", s.cfg.Port)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("webhook server shutdown error: %w", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}
