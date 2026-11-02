package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Server struct {
	HTTP            *http.Server
	ShutdownTimeout time.Duration
}

func (s Server) Run(ctx context.Context) error {
	if s.HTTP == nil {
		return fmt.Errorf("HTTP server is required")
	}
	listener, err := net.Listen("tcp", s.HTTP.Addr)
	if err != nil {
		return fmt.Errorf("listen on %q: %w", s.HTTP.Addr, err)
	}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.HTTP.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		timeout := s.ShutdownTimeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := s.HTTP.Shutdown(shutdownCtx); err != nil {
			closeErr := s.HTTP.Close()
			return errors.Join(fmt.Errorf("shutdown HTTP server: %w", err), closeErr)
		}
		err := <-serveErr
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP during shutdown: %w", err)
		}
		return nil
	}
}
