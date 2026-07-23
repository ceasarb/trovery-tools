package otel

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const tracerName = "demigo-forge"

// Config holds OTel configuration, typically parsed from agent.yaml otel: section.
type Config struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint,omitempty"` // OTLP endpoint (e.g., localhost:4317)
	Protocol string `yaml:"protocol,omitempty"` // grpc or http (default: grpc)
	Insecure bool   `yaml:"insecure,omitempty"` // skip TLS verification
}

// Provider wraps the OTel tracer provider with convenience methods.
type Provider struct {
	tp     *sdktrace.TracerProvider
	tracer trace.Tracer
}

// Init sets up the OTel trace provider based on config.
// Returns a no-op provider when disabled, enabling call sites to skip nil checks.
func Init(ctx context.Context, cfg Config) (*Provider, error) {
	if !cfg.Enabled {
		return &Provider{tracer: noop.NewTracerProvider().Tracer(tracerName)}, nil
	}

	var exporter sdktrace.SpanExporter
	var err error

	switch cfg.Protocol {
	case "http":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exporter, err = otlptracehttp.New(ctx, opts...)
	default: // grpc
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
	}

	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(tp)

	return &Provider{
		tp:     tp,
		tracer: tp.Tracer(tracerName),
	}, nil
}

// Tracer returns the configured tracer.
func (p *Provider) Tracer() trace.Tracer {
	return p.tracer
}

// StartSpan starts a new span and returns the updated context and span.
func (p *Provider) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return p.tracer.Start(ctx, name)
}

// Shutdown flushes pending spans and shuts down the provider.
// Safe to call on a no-op provider (tp == nil).
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.tp == nil {
		return nil
	}
	return p.tp.Shutdown(ctx)
}

// IsEnabled returns true if tracing is active (not a no-op).
func (p *Provider) IsEnabled() bool {
	return p.tp != nil
}
