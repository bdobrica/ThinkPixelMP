// Package tracing initializes OpenTelemetry without capturing application
// payloads, evidence, descriptors, or request bodies.
package tracing

import (
	"context"
	"fmt"

	"github.com/bdobrica/ThinkPixelMP/internal/config"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
)

// Provider is an initialized tracer provider and W3C propagation contract.
// Callers own Shutdown and may install these values globally at process startup.
type Provider struct {
	TracerProvider *sdktrace.TracerProvider
	Propagator     propagation.TextMapPropagator
}

// New initializes tracing from validated configuration. Noop mode installs no
// exporter and never samples. OTLP exports spans only; payload capture is not
// enabled by this package and callers must use allowlisted bounded attributes.
func New(ctx context.Context, configured config.TelemetryConfig) (*Provider, error) {
	if ctx == nil {
		return nil, fmt.Errorf("tracing context is required")
	}
	if err := validate(configured); err != nil {
		return nil, err
	}
	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(configured.ServiceName)))
	if err != nil {
		return nil, fmt.Errorf("create tracing resource: %w", err)
	}
	options := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	if configured.Mode == "noop" {
		options = append(options, sdktrace.WithSampler(sdktrace.NeverSample()))
	} else {
		exporter, exportErr := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(configured.Endpoint))
		if exportErr != nil {
			return nil, fmt.Errorf("create OTLP trace exporter: %w", exportErr)
		}
		options = append(options, sdktrace.WithBatcher(exporter), sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(configured.SampleRatio))))
	}
	return &Provider{TracerProvider: sdktrace.NewTracerProvider(options...), Propagator: propagation.TraceContext{}}, nil
}

func validate(c config.TelemetryConfig) error {
	probe := config.Defaults()
	probe.Telemetry = c
	if err := probe.Validate(); err != nil {
		return fmt.Errorf("tracing configuration: %w", err)
	}
	return nil
}

func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.TracerProvider == nil {
		return nil
	}
	return p.TracerProvider.Shutdown(ctx)
}
