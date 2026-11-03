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
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

func TestLoggingMiddlewareAndResponseWriter(t *testing.T) {
	var output strings.Builder
	log := slog.New(slog.NewTextHandler(&output, nil))
	handler := LoggingMiddleware(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/items", nil))

	if recorder.Code != http.StatusOK || recorder.Body.String() != "ok" {
		t.Fatalf("response = %d %q, want 200 ok", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(output.String(), "status=200") {
		t.Fatalf("log output %q does not contain default status", output.String())
	}

	writer := &statusWriter{ResponseWriter: httptest.NewRecorder()}
	if got, err := writer.Write([]byte("body")); err != nil || got != len("body") {
		t.Fatalf("Write() = %d, %v; want %d, nil", got, err, len("body"))
	}
	if writer.status != http.StatusOK {
		t.Fatalf("implicit status = %d, want %d", writer.status, http.StatusOK)
	}
	if writer.Unwrap() != writer.ResponseWriter {
		t.Fatal("Unwrap() did not return the underlying response writer")
	}
}

func TestRequestIDValidationAndRouteName(t *testing.T) {
	for _, id := range []string{"", "contains space", strings.Repeat("x", 129), "line\nbreak"} {
		if validRequestID(id) {
			t.Errorf("validRequestID(%q) = true, want false", id)
		}
	}
	for _, id := range []string{"id", strings.Repeat("x", 128)} {
		if !validRequestID(id) {
			t.Errorf("validRequestID(%q) = false, want true", id)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	if got := routeName(request); got != "GET unknown" {
		t.Fatalf("routeName() = %q, want GET unknown", got)
	}
	request.Pattern = "GET /items/{id}"
	if got := routeName(request); got != "GET /items/{id}" {
		t.Fatalf("routeName() = %q, want registered route", got)
	}
}

func TestRequestIDMiddlewareReplacesInvalidID(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestID(r.Context()); got == "" || got == "invalid id" {
			t.Errorf("generated request ID = %q, want non-empty valid ID", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", "invalid id")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Request-ID"); got == "" || got == "invalid id" {
		t.Fatalf("response request ID = %q, want generated ID", got)
	}
}

func TestTracingMiddlewareMarksServerErrors(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown tracer provider: %v", err)
		}
	})

	handler := TracingMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	request := httptest.NewRequest(http.MethodGet, "/items", nil)
	request.Pattern = "GET /items"
	handler.ServeHTTP(httptest.NewRecorder(), request)

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d ended spans, want 1", len(spans))
	}
	if spans[0].Status().Code != codes.Error {
		t.Fatalf("span status = %v, want error", spans[0].Status().Code)
	}
}
