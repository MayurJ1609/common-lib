package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestNewRequiresServiceName(t *testing.T) {
	if _, err := New(context.Background(), Options{}); err == nil {
		t.Fatal("New() accepted an empty service name")
	}
}

func TestNewProviderAndShutdown(t *testing.T) {
	previous := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(previous) })

	provider, err := New(context.Background(), Options{
		ServiceName:    "orders",
		ServiceVersion: "1.2.3",
		Endpoint:       "localhost:4317",
		Insecure:       true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
