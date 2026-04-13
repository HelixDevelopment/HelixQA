// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package visionserver

import (
	"context"
	"fmt"
	"net/http"
)

// Server wraps a standard net/http server and wires the Handler's methods to
// well-known URL paths. It is safe to call Start and Stop from different
// goroutines.
type Server struct {
	httpServer *http.Server
	handler    *Handler
}

// NewServer creates a Server that listens on config.ListenAddr and routes
// requests to handler. The HTTP server is fully initialised but not yet
// started; call Start to begin accepting connections.
func NewServer(config *Config, handler *Handler) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/analyze", handler.HandleAnalyze)
	mux.HandleFunc("/providers", handler.HandleListProviders)
	mux.HandleFunc("/health", handler.HandleHealth)
	mux.HandleFunc("/learning/stats", handler.HandleLearningStats)
	mux.HandleFunc("/learning/clear", handler.HandleClearLearning)

	return &Server{
		httpServer: &http.Server{
			Addr:    config.ListenAddr,
			Handler: mux,
		},
		handler: handler,
	}
}

// Start begins accepting HTTP connections in a background goroutine. It
// returns an error immediately if the listener cannot be bound; otherwise it
// returns nil and the server runs until Stop is called.
func (s *Server) Start() error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Give the listener a brief moment to bind. If it fails immediately
	// (e.g. address already in use) we return the error to the caller.
	select {
	case err, ok := <-errCh:
		if ok && err != nil {
			return fmt.Errorf("visionserver: listen failed: %w", err)
		}
	default:
		// No immediate error — server is running in the background.
	}
	return nil
}

// Stop performs a graceful shutdown of the HTTP server. Ongoing requests are
// given the duration of ctx to complete before connections are forcibly closed.
func (s *Server) Stop(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("visionserver: shutdown failed: %w", err)
	}
	return nil
}
