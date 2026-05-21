package joblogs

import (
	"compress/gzip"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const DebugRuntimeTelemetryLogFilename = "job_runtime_telemetry_log.json.gz"

// DebugOutput writes local best-effort debug artifacts for hosted standalone
// jobs. Each record is appended as a complete gzip member so the artifact is
// readable even while the agent is still running.
type DebugOutput struct {
	mu     sync.Mutex
	logger *slog.Logger
	dir    string
}

func NewDebugOutput(logger *slog.Logger, dir string) (*DebugOutput, error) {
	if dir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create debug output dir %s: %w", dir, err)
	}
	return &DebugOutput{
		logger: logger,
		dir:    dir,
	}, nil
}

func (o *DebugOutput) WriteRuntimeTelemetryPayload(ctx context.Context, payload []byte) error {
	if o == nil || len(payload) == 0 {
		return nil
	}
	if err := o.appendGzipJSONRecord(DebugRuntimeTelemetryLogFilename, payload); err != nil {
		if o.logger != nil {
			o.logger.WarnContext(ctx, "debug_runtime_telemetry_write_failed",
				"path", filepath.Join(o.dir, DebugRuntimeTelemetryLogFilename),
				"error", err,
			)
		}
		return err
	}
	return nil
}

func (o *DebugOutput) appendGzipJSONRecord(path string, payload []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	root, err := os.OpenRoot(o.dir)
	if err != nil {
		return fmt.Errorf("open debug output root %s: %w", o.dir, err)
	}
	defer root.Close()

	file, err := root.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Join(o.dir, path), err)
	}
	defer file.Close()

	zw := gzip.NewWriter(file)
	if _, err := zw.Write(payload); err != nil {
		_ = zw.Close()
		return fmt.Errorf("write gzip payload: %w", err)
	}
	if _, err := zw.Write([]byte("\n")); err != nil {
		_ = zw.Close()
		return fmt.Errorf("write gzip newline: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}
	return nil
}
