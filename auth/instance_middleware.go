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
	"math"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/config"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

// InstanceJWTClaims holds JWT claims
type InstanceJWTClaims struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	PoolID string `json:"provider_id"`
	// Scope is either repository or organization
	Scope params.ForgeEntityType `json:"scope"`
	// Entity is the repo or org name
	Entity        string `json:"entity"`
	CreateAttempt int    `json:"create_attempt"`
	jwt.RegisteredClaims
}

func NewInstanceTokenGetter(jwtSecret string) (InstanceTokenGetter, error) {
	if jwtSecret == "" {
		return nil, fmt.Errorf("jwt secret is required")
	}
	return &instanceToken{
		jwtSecret: jwtSecret,
	}, nil
}

type instanceToken struct {
	jwtSecret string
}

func (i *instanceToken) NewInstanceJWTToken(instance params.Instance, entity string, entityType params.ForgeEntityType, ttlMinutes uint) (string, error) {
	// Token expiration is equal to the bootstrap timeout set on the pool plus the polling
	// interval garm uses to check for timed out runners. Runners that have not sent their info
	// by the end of this interval are most likely failed and will be reaped by garm anyway.
	var ttl int
	if ttlMinutes > math.MaxInt {
		ttl = math.MaxInt
	} else {
		ttl = int(ttlMinutes)
	}
	expireToken := time.Now().Add(time.Duration(ttl)*time.Minute + common.PoolReapTimeoutInterval)
	expires := &jwt.NumericDate{
		Time: expireToken,
	}
	claims := InstanceJWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: expires,
			Issuer:    "garm",
		},
		ID:            instance.ID,
		Name:          instance.Name,
		PoolID:        instance.PoolID,
		Scope:         entityType,
		Entity:        entity,
		CreateAttempt: instance.CreateAttempt,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(i.jwtSecret))
	if err != nil {
		return "", errors.Wrap(err, "signing token")
	}

	return tokenString, nil
}

// instanceMiddleware is the authentication middleware
// used with gorilla
type instanceMiddleware struct {
	store dbCommon.Store
	cfg   config.JWTAuth
}

// NewjwtMiddleware returns a populated jwtMiddleware
func NewInstanceMiddleware(store dbCommon.Store, cfg config.JWTAuth) (Middleware, error) {
	return &instanceMiddleware{
		store: store,
		cfg:   cfg,
	}, nil
}

func (amw *instanceMiddleware) claimsToContext(ctx context.Context, claims *InstanceJWTClaims) (context.Context, error) {
	if claims == nil {
		return ctx, runnerErrors.ErrUnauthorized
	}

	if claims.Name == "" {
		return nil, runnerErrors.ErrUnauthorized
	}

	instanceInfo, err := amw.store.GetInstanceByName(ctx, claims.Name)
	if err != nil {
		return ctx, runnerErrors.ErrUnauthorized
	}

	ctx = PopulateInstanceContext(ctx, instanceInfo)
	return ctx, nil
}

// Middleware implements the middleware interface
func (amw *instanceMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// nolint:golangci-lint,godox
		// TODO: Log error details when authentication fails
		ctx := r.Context()
		authorizationHeader := r.Header.Get("authorization")
		if authorizationHeader == "" {
			invalidAuthResponse(ctx, w)
			return
		}

		bearerToken := strings.Split(authorizationHeader, " ")
		if len(bearerToken) != 2 {
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

		if InstanceID(ctx) == "" {
			invalidAuthResponse(ctx, w)
			return
		}

		runnerStatus := InstanceRunnerStatus(ctx)
		if runnerStatus != params.RunnerInstalling && runnerStatus != params.RunnerPending {
			// Instances that have finished installing can no longer authenticate to the API
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

		// Only allow instances that are in the creating or running state to authenticate.
		if instanceParams.Status != commonParams.InstanceCreating && instanceParams.Status != commonParams.InstanceRunning {
			slog.InfoContext(
				ctx, "invalid instance status",
				"runner_name", InstanceName(ctx),
				"status", instanceParams.Status)
			invalidAuthResponse(ctx, w)
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
