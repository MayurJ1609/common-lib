package metrics

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Registry struct {
	Prometheus prometheus.Registerer
	Gatherer   prometheus.Gatherer
	requests   *prometheus.CounterVec
	duration   *prometheus.HistogramVec
}

func NewRegistry(namespace string) (*Registry, error) {
	reg := prometheus.NewRegistry()
	if err := reg.Register(collectors.NewGoCollector()); err != nil {
		return nil, fmt.Errorf("register Go collector: %w", err)
	}
	if err := reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		return nil, fmt.Errorf("register process collector: %w", err)
	}
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "http_requests_total",
		Help:      "Total number of completed HTTP requests.",
	}, []string{"method", "route", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request duration in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route", "status"})
	if err := reg.Register(requests); err != nil {
		return nil, fmt.Errorf("register HTTP request counter: %w", err)
	}
	if err := reg.Register(duration); err != nil {
		return nil, fmt.Errorf("register HTTP duration histogram: %w", err)
	}
	return &Registry{Prometheus: reg, Gatherer: reg, requests: requests, duration: duration}, nil
}

func (r *Registry) ObserveHTTP(method, route string, status int, elapsed time.Duration) {
	statusLabel := strconv.Itoa(status)
	r.requests.WithLabelValues(method, route, statusLabel).Inc()
	r.duration.WithLabelValues(method, route, statusLabel).Observe(elapsed.Seconds())
}

func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.Gatherer, promhttp.HandlerOpts{})
}

func RegisterPostgresPool(reg prometheus.Registerer, pool *pgxpool.Pool) error {
	if reg == nil || pool == nil {
		return fmt.Errorf("Prometheus registerer and PostgreSQL pool are required")
	}
	if err := reg.Register(newPostgresPoolCollector(pool)); err != nil {
		return fmt.Errorf("register PostgreSQL pool metrics: %w", err)
	}
	return nil
}

type postgresPoolCollector struct {
	pool *pgxpool.Pool
}

func newPostgresPoolCollector(pool *pgxpool.Pool) *postgresPoolCollector {
	return &postgresPoolCollector{pool: pool}
}

func (c *postgresPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range postgresPoolDescs {
		ch <- desc
	}
}

func (c *postgresPoolCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.pool.Stat()
	values := []float64{
		float64(stats.AcquiredConns()),
		float64(stats.IdleConns()),
		float64(stats.TotalConns()),
		float64(stats.MaxConns()),
		float64(stats.NewConnsCount()),
		float64(stats.MaxLifetimeDestroyCount()),
		float64(stats.MaxIdleDestroyCount()),
	}
	for i, value := range values {
		valueType := prometheus.GaugeValue
		if i >= 4 {
			valueType = prometheus.CounterValue
		}
		ch <- prometheus.MustNewConstMetric(postgresPoolDescs[i], valueType, value)
	}
}

var postgresPoolDescs = []*prometheus.Desc{
	prometheus.NewDesc("db_pool_acquired_connections", "Number of currently acquired PostgreSQL connections.", nil, nil),
	prometheus.NewDesc("db_pool_idle_connections", "Number of idle PostgreSQL connections.", nil, nil),
	prometheus.NewDesc("db_pool_total_connections", "Total number of PostgreSQL connections.", nil, nil),
	prometheus.NewDesc("db_pool_max_connections", "Maximum number of PostgreSQL connections.", nil, nil),
	prometheus.NewDesc("db_pool_new_connections_total", "Total number of newly opened PostgreSQL connections.", nil, nil),
	prometheus.NewDesc("db_pool_max_lifetime_closed_total", "Connections closed due to exceeding their maximum lifetime.", nil, nil),
	prometheus.NewDesc("db_pool_max_idle_closed_total", "Connections closed due to exceeding their maximum idle duration.", nil, nil),
}
