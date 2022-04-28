package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	apiParams "runner-manager/apiserver/params"
	"runner-manager/config"
	dbCommon "runner-manager/database/common"
	runnerErrors "runner-manager/errors"
	"runner-manager/params"
	"runner-manager/runner/common"

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

// JWTClaims holds JWT claims
type JWTClaims struct {
	UserID   string `json:"user"`
	TokenID  string `json:"token_id"`
	FullName string `json:"full_name"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.StandardClaims
}

func NewInstanceJWTToken(instance params.Instance, secret, entity string, poolType common.PoolType) (string, error) {
	// make TTL configurable?
	expireToken := time.Now().Add(3 * time.Hour).Unix()
	claims := InstanceJWTClaims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expireToken,
			Issuer:    "runner-manager",
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

// jwtMiddleware is the authentication middleware
// used with gorilla
type jwtMiddleware struct {
	store dbCommon.Store
	auth  *Authenticator
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

	userInfo, err := amw.store.GetUser(ctx, claims.UserID)
	if err != nil {
		return ctx, runnerErrors.ErrUnauthorized
	}

	ctx = PopulateContext(ctx, userInfo)
	return ctx, nil
}

func invalidAuthResponse(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(
		apiParams.APIErrorResponse{
			Error:   "Authentication failed",
			Details: "Invalid authentication token",
		})
}

// Middleware implements the middleware interface
func (amw *jwtMiddleware) Middleware(next http.Handler) http.Handler {
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

		claims := &JWTClaims{}
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
		if !IsEnabled(ctx) {
			invalidAuthResponse(w)
			return
		}

		ctx = SetJWTClaim(ctx, *claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
