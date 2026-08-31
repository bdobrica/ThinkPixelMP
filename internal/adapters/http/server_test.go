package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/config"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/telemetry/metrics"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type readinessFunc func(context.Context) error

func (function readinessFunc) Check(ctx context.Context) error { return function(ctx) }

func newTestServer(t *testing.T, api http.Handler, readiness Readiness, maximum int64) *Server {
	t.Helper()
	registry, err := metrics.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ids, err := shared.NewUUIDGenerator(clock.Fixed{Time: time.UnixMilli(1_700_000_000_000)}, bytes.NewReader(bytes.Repeat([]byte{1}, 1000)))
	if err != nil {
		t.Fatal(err)
	}
	configured := config.Defaults().HTTP
	configured.MaxBodyBytes = maximum
	server, err := New(configured, Options{API: api, Readiness: readiness, Gatherer: registry.Gatherer(), Metrics: registry, IDs: ids, TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample())), Propagator: propagation.TraceContext{}})
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func TestHealthMetricsAndRequestIDs(t *testing.T) {
	server := newTestServer(t, nil, nil, 32)
	for _, path := range []string{"/livez", "/readyz", "/metrics"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", path, response.Code, response.Body.String())
		}
		if _, err := shared.ParseUUID(response.Header().Get(requestIDHeader)); err != nil {
			t.Fatalf("%s request ID: %v", path, err)
		}
	}
}

func TestReadinessFailureIsSafeProblem(t *testing.T) {
	secret := "database-password-canary"
	server := newTestServer(t, nil, readinessFunc(func(context.Context) error { return errors.New(secret) }), 32)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), secret) {
		t.Fatal("readiness error leaked")
	}
}

func TestTraceContextAndBodyLimit(t *testing.T) {
	reached := false
	server := newTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		reached = true
		writer.WriteHeader(http.StatusNoContent)
	}), nil, 8)
	request := httptest.NewRequest(http.MethodPost, "/v1/test", strings.NewReader("123456789"))
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", response.Code)
	}
	if reached {
		t.Fatal("oversized request reached API")
	}
}

func TestTraceParentIsExtracted(t *testing.T) {
	var got string
	server := newTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		got = trace.SpanContextFromContext(request.Context()).TraceID().String()
		writer.WriteHeader(http.StatusNoContent)
	}), nil, 32)
	request := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace ID %q", got)
	}
}

func TestPanicAndUnknownErrorsDoNotLeak(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("secret-canary") }), nil, 32)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/panic", nil))
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "secret-canary") {
		t.Fatalf("unsafe panic response: %d %s", response.Code, response.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(context.Background())
	errorResponse := httptest.NewRecorder()
	WriteError(errorResponse, request, errors.New("sql-password-canary"))
	var body problem
	if err := json.Unmarshal(errorResponse.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "internal_error" || strings.Contains(errorResponse.Body.String(), "canary") {
		t.Fatalf("unsafe error response: %s", errorResponse.Body.String())
	}
}

func TestTypedErrorsMapWithoutDetail(t *testing.T) {
	code, err := shared.NewReasonCode("artifact_conflict")
	if err != nil {
		t.Fatal(err)
	}
	typed := shared.NewTypedError(shared.ErrorConflict, code)
	response := httptest.NewRecorder()
	WriteError(response, httptest.NewRequest(http.MethodGet, "/", nil), typed)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"artifact_conflict"`) {
		t.Fatalf("unexpected typed error: %d %s", response.Code, response.Body.String())
	}
}
