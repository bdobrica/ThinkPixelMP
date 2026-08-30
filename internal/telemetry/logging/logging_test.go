package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/bdobrica/ThinkPixelMP/internal/config"
)

func TestNewFiltersLevelsAndEmitsJSONEvents(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(&output, "warn")
	if err != nil {
		t.Fatal(err)
	}
	if err := logger.Info(context.Background(), "service.filtered"); err != nil {
		t.Fatal(err)
	}
	if err := logger.Warn(context.Background(), "service.started", slog.String("component", "api")); err != nil {
		t.Fatal(err)
	}
	entries := decodeEntries(t, output.String())
	if len(entries) != 1 || entries[0]["event"] != "service.started" || entries[0]["msg"] != "service.started" || entries[0]["level"] != "WARN" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	if _, err := New(&output, "verbose"); err == nil {
		t.Fatal("unsupported level accepted")
	}
	if _, err := New(nil, "info"); err == nil {
		t.Fatal("nil destination accepted")
	}
}

func TestEventNamesConstrainMessages(t *testing.T) {
	const canary = "MESSAGE_SECRET_CANARY_8135"
	var output bytes.Buffer
	logger, _ := New(&output, "info")
	for _, event := range []string{"single", "HTTP.request", "http request.completed", "http.request." + canary} {
		if err := logger.Info(context.Background(), event); err == nil {
			t.Errorf("accepted %q", event)
		}
	}
	if output.Len() != 0 {
		t.Fatalf("invalid message emitted: %s", output.String())
	}
}

func TestTrustedCorrelationCannotBeSpoofed(t *testing.T) {
	var output bytes.Buffer
	logger, _ := New(&output, "info")
	correlation := Correlation{Tenant: "tenant-1", PublisherID: "publisher-1", ArtifactID: "artifact-1", ArtifactVersionID: "version-1", ArtifactDigest: "sha256:abc", CatalogID: "catalog-1", PromotionRequestID: "promotion-1", ResolutionID: "resolution-1", ImportSourceID: "source-1", RequestID: "request-1", TraceID: "trace-1"}
	ctx, err := WithCorrelation(context.Background(), correlation)
	if err != nil {
		t.Fatal(err)
	}
	logger = logger.With(slog.String("request-id", "spoofed-prebound"), slog.String("level", "spoofed-level"))
	if err := logger.Info(ctx, "artifact.inspected", slog.String("tenant", "spoofed-record"), slog.String("event", "spoofed-event"), slog.String("msg", "spoofed-message"), slog.Any("nested", map[string]any{"trace_id": "spoofed-nested", "safe": "kept"})); err != nil {
		t.Fatal(err)
	}
	entry := decodeEntries(t, output.String())[0]
	for name, value := range correlation.fields() {
		if entry[name] != value {
			t.Errorf("%s=%v, want %s", name, entry[name], value)
		}
	}
	if strings.Contains(output.String(), "spoofed") {
		t.Fatalf("spoofed correlation emitted: %s", output.String())
	}
}

func TestInvalidCorrelationRejectsEntireUpdate(t *testing.T) {
	original := context.Background()
	for _, value := range []string{"bad id", " padded", strings.Repeat("a", maxIdentifierBytes+1), "line\nbreak"} {
		ctx, err := WithCorrelation(original, Correlation{RequestID: value})
		if err == nil || ctx != original {
			t.Errorf("accepted %q", value)
		}
	}
}

type secretLogValuer struct{ canary string }

func (v secretLogValuer) LogValue() slog.Value {
	return slog.GroupValue(slog.String("access_token", v.canary), slog.String("status", "safe"))
}

type secretStruct struct {
	Name       string `json:"name"`
	Password   string `json:"password"`
	Credential string
}

type secretLoggableError struct{ canary string }

func (e secretLoggableError) Error() string { return e.canary }
func (e secretLoggableError) LogValue() slog.Value {
	return slog.StringValue(e.canary)
}

