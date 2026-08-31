// Package httpadapter exposes application use cases through a bounded HTTP
// transport. It owns transport concerns only; readiness dependencies and API
// use cases are supplied through narrow interfaces.
package httpadapter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/bdobrica/ThinkPixelMP/internal/config"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/telemetry/logging"
	"github.com/bdobrica/ThinkPixelMP/internal/telemetry/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const requestIDHeader = "X-Request-ID"

// Readiness reports whether the process can safely serve its configured role.
// Returned errors are classified but never serialized to clients.
type Readiness interface {
	Check(context.Context) error
}

type Options struct {
	API            http.Handler
	Readiness      Readiness
	Gatherer       prometheus.Gatherer
	Metrics        *metrics.Registry
	Logger         *logging.Logger
	IDs            *shared.UUIDGenerator
	TracerProvider trace.TracerProvider
	Propagator     propagation.TextMapPropagator
}

// Server is the baseline HTTP adapter and its graceful lifecycle owner.
type Server struct {
	httpServer      *http.Server
	shutdownTimeout time.Duration
}

func New(configured config.HTTPConfig, options Options) (*Server, error) {
	if options.IDs == nil || options.TracerProvider == nil || options.Propagator == nil || options.Gatherer == nil {
		return nil, fmt.Errorf("http adapter: IDs, tracer provider, propagator, and metrics gatherer are required")
	}
	if configured.Address == "" || configured.MaxBodyBytes < 1 || configured.MaxHeaderBytes < 1 || configured.ReadHeaderTimeout <= 0 || configured.ReadTimeout <= 0 || configured.WriteTimeout <= 0 || configured.IdleTimeout <= 0 || configured.ShutdownTimeout <= 0 {
		return nil, fmt.Errorf("http adapter: invalid HTTP configuration")
	}

	adapter := &middleware{options: options, maxBodyBytes: configured.MaxBodyBytes}
	mux := http.NewServeMux()
	mux.Handle("/livez", adapter.wrap(http.HandlerFunc(adapter.livez)))
	mux.Handle("/readyz", adapter.wrap(http.HandlerFunc(adapter.readyz)))
	metricsHandler := promhttp.HandlerFor(options.Gatherer, promhttp.HandlerOpts{ErrorHandling: promhttp.HTTPErrorOnError})
	mux.Handle("/metrics", adapter.wrap(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writeProblem(writer, http.StatusMethodNotAllowed, "invalid", "method_not_allowed", "Method Not Allowed", requestID(request.Context()))
			return
		}
		metricsHandler.ServeHTTP(writer, request)
	})))
	if options.API != nil {
		mux.Handle("/v1/", adapter.wrap(options.API))
	}
	mux.Handle("/", adapter.wrap(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeProblem(writer, http.StatusNotFound, "not_found", "route_not_found", "Not Found", requestID(request.Context()))
	})))

	return &Server{httpServer: &http.Server{
		Addr:              configured.Address,
		Handler:           mux,
		ReadHeaderTimeout: configured.ReadHeaderTimeout,
		ReadTimeout:       configured.ReadTimeout,
		WriteTimeout:      configured.WriteTimeout,
		IdleTimeout:       configured.IdleTimeout,
		MaxHeaderBytes:    configured.MaxHeaderBytes,
	}, shutdownTimeout: configured.ShutdownTimeout}, nil
}

func (server *Server) Handler() http.Handler { return server.httpServer.Handler }

// Run serves until the context is cancelled, then performs a bounded graceful
// shutdown. An unexpected serving failure is returned to the process owner.
func (server *Server) Run(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("http adapter: run context is required")
	}
	listener, err := net.Listen("tcp", server.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.httpServer.Serve(listener) }()
	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), server.shutdownTimeout)
		defer cancel()
		if err := server.httpServer.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown HTTP: %w", err)
		}
		if err := <-serveErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}
}
