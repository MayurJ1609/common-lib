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
