package httpx

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MayurJ1609/common-lib/metrics"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestRequestIDAndMetricsMiddleware(t *testing.T) {
	registry, err := metrics.NewRegistry("test")
	if err != nil {
		t.Fatalf("create metrics registry: %v", err)
	}
	handler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := RequestID(r.Context()); got != "request-123" {
				t.Errorf("request ID = %q, want request-123", got)
			}
			w.WriteHeader(http.StatusAccepted)
		}),
		RequestIDMiddleware,
		MetricsMiddleware(registry),
	)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", "request-123")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Request-ID"); got != "request-123" {
		t.Fatalf("response request ID = %q, want request-123", got)
	}
	if got := recorder.Code; got != http.StatusAccepted {
		t.Fatalf("response status = %d, want %d", got, http.StatusAccepted)
	}
	response := httptest.NewRecorder()
	registry.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, err := io.ReadAll(response.Result().Body)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	if !strings.Contains(string(body), `test_http_requests_total{method="GET",route="GET unknown",status="202"} 1`) {
		t.Fatalf("metrics output does not contain observed request: %s", body)
	}
}

func TestTracingMiddlewareCreatesServerSpan(t *testing.T) {
	previousProvider := otel.GetTracerProvider()
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown tracer provider: %v", err)
		}
	})

	var spanValid bool
	handler := TracingMiddleware()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		spanValid = trace.SpanFromContext(r.Context()).SpanContext().IsValid()
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items", nil))

	if !spanValid {
		t.Fatal("request did not receive an OpenTelemetry span")
	}
}

func TestRecoverMiddleware(t *testing.T) {
	handler := Chain(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("test panic") }),
		RecoverMiddleware(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := recorder.Code; got != http.StatusInternalServerError {
		t.Fatalf("response status = %d, want %d", got, http.StatusInternalServerError)
	}
}
