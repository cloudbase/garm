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

package scalesets

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common/mocks"
)

func githubClientWithCABundle(t *testing.T, caBundle []byte) *mocks.GithubClient {
	t.Helper()
	cli := mocks.NewGithubClient(t)
	cli.On("GetEntity").Return(params.ForgeEntity{
		Credentials: params.ForgeCredentials{
			CABundle: caBundle,
		},
	}).Maybe()
	return cli
}

func TestNewClientOwnsTransport(t *testing.T) {
	scaleSetCli, err := NewClient(githubClientWithCABundle(t, nil))
	require.NoError(t, err)

	transport, ok := scaleSetCli.httpClient.Transport.(*http.Transport)
	require.True(t, ok, "expected an owned *http.Transport, not the default")
	require.Same(t, transport, scaleSetCli.longPollClient.Transport, "both clients must share one connection pool")

	require.NotNil(t, transport.HTTP2, "HTTP/2 health checks must be configured")
	require.Positive(t, transport.HTTP2.SendPingTimeout, "health check pings must be enabled")
	require.NotNil(t, transport.TLSClientConfig)
	require.Nil(t, transport.TLSClientConfig.RootCAs, "no CA bundle means the system trust store")

	require.Equal(t, requestTimeout, scaleSetCli.httpClient.Timeout)
	require.Equal(t, longPollRequestTimeout, scaleSetCli.longPollClient.Timeout)
}

func TestNewClientRejectsBadCABundle(t *testing.T) {
	_, err := NewClient(githubClientWithCABundle(t, []byte("not a pem")))
	require.ErrorContains(t, err, "failed to parse CA cert")
}

func TestClientTrustsCredentialCA(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	caBundle := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: srv.Certificate().Raw,
	})

	// The server's CA lives only in the credentials bundle, not in the
	// system trust store. The client built from the credentials connects.
	scaleSetCli, err := NewClient(githubClientWithCABundle(t, caBundle))
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := scaleSetCli.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	// A client without the bundle must refuse the connection instead of
	// silently trusting an unknown CA.
	noCACli, err := NewClient(githubClientWithCABundle(t, nil))
	require.NoError(t, err)
	req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	_, err = noCACli.Do(req) //nolint:bodyclose // the request must fail before a body exists
	require.ErrorContains(t, err, "certificate")
}
