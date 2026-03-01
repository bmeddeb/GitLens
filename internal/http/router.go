package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/user/gitlens-pro/internal/config"
	"go.uber.org/zap"
)

func NewRouter(cfg *config.AppConfig, logger *zap.Logger) *chi.Mux {
	r := chi.NewRouter()

	rateLimiter := NewRateLimiter(cfg.Access.RateLimitPerMin)

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(RequestLogger(logger))
	r.Use(Recoverer(logger))
	r.Use(rateLimiter.Middleware)

	return r
}

type Server struct {
	server *http.Server
	logger *zap.Logger
}

func NewServer(cfg *config.AppConfig, router *chi.Mux, logger *zap.Logger) *Server {
	return &Server{
		server: &http.Server{
			Addr:         cfg.Server.Addr(),
			Handler:      router,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			BaseContext: func(l net.Listener) context.Context {
				return context.Background()
			},
		},
		logger: logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", zap.String("addr", s.server.Addr))
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return s.server.Shutdown(shutdownCtx)
}
