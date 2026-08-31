package httpadapter

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/bdobrica/ThinkPixelMP/internal/telemetry/logging"
	"github.com/bdobrica/ThinkPixelMP/internal/telemetry/metrics"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type middleware struct {
	options      Options
	maxBodyBytes int64
}

func (adapter *middleware) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID, err := adapter.options.IDs.New()
		if err != nil {
			writeProblem(writer, http.StatusInternalServerError, "internal", "request_id_unavailable", "Internal Server Error", "")
			return
		}
		requestIDText := requestID.String()
		writer.Header().Set(requestIDHeader, requestIDText)

		ctx := adapter.options.Propagator.Extract(request.Context(), propagation.HeaderCarrier(request.Header))
		ctx, span := adapter.options.TracerProvider.Tracer("thinkpixelmp/http").Start(ctx, "http.request", trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		correlation := logging.Correlation{RequestID: requestIDText}
		if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
			correlation.TraceID = spanContext.TraceID().String()
		}
		ctx, _ = logging.WithCorrelation(ctx, correlation)
		request = request.WithContext(ctx)
		if request.ContentLength > adapter.maxBodyBytes {
			writeProblem(writer, http.StatusRequestEntityTooLarge, "invalid", "request_body_too_large", "Content Too Large", requestIDText)
			return
		}
		request.Body = http.MaxBytesReader(writer, request.Body, adapter.maxBodyBytes)

		tracked := &responseWriter{ResponseWriter: writer, status: http.StatusOK}
		defer func() {
			if recovered := recover(); recovered != nil {
				if adapter.options.Logger != nil {
					_ = adapter.options.Logger.Error(ctx, "http.panic_recovered", slog.String("code", "handler_panic"), slog.Int("stack_bytes", len(debug.Stack())))
				}
				if !tracked.wroteHeader {
					writeProblem(tracked, http.StatusInternalServerError, "internal", "handler_panic", "Internal Server Error", requestIDText)
				}
			}
			if adapter.options.Metrics != nil {
				outcome, reason := metrics.OutcomeSucceeded, metrics.ReasonNone
				if tracked.status >= 400 {
					outcome, reason = metrics.OutcomeFailed, metrics.ReasonInvalid
					if tracked.status >= 500 {
						reason = metrics.ReasonInternal
					}
				}
				_ = adapter.options.Metrics.RecordOperation(metrics.OperationAPI, outcome, reason)
			}
		}()
		next.ServeHTTP(tracked, request)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// Unwrap lets net/http.ResponseController retain optional capabilities such as
// flushing for future SSE handlers without transport-specific type assertions.
func (writer *responseWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func (writer *responseWriter) WriteHeader(status int) {
	if writer.wroteHeader {
		return
	}
	writer.status, writer.wroteHeader = status, true
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *responseWriter) Write(body []byte) (int, error) {
	if !writer.wroteHeader {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(body)
}

func (adapter *middleware) livez(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeProblem(writer, http.StatusMethodNotAllowed, "invalid", "method_not_allowed", "Method Not Allowed", requestID(request.Context()))
		return
	}
	writeHealth(writer)
}

func (adapter *middleware) readyz(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeProblem(writer, http.StatusMethodNotAllowed, "invalid", "method_not_allowed", "Method Not Allowed", requestID(request.Context()))
		return
	}
	if adapter.options.Readiness != nil && adapter.options.Readiness.Check(request.Context()) != nil {
		writeProblem(writer, http.StatusServiceUnavailable, "unavailable", "service_not_ready", "Service Unavailable", requestID(request.Context()))
		return
	}
	writeHealth(writer)
}

func requestID(ctx context.Context) string { return logging.CorrelationFromContext(ctx).RequestID }
