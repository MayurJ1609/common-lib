package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

func TestRegistryObservesHTTPAndServesMetrics(t *testing.T) {
	registry, err := NewRegistry("orders")
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	registry.ObserveHTTP(http.MethodPost, "/orders", http.StatusCreated, 250*time.Millisecond)
	recorder := httptest.NewRecorder()
	registry.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatalf("read metrics response: %v", err)
	}
	for _, want := range []string{
		`orders_http_requests_total{method="POST",route="/orders",status="201"} 1`,
		`orders_http_request_duration_seconds_count{method="POST",route="/orders",status="201"} 1`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("metrics output does not contain %q", want)
		}
	}
}

func TestRegisterPostgresPool(t *testing.T) {
	if err := RegisterPostgresPool(nil, nil); err == nil {
		t.Fatal("RegisterPostgresPool(nil, nil) should fail")
	}

	config, err := pgxpool.ParseConfig("postgres://user:pass@127.0.0.1:5432/test")
	if err != nil {
		t.Fatalf("parse PostgreSQL config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	t.Cleanup(pool.Close)

	registry := prometheus.NewRegistry()
	if err := RegisterPostgresPool(registry, pool); err != nil {
		t.Fatalf("RegisterPostgresPool() error = %v", err)
	}
	if err := RegisterPostgresPool(registry, pool); err == nil {
		t.Fatal("registering the same PostgreSQL collector twice should fail")
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather PostgreSQL metrics: %v", err)
	}
	if len(families) != 7 {
		t.Fatalf("got %d PostgreSQL metric families, want 7", len(families))
	}
}
