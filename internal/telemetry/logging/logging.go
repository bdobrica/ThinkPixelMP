// Package logging provides bounded, structured, redacting event logs.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"unicode"
)

const (
	RedactedMarker     = "[REDACTED]"
	TruncatedMarker    = "[TRUNCATED]"
	CycleMarker        = "[CYCLE]"
	UnsupportedMarker  = "[UNSUPPORTED]"
	maxIdentifierBytes = 128
)

var eventNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

// Correlation is trusted request and domain identity attached by middleware or
// application orchestration, never reconstructed from arbitrary log fields.
type Correlation struct {
	Tenant             string
	PublisherID        string
	ArtifactID         string
	ArtifactVersionID  string
	ArtifactDigest     string
	CatalogID          string
	PromotionRequestID string
	ResolutionID       string
	ImportSourceID     string
	RequestID          string
	TraceID            string
}

type correlationContextKey struct{}

// WithCorrelation validates and attaches correlation to a context. Empty
// fields are omitted; any invalid non-empty field rejects the entire update.
func WithCorrelation(ctx context.Context, correlation Correlation) (context.Context, error) {
	for name, value := range correlation.fields() {
		if value != "" && !safeIdentifier(value) {
			return ctx, fmt.Errorf("%s: invalid correlation identifier", name)
		}
	}
	return context.WithValue(ctx, correlationContextKey{}, correlation), nil
}

// CorrelationFromContext returns validated correlation, if present.
func CorrelationFromContext(ctx context.Context) Correlation {
	correlation, _ := ctx.Value(correlationContextKey{}).(Correlation)
	return correlation
}

func (c Correlation) fields() map[string]string {
	return map[string]string{
		"tenant": c.Tenant, "publisher_id": c.PublisherID, "artifact_id": c.ArtifactID,
		"artifact_version_id": c.ArtifactVersionID, "artifact_digest": c.ArtifactDigest,
		"catalog_id": c.CatalogID, "promotion_request_id": c.PromotionRequestID,
		"resolution_id": c.ResolutionID, "import_source_id": c.ImportSourceID,
		"request_id": c.RequestID, "trace_id": c.TraceID,
	}
}

func safeIdentifier(value string) bool {
	if value == "" || len(value) > maxIdentifierBytes || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.IsSpace(character) || !unicode.IsPrint(character) {
			return false
		}
	}
	return true
}

// Logger exposes only validated event logging operations.
type Logger struct {
	logger *slog.Logger
	attrs  []slog.Attr
}

// New creates a JSON logger writing one object per line.
func New(destination io.Writer, level string) (*Logger, error) {
	if destination == nil {
		return nil, fmt.Errorf("log destination is required")
	}
	parsed, err := ParseLevel(level)
	if err != nil {
		return nil, err
	}
	handler := slog.NewJSONHandler(destination, &slog.HandlerOptions{Level: parsed})
	return &Logger{logger: slog.New(handler)}, nil
}

// ParseLevel validates the configuration vocabulary.
func ParseLevel(value string) (slog.Level, error) {
	switch value {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", value)
	}
}

// With returns an immutable child logger with sanitized pre-bound attributes.
func (logger *Logger) With(attrs ...slog.Attr) *Logger {
	combined := append(append([]slog.Attr(nil), logger.attrs...), attrs...)
	return &Logger{logger: logger.logger, attrs: sanitizeAttrs(combined, 0)}
}

func (logger *Logger) Debug(ctx context.Context, event string, attrs ...slog.Attr) error {
	return logger.Log(ctx, slog.LevelDebug, event, attrs...)
}
func (logger *Logger) Info(ctx context.Context, event string, attrs ...slog.Attr) error {
	return logger.Log(ctx, slog.LevelInfo, event, attrs...)
}
func (logger *Logger) Warn(ctx context.Context, event string, attrs ...slog.Attr) error {
	return logger.Log(ctx, slog.LevelWarn, event, attrs...)
}
func (logger *Logger) Error(ctx context.Context, event string, attrs ...slog.Attr) error {
	return logger.Log(ctx, slog.LevelError, event, attrs...)
}

// Log emits a stable event after sanitizing all attributes.
func (logger *Logger) Log(ctx context.Context, level slog.Level, event string, attrs ...slog.Attr) error {
	if !eventNamePattern.MatchString(event) {
		return fmt.Errorf("invalid event name %q", event)
	}
	if !logger.logger.Enabled(ctx, level) {
		return nil
	}
	clean := make([]slog.Attr, 0, 1+maxCollectionEntries+11)
	clean = append(clean, slog.String("event", event))
	combined := append(append([]slog.Attr(nil), logger.attrs...), attrs...)
	clean = append(clean, sanitizeAttrs(combined, 0)...)
	for name, value := range CorrelationFromContext(ctx).fields() {
		if value != "" {
			clean = append(clean, slog.String(name, value))
		}
	}
	logger.logger.LogAttrs(ctx, level, event, clean...)
	return nil
}
