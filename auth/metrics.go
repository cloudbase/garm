package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/cloudbase/garm/config"
)

type MetricsMiddleware struct {
	cfg config.JWTAuth
}

func NewMetricsMiddleware(cfg config.JWTAuth) (*MetricsMiddleware, error) {
	return &MetricsMiddleware{
		cfg: cfg,
	}, nil
}

func (m *MetricsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(bearerToken[1], claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("invalid signing method")
			}
			return []byte(m.cfg.Secret), nil
		})
		if err != nil {
			invalidAuthResponse(ctx, w)
			return
		}

		if !token.Valid {
			invalidAuthResponse(ctx, w)
			return
		}

		// we fully trust the claims
		if !claims.ReadMetrics {
			invalidAuthResponse(ctx, w)
			return
		}

		ctx = context.WithValue(ctx, isAdminKey, false)
		ctx = context.WithValue(ctx, readMetricsKey, true)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
