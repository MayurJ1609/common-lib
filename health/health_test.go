package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadinessReportsFailedChecks(t *testing.T) {
	handler := New(map[string]Check{
		"database": func(context.Context) error { return errors.New("unavailable") },
	}, time.Second)
	recorder := httptest.NewRecorder()

	handler.Readiness(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if got := recorder.Body.String(); got == "" {
		t.Fatal("readiness response body is empty")
	}
}

func TestHealthEndpoints(t *testing.T) {
	handler := New(map[string]Check{}, 0)
	mux := http.NewServeMux()
	handler.Register(mux)

	liveness := httptest.NewRecorder()
	mux.ServeHTTP(liveness, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if liveness.Code != http.StatusNoContent {
		t.Fatalf("liveness status = %d, want %d", liveness.Code, http.StatusNoContent)
	}

	readiness := httptest.NewRecorder()
	mux.ServeHTTP(readiness, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if readiness.Code != http.StatusOK || readiness.Body.String() != "{\"status\":\"ready\"}\n" {
		t.Fatalf("readiness response = %d %q, want ready response", readiness.Code, readiness.Body.String())
	}
}

func TestReadinessRunsAllChecks(t *testing.T) {
	var calls []string
	handler := New(map[string]Check{
		"first": func(context.Context) error { calls = append(calls, "first"); return nil },
		"second": func(context.Context) error {
			calls = append(calls, "second")
			return errors.New("unavailable")
		},
	}, time.Second)
	recorder := httptest.NewRecorder()
	handler.Readiness(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if len(calls) != 2 {
		t.Fatalf("got %d check calls, want 2", len(calls))
	}
}
