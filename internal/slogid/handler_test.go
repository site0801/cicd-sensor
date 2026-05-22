package slogid_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"testing"

	"github.com/cicd-sensor/cicd-sensor/internal/slogid"
)

var uuidV7Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slogid.Wrap(slog.NewJSONHandler(buf, nil)))
}

func decodeLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	dec := json.NewDecoder(buf)
	for dec.More() {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			t.Fatalf("decode: %v", err)
		}
		out = append(out, m)
	}
	return out
}

func TestWrap_AddsUUIDv7PerRecord(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)
	logger.Info("first")
	logger.Info("second")

	lines := decodeLines(t, &buf)
	if len(lines) != 2 {
		t.Fatalf("lines: got %d, want 2", len(lines))
	}
	first, _ := lines[0][slogid.Key].(string)
	second, _ := lines[1][slogid.Key].(string)
	if !uuidV7Pattern.MatchString(first) {
		t.Fatalf("first log_id not UUIDv7: %q", first)
	}
	if !uuidV7Pattern.MatchString(second) {
		t.Fatalf("second log_id not UUIDv7: %q", second)
	}
	if first == second {
		t.Fatalf("log_id must be unique per record, got duplicate %q", first)
	}
}

func TestWrap_PropagatesThroughWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf).With("component", "test")
	logger.Info("msg")
	lines := decodeLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("lines: got %d, want 1", len(lines))
	}
	if _, ok := lines[0][slogid.Key].(string); !ok {
		t.Fatalf("log_id missing after With(): %v", lines[0])
	}
	if lines[0]["component"] != "test" {
		t.Fatalf("With() attr lost: %v", lines[0])
	}
}

func TestWrap_WithGroupNestsLogID(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf).WithGroup("nested")
	logger.Info("msg", "extra", "value")
	lines := decodeLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("lines: got %d, want 1", len(lines))
	}
	// Under an active group, log_id lands inside the group along with the
	// other record attrs. This is the standard slog group behaviour; we
	// only verify it still appears so consumers can find it.
	nested, ok := lines[0]["nested"].(map[string]any)
	if !ok {
		t.Fatalf("group attrs missing: %v", lines[0])
	}
	if _, ok := nested[slogid.Key].(string); !ok {
		t.Fatalf("log_id missing from group: %v", nested)
	}
	if nested["extra"] != "value" {
		t.Fatalf("group content wrong: %v", nested)
	}
}

func TestWrap_EnabledRespectsInner(t *testing.T) {
	inner := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	h := slogid.Wrap(inner)
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatalf("Enabled(Debug) should be false when inner is Warn-only")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Fatalf("Enabled(Error) should be true when inner is Warn-only")
	}
}
