package otel

import (
	"context"
	"testing"
)

func TestInit_Disabled(t *testing.T) {
	p, err := Init(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if p.IsEnabled() {
		t.Error("expected disabled provider")
	}

	// Verify no-op tracer works without panic
	ctx, span := p.StartSpan(context.Background(), "test")
	span.End()
	_ = ctx

	// Shutdown should be safe on no-op
	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}

func TestInit_EnabledBadEndpoint(t *testing.T) {
	// Enabled but with a valid config (exporter creation succeeds even
	// with unreachable endpoints — it fails on flush, not connect).
	p, err := Init(context.Background(), Config{
		Enabled:  true,
		Endpoint: "localhost:4317",
		Protocol: "grpc",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !p.IsEnabled() {
		t.Error("expected enabled provider")
	}

	// Verify tracer works
	tracer := p.Tracer()
	if tracer == nil {
		t.Error("Tracer() returned nil")
	}

	ctx, span := p.StartSpan(context.Background(), "test-span")
	span.End()
	_ = ctx

	// Shutdown (will fail to flush but shouldn't panic)
	_ = p.Shutdown(context.Background())
}

func TestInit_HTTPProtocol(t *testing.T) {
	p, err := Init(context.Background(), Config{
		Enabled:  true,
		Endpoint: "localhost:4318",
		Protocol: "http",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !p.IsEnabled() {
		t.Error("expected enabled provider with http protocol")
	}

	_ = p.Shutdown(context.Background())
}

func TestInit_DefaultProtocol(t *testing.T) {
	// Empty protocol defaults to gRPC
	p, err := Init(context.Background(), Config{
		Enabled:  true,
		Endpoint: "localhost:4317",
		Insecure: true,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !p.IsEnabled() {
		t.Error("expected enabled provider with default protocol")
	}

	_ = p.Shutdown(context.Background())
}
