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

func WithSlogContext(ctx context.Context, attrs ...slog.Attr) context.Context {
	return context.WithValue(ctx, slogCtxFields, attrs)
}
