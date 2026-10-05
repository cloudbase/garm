// Copyright 2026 Cloudbase Solutions SRL
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

package params

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

var (
	http2HealthCheckMux  sync.RWMutex
	http2ReadIdleTimeout time.Duration
	http2PingTimeout     time.Duration
)

// SetHTTP2HealthChecks configures HTTP/2 health checks (transparent PING
// frames) on all outbound HTTP transports created via this package. When
// readIdleTimeout is greater than 0, a PING frame is sent on any HTTP/2
// connection that has seen no frames for that duration; if no response
// arrives within pingTimeout, the connection is closed and in-flight
// requests fail fast instead of hanging until the kernel TCP timeout.
// A readIdleTimeout of 0 (the default) leaves behavior unchanged.
func SetHTTP2HealthChecks(readIdleTimeout, pingTimeout time.Duration) {
	http2HealthCheckMux.Lock()
	defer http2HealthCheckMux.Unlock()
	http2ReadIdleTimeout = readIdleTimeout
	http2PingTimeout = pingTimeout
}

func getHTTP2HealthChecks() (readIdleTimeout, pingTimeout time.Duration) {
	http2HealthCheckMux.RLock()
	defer http2HealthCheckMux.RUnlock()
	return http2ReadIdleTimeout, http2PingTimeout
}

// applyHTTP2HealthChecks enables HTTP/2 health checks on the given transport
// if they were configured via SetHTTP2HealthChecks. It is a no-op otherwise.
func applyHTTP2HealthChecks(transport *http.Transport) {
	readIdleTimeout, pingTimeout := getHTTP2HealthChecks()
	if readIdleTimeout <= 0 {
		return
	}
	h2Transport, err := http2.ConfigureTransports(transport)
	if err != nil {
		slog.With(slog.Any("error", err)).Error("failed to configure HTTP/2 health checks on transport")
		return
	}
	h2Transport.ReadIdleTimeout = readIdleTimeout
	if pingTimeout > 0 {
		h2Transport.PingTimeout = pingTimeout
	}
}

// NewHTTPClient returns an *http.Client suitable for API calls to a forge.
// If HTTP/2 health checks are configured, the client uses a dedicated
// transport with health checks enabled; otherwise it uses the default
// transport, preserving previous behavior.
func NewHTTPClient() *http.Client {
	readIdleTimeout, _ := getHTTP2HealthChecks()
	if readIdleTimeout <= 0 {
		return &http.Client{}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	applyHTTP2HealthChecks(transport)
	return &http.Client{Transport: transport}
}
