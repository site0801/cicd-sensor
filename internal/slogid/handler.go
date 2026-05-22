// Package slogid decorates a slog.Handler so every emitted record carries a
// unique log_id attribute, letting central logging consumers reference any
// individual line by ID.
package slogid

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// Key is the attribute name added to every record.
const Key = "log_id"

// Wrap returns a slog.Handler that delegates to inner after appending a
// UUIDv7 log_id attribute to each record.
func Wrap(inner slog.Handler) slog.Handler { return &handler{inner: inner} }

type handler struct{ inner slog.Handler }

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(slog.String(Key, newID()))
	return h.inner.Handle(ctx, r)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{inner: h.inner.WithAttrs(attrs)}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{inner: h.inner.WithGroup(name)}
}

func newID() string {
	id, _ := uuid.NewV7()
	return id.String()
}
