package tracing

import (
	"context"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/config"
	"go.opentelemetry.io/otel/propagation"
)

func TestNoopProviderDoesNotSampleAndUsesW3CPropagation(t *testing.T) {
	configured := config.Defaults().Telemetry
	p, err := New(context.Background(), configured)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
	span := p.TracerProvider.Tracer("test").Start
	_, started := span(context.Background(), "bounded.operation")
	if started.IsRecording() {
		t.Fatal("noop tracing recorded a span")
	}
	started.End()
	carrier := propagation.MapCarrier{}
	ctx := propagation.TraceContext{}.Extract(context.Background(), propagation.MapCarrier{"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"})
	p.Propagator.Inject(ctx, carrier)
	if carrier.Get("traceparent") == "" {
		t.Fatal("W3C trace context was not propagated")
	}
	if fields := p.Propagator.Fields(); len(fields) != 2 || fields[0] != "traceparent" || fields[1] != "tracestate" {
		t.Fatalf("unexpected propagated fields: %v", fields)
	}
}

func TestTracingRejectsUnsafeOrIncompleteConfiguration(t *testing.T) {
	for _, configured := range []config.TelemetryConfig{
		{Mode: "otlp", Endpoint: "https://user:secret@example.test", ServiceName: "thinkpixelmp", SampleRatio: 1},
		{Mode: "otlp", Endpoint: "https://example.test?token=secret", ServiceName: "thinkpixelmp", SampleRatio: 1},
		{Mode: "noop", Endpoint: "https://example.test", ServiceName: "thinkpixelmp"},
		{Mode: "unknown", ServiceName: "thinkpixelmp"},
	} {
		if _, err := New(context.Background(), configured); err == nil {
			t.Fatalf("accepted unsafe config: %#v", configured)
		}
	}
	//lint:ignore SA1012 This deliberately verifies the public nil-context guard.
	if _, err := New(nil, config.Defaults().Telemetry); err == nil {
		t.Fatal("nil context accepted")
	}
}
