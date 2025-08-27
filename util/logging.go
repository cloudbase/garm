// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.

package util

import (
	"context"
	"log/slog"
)

type slogContextKey string

const (
	slogCtxFields slogContextKey = "slog_ctx_fields"
)

var _ slog.Handler = &SlogMultiHandler{}

func WithSlogContext(ctx context.Context, attrs ...slog.Attr) context.Context {
	return context.WithValue(ctx, slogCtxFields, attrs)
}

type SlogMultiHandler struct {
	Handlers []slog.Handler
}

func (m *SlogMultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// Enabled if any handler is enabled
	for _, h := range m.Handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *SlogMultiHandler) Handle(ctx context.Context, r slog.Record) error {
	record := r.Clone()
	attrs, ok := ctx.Value(slogCtxFields).([]slog.Attr)
	if ok {
		for _, v := range attrs {
			record.AddAttrs(v)
		}
	}

	var firstErr error
	for _, h := range m.Handlers {
		if err := h.Handle(ctx, record); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *SlogMultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(m.Handlers))
	for i, h := range m.Handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return &SlogMultiHandler{
		Handlers: hs,
	}
}

func (m *SlogMultiHandler) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(m.Handlers))
	for i, h := range m.Handlers {
		hs[i] = h.WithGroup(name)
	}
	return &SlogMultiHandler{hs}
}
