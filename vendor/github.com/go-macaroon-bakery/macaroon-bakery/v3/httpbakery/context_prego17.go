// +build !go1.7

package httpbakery

import (
	"context"
	"net/http"
)

func contextFromRequest(req *http.Request) context.Context {
	return context.Background()
}