func TestRecursiveRedactionAndExplicitClassification(t *testing.T) {
	const canary = "SECRET_CANARY_4f13"
	var output bytes.Buffer
	logger, _ := New(&output, "debug")
	logger = logger.With(slog.Any("credentials", map[string]any{"password": canary}))
	err := logger.Debug(context.Background(), "security.redaction_test",
		slog.String("Authorization", "Bearer "+canary),
		slog.Any("headers", map[string][]string{"X-API-Key": {canary}, "Content-Type": {"application/json"}}),
		slog.Any("valuer", secretLogValuer{canary}),
		slog.Any("nested", []any{map[string]any{"refresh-token": canary, "safe": "visible"}}),
		slog.Any("structure", secretStruct{Name: "visible-name", Password: canary, Credential: canary}),
		slog.Any("confidential", Confidential(canary)), slog.Any("restricted", Restricted(canary)),
	)
	if err != nil {
		t.Fatal(err)
	}
	logged := output.String()
	if strings.Contains(logged, canary) || strings.Contains(logged, "Bearer") {
		t.Fatalf("secret leaked: %s", logged)
	}
	for _, safe := range []string{RedactedMarker, "application/json", "visible", "visible-name", "safe"} {
		if !strings.Contains(logged, safe) {
			t.Errorf("missing %q in %s", safe, logged)
		}
	}
}

func TestErrorsBodiesAndConfigurationSecretsAreSuppressed(t *testing.T) {
	const canary = "BOUNDARY_SECRET_CANARY_2894"
	ref, _ := config.ParseSecretRef("env:" + canary)
	cfg := config.Defaults()
	cfg.Database.URL = ref
	var output bytes.Buffer
	logger, _ := New(&output, "info")
	if err := logger.Error(context.Background(), "dependency.failed", slog.Any("failure", errors.New(canary)), slog.Any("loggable_failure", secretLoggableError{canary}), slog.String("request.body", canary), slog.Any("configuration", cfg)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), canary) {
		t.Fatalf("boundary value leaked: %s", output.String())
	}
}

type cycleNode struct {
	Name string
	Next *cycleNode
}

func TestStructuralBoundsCyclesAndUnsupportedValues(t *testing.T) {
	root := &cycleNode{Name: "root"}
	root.Next = root
	large := make([]any, maxCollectionEntries+2)
	for index := range large {
		large[index] = index
	}
	deep := any("leaf")
	for range maxDepth + 2 {
		deep = map[string]any{"child": deep}
	}
	var output bytes.Buffer
	logger, _ := New(&output, "info")
	if err := logger.Info(context.Background(), "logging.bounds_checked", slog.String("long", strings.Repeat("x", maxStringBytes+1)), slog.Any("large", large), slog.Any("deep", deep), slog.Any("cycle", root), slog.Any("unsupported", map[int]string{1: "value"})); err != nil {
		t.Fatal(err)
	}
	logged := output.String()
	for _, marker := range []string{TruncatedMarker, CycleMarker, UnsupportedMarker} {
		if !strings.Contains(logged, marker) {
			t.Errorf("missing %s in %s", marker, logged)
		}
	}
}

func TestLoggerWithIsImmutable(t *testing.T) {
	var output bytes.Buffer
	base, _ := New(&output, "info")
	child := base.With(slog.String("component", "worker"))
	_ = base.Info(context.Background(), "service.base")
	_ = child.Info(context.Background(), "service.child")
	entries := decodeEntries(t, output.String())
	if _, ok := entries[0]["component"]; ok {
		t.Fatal("child attribute mutated parent")
	}
	if entries[1]["component"] != "worker" {
		t.Fatal("child attribute absent")
	}
}

func TestRepeatedWithCannotBypassAttributeLimit(t *testing.T) {
	var output bytes.Buffer
	logger, _ := New(&output, "info")
	for index := 0; index < maxCollectionEntries+10; index++ {
		logger = logger.With(slog.Int(fmt.Sprintf("field_%d", index), index))
	}
	if err := logger.Info(context.Background(), "logging.attributes_bounded"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), TruncatedMarker) {
		t.Fatalf("attribute truncation absent: %s", output.String())
	}
}

func TestConcurrentLoggingProducesCompleteRecords(t *testing.T) {
	var output lockedBuffer
	logger, _ := New(&output, "info")
	var group sync.WaitGroup
	for index := 0; index < 32; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			_ = logger.Info(context.Background(), "worker.completed", slog.Int("worker", index))
		}(index)
	}
	group.Wait()
	if entries := decodeEntries(t, output.String()); len(entries) != 32 {
		t.Fatalf("entries=%d", len(entries))
	}
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(value)
}
func (b *lockedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.buffer.String() }

func decodeEntries(t *testing.T, output string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}
