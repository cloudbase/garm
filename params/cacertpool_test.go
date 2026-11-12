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
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The endpoint CA bundle widens the system trust store rather than replacing
// it. Replacing it would make every publicly signed host GARM reaches for an
// endpoint unreachable, the tools metadata URL among them.
func TestCACertPoolWidensSystemRoots(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	bundle := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: srv.Certificate().Raw,
	})

	system, err := x509.SystemCertPool()
	if err != nil || system == nil {
		t.Skip("no system cert pool on this host")
	}

	roots, err := ForgeCredentials{CABundle: bundle}.CACertPool()
	if err != nil {
		t.Fatalf("CACertPool: %v", err)
	}
	if roots == nil {
		t.Fatal("expected a pool when a bundle is set")
	}
	//nolint:staticcheck // Subjects is deprecated but fine for a size comparison
	if got, want := len(roots.Subjects()), len(system.Subjects()); got <= want {
		t.Errorf("expected the system roots plus the bundle, got %d subjects vs %d system", got, want)
	}
}

// With no bundle the system trust store applies, signalled by a nil pool.
func TestCACertPoolNoBundle(t *testing.T) {
	roots, err := ForgeCredentials{}.CACertPool()
	if err != nil {
		t.Fatalf("CACertPool: %v", err)
	}
	if roots != nil {
		t.Error("expected a nil pool so the system trust store applies")
	}
}

func TestCACertPoolInvalidBundle(t *testing.T) {
	if _, err := (ForgeCredentials{CABundle: []byte("not a pem block")}).CACertPool(); err == nil {
		t.Error("expected an invalid CA bundle to fail")
	}
}
