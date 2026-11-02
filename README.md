# common-lib

Shared Go service components for configuration, PostgreSQL/MongoDB/Redis/Kafka clients and migrations, logging, metrics, tracing, HTTP middleware, health checks, server lifecycle, and typed errors.

This is the standalone Go module `github.com/MayurJ1609/common-lib`. Add it to a service with:

```sh
go get github.com/MayurJ1609/common-lib
```

## Packages

- `config`: load a YAML/JSON/TOML or `.env` configuration file with Viper and override values from environment variables; `LoadDependencyConfig` loads the shared PostgreSQL, MongoDB, Redis, and Kafka settings from `.env.example` or the environment.
- `logger`: create a JSON `slog` logger with service and level options.
- `db`: create and ping a pgx connection pool; connect PostgreSQL, MongoDB, Redis, and Kafka; apply embedded PostgreSQL migrations; and expose health, cache, and publish operations.
- `metrics`: create a private Prometheus registry with Go/process collectors, HTTP request metrics, and an optional pgx pool collector.
- `tracing`: configure an OTLP/gRPC OpenTelemetry trace provider.
- `httpx`: compose request ID, structured request logging, Prometheus metrics, OpenTelemetry tracing, and panic recovery middleware.
- `health`: register liveness and dependency readiness endpoints.
- `server`: run an HTTP server until cancellation and gracefully shut it down.
- `errors`: create shared errors with machine-readable codes and wrapped causes.

## Service example

The runnable scaffold in [`examples/service-template`](examples/service-template) shows the packages together. Copy it into a service, then add dependencies and configuration checks specific to that application.

```sh
cd examples/service-template
go run .
```

By default the example listens on `:8080`, exports traces to `localhost:4317`, and exposes `/healthz`, `/readyz`, and `/metrics`. Set `HTTP_ADDR` and `OTEL_EXPORTER_OTLP_ENDPOINT` to override those defaults.

### Configuration

`config.Load[T]` accepts an optional configuration file and environment prefix. Struct fields should use `mapstructure` tags when environment names need underscores:

```go
type Config struct {
	HTTPAddr string `mapstructure:"http_addr"`
}

cfg, err := config.Load[Config](config.Options{
	File:      "config.yaml",
	EnvPrefix: "APP",
})
```

The file may be YAML, JSON, TOML, or dotenv, based on its extension. Environment variables override file values; the example field maps to `APP_HTTP_ADDR`.

`config.LoadDependencyConfig` provides shared backend settings for services that use `db.Connect`. It reads `.env.example` from the process working directory, then applies non-empty environment variable overrides.

The database lifecycle runs the migrations embedded in `db/migrations/sql/` and initializes configured MongoDB collections and the Kafka topic. Applications should add their own migration SQL there when adopting the common database bundle.

## Development

```sh
go test ./...
go vet ./...
go build ./examples/service-template
```
