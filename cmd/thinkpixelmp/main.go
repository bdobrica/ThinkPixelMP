package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	httpadapter "github.com/bdobrica/ThinkPixelMP/internal/adapters/http"
	"github.com/bdobrica/ThinkPixelMP/internal/config"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/clock"
	"github.com/bdobrica/ThinkPixelMP/internal/telemetry/logging"
	"github.com/bdobrica/ThinkPixelMP/internal/telemetry/metrics"
	"github.com/bdobrica/ThinkPixelMP/internal/telemetry/tracing"
)

func main() {
	if err := run(os.Args[1:], os.Environ()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "thinkpixelmp: %v\n", err)
		os.Exit(1)
	}
}

func run(args, environ []string) error {
	configured, err := config.Load(args, environ)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	logger, err := logging.New(os.Stderr, configured.Log.Level)
	if err != nil {
		return fmt.Errorf("initialize logging: %w", err)
	}
	registry, err := metrics.NewRegistry()
	if err != nil {
		return fmt.Errorf("initialize metrics: %w", err)
	}
	ids, err := shared.NewUUIDGenerator(clock.System{}, nil)
	if err != nil {
		return fmt.Errorf("initialize identifiers: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	provider, err := tracing.New(ctx, configured.Telemetry)
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), configured.HTTP.ShutdownTimeout)
		defer cancel()
		_ = provider.Shutdown(shutdownContext)
	}()

	server, err := httpadapter.New(configured.HTTP, httpadapter.Options{
		Gatherer:       registry.Gatherer(),
		Metrics:        registry,
		Logger:         logger,
		IDs:            ids,
		TracerProvider: provider.TracerProvider,
		Propagator:     provider.Propagator,
	})
	if err != nil {
		return fmt.Errorf("initialize HTTP server: %w", err)
	}
	return server.Run(ctx)
}
