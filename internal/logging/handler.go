package logging

import (
	"context"
	"log/slog"
)

// RedactHandler wraps another slog.Handler and removes secrets from the
// message and from every attribute (including nested groups) before the record
// reaches the wrapped handler.
type RedactHandler struct {
	next slog.Handler
	r    *Redactor
}

// NewRedactHandler wraps next. If r is nil, Default is used.
func NewRedactHandler(next slog.Handler, r *Redactor) *RedactHandler {
	if r == nil {
		r = Default
	}
	return &RedactHandler{next: next, r: r}
}

func (h *RedactHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *RedactHandler) Handle(ctx context.Context, rec slog.Record) error {
	out := slog.NewRecord(rec.Time, rec.Level, h.r.RedactString(rec.Message), rec.PC)
	rec.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.next.Handle(ctx, out)
}

func (h *RedactHandler) WithAttrs(as []slog.Attr) slog.Handler {
	red := make([]slog.Attr, len(as))
	for i, a := range as {
		red[i] = h.redactAttr(a)
	}
	return &RedactHandler{next: h.next.WithAttrs(red), r: h.r}
}

func (h *RedactHandler) WithGroup(name string) slog.Handler {
	return &RedactHandler{next: h.next.WithGroup(name), r: h.r}
}

func (h *RedactHandler) redactAttr(a slog.Attr) slog.Attr {
	a.Value = a.Value.Resolve()
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		red := make([]slog.Attr, len(attrs))
		for i, g := range attrs {
			red[i] = h.redactAttr(g)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(red...)}
	}
	if h.r.IsSensitiveKey(a.Key) {
		return slog.String(a.Key, Redacted)
	}
	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, h.r.RedactString(a.Value.String()))
	case slog.KindAny:
		if v, changed := h.r.redactAny(a.Value.Any()); changed {
			return slog.Any(a.Key, v)
		}
	}
	return a
}
