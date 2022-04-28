package auth

import "net/http"

// Middleware defines an authentication middleware
type Middleware interface {
	Middleware(next http.Handler) http.Handler
}
