package auth

import (
	"fmt"
	"garm/config"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt"
)

type MetricsMiddleware struct {
	cfg config.JWTAuth
}

func NewMetricsMiddleware(cfg config.JWTAuth) *MetricsMiddleware {
	return &MetricsMiddleware{
		cfg: cfg,
	}
}

func (m *MetricsMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

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
			return []byte(m.cfg.Secret), nil
		})

		if err != nil {
			invalidAuthResponse(w)
			return
		}

		if !token.Valid {
			invalidAuthResponse(w)
			return
		}

		// we fully trust the claims
		if !claims.ReadMetrics {
			invalidAuthResponse(w)
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
