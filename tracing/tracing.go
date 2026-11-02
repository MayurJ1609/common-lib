package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Options struct {
	ServiceName    string
	ServiceVersion string
	Endpoint       string
	Insecure       bool
}

type Provider struct {
	provider *sdktrace.TracerProvider
}

func New(ctx context.Context, options Options) (*Provider, error) {
	if options.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	exporterOptions := []otlptracegrpc.Option{}
	if options.Endpoint != "" {
		exporterOptions = append(exporterOptions, otlptracegrpc.WithEndpoint(options.Endpoint))
	}
	if options.Insecure {
		exporterOptions = append(exporterOptions, otlptracegrpc.WithInsecure())
	}
	exporter, err := otlptracegrpc.New(ctx, exporterOptions...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	attributes := []attribute.KeyValue{attribute.String("service.name", options.ServiceName)}
	if options.ServiceVersion != "" {
		attributes = append(attributes, attribute.String("service.version", options.ServiceVersion))
	}
	res, err := resource.New(ctx, resource.WithAttributes(attributes...))
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, fmt.Errorf("create trace resource: %w", err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return &Provider{provider: tp}, nil
}

func (p *Provider) Shutdown(ctx context.Context) error {
	if err := p.provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown trace provider: %w", err)
	}
	return nil
}
