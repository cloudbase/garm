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
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	jwt "github.com/golang-jwt/jwt/v5"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/config"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (i *instanceToken) NewAgentJWTToken(instance params.Instance, entity params.ForgeEntity) (string, error) {
	claims := InstanceJWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "garm",
		},
		ID:            instance.ID,
		Name:          instance.Name,
		PoolID:        instance.PoolID,
		Scope:         entity.EntityType,
		Entity:        entity.ID,
		IsAgent:       true,
		ForgeType:     string(entity.Credentials.ForgeType),
		CreateAttempt: instance.CreateAttempt,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(i.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("error signing token: %w", err)
	}

	return tokenString, nil
}

// agentMiddleware is the authentication middleware
// used with gorilla
type agentMiddleware struct {
	store dbCommon.Store
	cfg   config.JWTAuth
}

// NewjwtMiddleware returns a populated jwtMiddleware
func AgentMiddleware(store dbCommon.Store, cfg config.JWTAuth) (Middleware, error) {
	return &agentMiddleware{
		store: store,
		cfg:   cfg,
	}, nil
}

func (amw *agentMiddleware) claimsToContext(ctx context.Context, claims *InstanceJWTClaims) (context.Context, error) {
	if claims == nil {
		return ctx, runnerErrors.ErrUnauthorized
	}

	if claims.Name == "" {
		return nil, runnerErrors.ErrUnauthorized
	}

	instanceInfo, err := amw.store.GetInstance(ctx, claims.Name)
	if err != nil {
		return ctx, runnerErrors.ErrUnauthorized
	}

	entity, err := getForgeEntityFromInstance(ctx, amw.store, instanceInfo)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get entity from instance", "error", err)
		return ctx, runnerErrors.ErrUnauthorized
	}

	ctx = PopulateInstanceContext(ctx, instanceInfo, entity, claims)
	return ctx, nil
}

// Middleware implements the middleware interface
func (amw *agentMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		authorizationHeader := r.Header.Get("authorization")
		if authorizationHeader == "" {
			slog.InfoContext(ctx, "authorization header was empty")
			invalidAuthResponse(ctx, w)
			return
		}

		bearerToken := strings.Split(authorizationHeader, " ")
		if len(bearerToken) != 2 {
			slog.InfoContext(ctx, "invalid authorization header")
			invalidAuthResponse(ctx, w)
			return
		}

		claims := &InstanceJWTClaims{}
		token, err := jwt.ParseWithClaims(bearerToken[1], claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("invalid signing method")
			}
			return []byte(amw.cfg.Secret), nil
		})
		if err != nil {
			slog.InfoContext(ctx, "failed to validate JWT token", "error", err)
			invalidAuthResponse(ctx, w)
			return
		}

		if !claims.IsAgent {
			invalidAuthResponse(ctx, w)
			return
		}

		if !token.Valid {
			slog.InfoContext(ctx, "JWT token is invalid")
			invalidAuthResponse(ctx, w)
			return
		}

		ctx, err = amw.claimsToContext(ctx, claims)
		if err != nil {
			slog.InfoContext(ctx, "failed to populate context", "error", err)
			invalidAuthResponse(ctx, w)
			return
		}

		if InstanceID(ctx) == "" {
			slog.InfoContext(ctx, "failed to find instance ID in context")
			invalidAuthResponse(ctx, w)
			return
		}

		runnerStatus := InstanceRunnerStatus(ctx)
		switch runnerStatus {
		case params.RunnerActive, params.RunnerTerminated, params.RunnerFailed:
			// Once a job starts to run, we can no longer trust that the JWT token was not compromised.
			// Any new auth requests using that token are not to be allowed.
			slog.InfoContext(ctx, "invalid runner status", "status", runnerStatus)
			invalidAuthResponse(ctx, w)
			return
		}

		instanceParams, err := InstanceParams(ctx)
		if err != nil {
			slog.InfoContext(
				ctx, "could not find instance params",
				"runner_name", InstanceName(ctx))
			invalidAuthResponse(ctx, w)
			return
		}

		// Token was generated for a previous attempt at creating this instance.
		if claims.CreateAttempt != instanceParams.CreateAttempt {
			slog.InfoContext(
				ctx, "invalid token create attempt",
				"runner_name", InstanceName(ctx),
				"token_create_attempt", claims.CreateAttempt,
				"instance_create_attempt", instanceParams.CreateAttempt)
			invalidAuthResponse(ctx, w)
			return
		}

		// instance must be running. Anything else is either still creating or in the process
		// of being deleted and shouldn't be trying to authenticate.
		if instanceParams.Status != commonParams.InstanceRunning {
			slog.InfoContext(ctx, "invalid instance status", "status", instanceParams.Status)
			invalidAuthResponse(ctx, w)
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
