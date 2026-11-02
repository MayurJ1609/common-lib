package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MayurJ1609/common-lib/health"
	"github.com/MayurJ1609/common-lib/httpx"
	"github.com/MayurJ1609/common-lib/logger"
	"github.com/MayurJ1609/common-lib/metrics"
	"github.com/MayurJ1609/common-lib/server"
	"github.com/MayurJ1609/common-lib/tracing"
)

func main() {
	log := logger.New(logger.Options{Service: "service-template", Level: "info"})
	metricRegistry, err := metrics.NewRegistry("service")
	if err != nil {
		log.Error("initialize metrics", "error", err)
		os.Exit(1)
	}
	traceProvider, err := tracing.New(context.Background(), tracing.Options{
		ServiceName:    "service-template",
		ServiceVersion: "dev",
		Endpoint:       envOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		Insecure:       true,
	})
	if err != nil {
		log.Error("initialize tracing", "error", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	health.New(nil, 3*time.Second).Register(mux)
	mux.Handle("GET /metrics", metricRegistry.Handler())
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("service-template is running\n"))
	})
	handler := httpx.Chain(
		mux,
		httpx.RequestIDMiddleware,
		httpx.LoggingMiddleware(log),
		httpx.MetricsMiddleware(metricRegistry),
		httpx.RecoverMiddleware(log),
	)
	httpServer := &http.Server{
		Addr:              envOrDefault("HTTP_ADDR", ":8080"),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runErr := server.Server{HTTP: httpServer, ShutdownTimeout: 10 * time.Second}.Run(ctx)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := traceProvider.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown tracing", "error", err)
	}
	if runErr != nil && !errors.Is(runErr, http.ErrServerClosed) {
		log.Error("run HTTP server", "error", runErr)
		os.Exit(1)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
