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
	"net/http"
	"strings"
	"time"

	"garm/config"
	dbCommon "garm/database/common"
	runnerErrors "garm/errors"
	"garm/params"
	"garm/runner/common"

	"github.com/golang-jwt/jwt"
	"github.com/pkg/errors"
)

// InstanceJWTClaims holds JWT claims
type InstanceJWTClaims struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	PoolID string `json:"provider_id"`
	// Scope is either repository or organization
	Scope common.PoolType `json:"scope"`
	// Entity is the repo or org name
	Entity string `json:"entity"`
	jwt.StandardClaims
}

func NewInstanceJWTToken(instance params.Instance, secret, entity string, poolType common.PoolType) (string, error) {
	// make TTL configurable?
	expireToken := time.Now().Add(15 * time.Minute).Unix()
	claims := InstanceJWTClaims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expireToken,
			Issuer:    "garm",
		},
		ID:     instance.ID,
		Name:   instance.Name,
		PoolID: instance.PoolID,
		Scope:  poolType,
		Entity: entity,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", errors.Wrap(err, "signing token")
	}

	return tokenString, nil
}

// instanceMiddleware is the authentication middleware
// used with gorilla
type instanceMiddleware struct {
	store dbCommon.Store
	auth  *Authenticator
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
		// TODO: Log error details when authentication fails
		ctx := r.Context()
		authorizationHeader := r.Header.Get("authorization")
		if authorizationHeader == "" {
			invalidAuthResponse(w)
			return
		}

		bearerToken := strings.Split(authorizationHeader, " ")
		if len(bearerToken) != 2 {
			invalidAuthResponse(w)
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
			invalidAuthResponse(w)
			return
		}

		if !token.Valid {
			invalidAuthResponse(w)
			return
		}

		ctx, err = amw.claimsToContext(ctx, claims)
		if err != nil {
			invalidAuthResponse(w)
			return
		}

		if InstanceID(ctx) == "" {
			invalidAuthResponse(w)
		}

		// ctx = SetJWTClaim(ctx, *claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
