// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	apiParams "github.com/cloudbase/garm/apiserver/params"
	"github.com/cloudbase/garm/config"
	dbCommon "github.com/cloudbase/garm/database/common"
)

// JWTClaims holds JWT claims
type JWTClaims struct {
	UserID      string `json:"user"`
	TokenID     string `json:"token_id"`
	FullName    string `json:"full_name"`
	IsAdmin     bool   `json:"is_admin"`
	ReadMetrics bool   `json:"read_metrics"`
	Generation  uint   `json:"generation"`
	jwt.RegisteredClaims
}

// jwtMiddleware is the authentication middleware
// used with gorilla
type jwtMiddleware struct {
	store dbCommon.Store
	cfg   config.JWTAuth
}

// NewjwtMiddleware returns a populated jwtMiddleware
func NewjwtMiddleware(store dbCommon.Store, cfg config.JWTAuth) (Middleware, error) {
	return &jwtMiddleware{
		store: store,
		cfg:   cfg,
	}, nil
}

func (amw *jwtMiddleware) claimsToContext(ctx context.Context, claims *JWTClaims) (context.Context, error) {
	if claims == nil {
		return ctx, runnerErrors.ErrUnauthorized
	}

	if claims.UserID == "" {
		return nil, runnerErrors.ErrUnauthorized
	}

	userInfo, err := amw.store.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return ctx, runnerErrors.ErrUnauthorized
	}

	var expiresAt *time.Time
	if claims.ExpiresAt != nil {
		expires := claims.ExpiresAt.UTC()
		expiresAt = &expires
	}

	if userInfo.Generation != claims.Generation {
		// Password was reset since token was issued. Invalidate.
		return ctx, runnerErrors.ErrUnauthorized
	}

	ctx = PopulateContext(ctx, userInfo, expiresAt)
	return ctx, nil
}

func invalidAuthResponse(ctx context.Context, w http.ResponseWriter) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	if err := json.NewEncoder(w).Encode(
		apiParams.APIErrorResponse{
			Error: "Authentication failed",
		}); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

func (amw *jwtMiddleware) getTokenFromRequest(r *http.Request) (string, error) {
	authorizationHeader := r.Header.Get("authorization")
	if authorizationHeader == "" {
		cookie, err := r.Cookie("garm_token")
		if err != nil {
			return "", fmt.Errorf("failed to get cookie: %w", err)
		}
		return cookie.Value, nil
	}

	bearerToken := strings.Split(authorizationHeader, " ")
	if len(bearerToken) != 2 {
		return "", fmt.Errorf("invalid auth header")
	}
	return bearerToken[1], nil
}

// Middleware implements the middleware interface
func (amw *jwtMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// nolint:golangci-lint,godox
		// TODO: Log error details when authentication fails
		ctx := r.Context()
		authToken, err := amw.getTokenFromRequest(r)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get auth token", "error", err)
			invalidAuthResponse(ctx, w)
			return
		}
		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(authToken, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("invalid signing method")
			}
			return []byte(amw.cfg.Secret), nil
		})
		if err != nil {
			invalidAuthResponse(ctx, w)
			return
		}

		if !token.Valid {
			invalidAuthResponse(ctx, w)
			return
		}

		ctx, err = amw.claimsToContext(ctx, claims)
		if err != nil {
			invalidAuthResponse(ctx, w)
			return
		}
		if !IsEnabled(ctx) {
			invalidAuthResponse(ctx, w)
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
