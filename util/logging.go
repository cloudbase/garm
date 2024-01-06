package util

import (
	"context"
	"log/slog"
)

type slogContextKey string

const (
	slogCtxFields slogContextKey = "slog_ctx_fields"
)

type ContextHandler struct {
	slog.Handler
}

func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	attrs, ok := ctx.Value(slogCtxFields).([]slog.Attr)
	if ok {
		for _, v := range attrs {
			r.AddAttrs(v)
		}
	}
	return h.Handler.Handle(ctx, r)
}

func WithContext(ctx context.Context, attrs ...slog.Attr) context.Context {
	return context.WithValue(ctx, slogCtxFields, attrs)
}
