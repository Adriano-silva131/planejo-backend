package logger

import (
	"context"
	"log/slog"
	"slices"
)

type attrsKey struct{}

// With returns a ctx whose log lines, written with the *Context functions of
// slog, carry attrs.
func With(ctx context.Context, attrs ...slog.Attr) context.Context {
	prev, _ := ctx.Value(attrsKey{}).([]slog.Attr)
	return context.WithValue(ctx, attrsKey{}, append(slices.Clip(prev), attrs...))
}

type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if attrs, ok := ctx.Value(attrsKey{}).([]slog.Attr); ok {
		r.AddAttrs(attrs...)
	}
	return h.Handler.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
