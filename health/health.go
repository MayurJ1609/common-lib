package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Check func(context.Context) error

type Handler struct {
	checks  map[string]Check
	timeout time.Duration
}

func New(checks map[string]Check, timeout time.Duration) *Handler {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &Handler{checks: checks, timeout: timeout}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.Liveness)
	mux.HandleFunc("GET /readyz", h.Readiness)
}

func (h *Handler) Liveness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	failures := make(map[string]string)
	for name, check := range h.checks {
		if err := check(ctx); err != nil {
			failures[name] = err.Error()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if len(failures) > 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "not_ready", "checks": failures})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}
