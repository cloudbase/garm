// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

//go:build testing

package v011

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

// TestIncusAdmissionInterop exercises both wire implementations, not a shell
// fixture. The provider executable is built separately from the Incus source.
func TestIncusAdmissionInterop(t *testing.T) {
	executable := os.Getenv("GARM_TEST_INCUS_PROVIDER")
	if executable == "" {
		t.Skip("GARM_TEST_INCUS_PROVIDER is not set")
	}
	require.True(t, filepath.IsAbs(executable))
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	certTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "synthetic-test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	fingerprint := strings.Repeat("a", 64)
	var present atomic.Bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			t.Error("provider did not present its client certificate")
		}
		if (strings.HasPrefix(r.URL.Path, "/1.0/instances") || strings.HasPrefix(r.URL.Path, "/1.0/profiles") || strings.HasPrefix(r.URL.Path, "/1.0/images")) && r.URL.Query().Get("project") != "test" {
			t.Errorf("incorrect project on %s", r.URL.Path)
		}
		var metadata any
		switch r.URL.Path {
		case "/1.0":
			metadata = map[string]any{"api_extensions": []string{"projects", "project_usage", "container_full", "instance_get_full", "virtual-machines"}, "auth": "trusted", "environment": map[string]any{"driver": "qemu", "kernel_architecture": "x86_64", "server_version": "6.0.7", "server_clustered": false}}
		case "/1.0/projects/test":
			metadata = map[string]any{"name": "test"}
		case "/1.0/projects/test/state":
			limits := map[string]any{}
			for _, k := range []string{"cpu", "memory", "disk", "instances", "virtual-machines"} {
				limits[k] = map[string]int{"limit": -1, "usage": 0}
			}
			metadata = map[string]any{"resources": limits}
		case "/1.0/profiles":
			metadata = []string{"/1.0/profiles/runner"}
		case "/1.0/profiles/runner":
			metadata = map[string]any{"name": "runner", "config": map[string]string{"limits.cpu": "2", "limits.memory": "2GiB"}, "devices": map[string]any{"root": map[string]string{"type": "disk", "path": "/", "pool": "test", "size": "10GiB"}}}
		case "/1.0/images/" + fingerprint:
			metadata = map[string]any{"fingerprint": fingerprint, "architecture": "x86_64", "type": "virtual-machine"}
		case "/1.0/instances":
			metadata = []any{}
			if present.Load() {
				metadata = []any{map[string]any{"name": "guest-one", "expanded_config": map[string]string{"user.runner-controller-id": "controller"}}, map[string]any{"name": "other", "expanded_config": map[string]string{"user.runner-controller-id": "other"}}}
			}
		case "/1.0/instances/missing":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"type":"error","error":"not found","error_code":404}`))
			return
		default:
			t.Errorf("unexpected Incus path %s", r.URL.Path)
			http.Error(w, "unexpected", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "sync", "status": "Success", "status_code": 200, "metadata": metadata})
	})
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, ClientAuth: tls.RequestClientCert, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	dir := t.TempDir()
	write := func(name string, b []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, b, 0o600))
		return path
	}
	certPath, keyPath := write("cert.pem", certPEM), write("key.pem", keyPEM)
	cfgPath := write("provider.toml", []byte(fmt.Sprintf("url = %q\nproject_name = %q\nclient_certificate = %q\nclient_key = %q\ntls_server_certificate = %q\n", server.URL, "test", certPath, keyPath, certPath)))
	p, err := NewProvider(context.Background(), &config.Provider{Name: "test", ProviderType: params.ExternalProvider, External: config.External{ProviderExecutable: executable, ConfigFile: cfgPath, InterfaceVersion: "0.1.1"}}, "controller")
	require.NoError(t, err)
	readiness, ok := p.(common.ReadinessProvider)
	require.True(t, ok)
	inv, ok := p.(common.AdmissionInventoryProvider)
	require.True(t, ok)
	absence, ok := p.(common.AuthoritativeAbsenceProvider)
	require.True(t, ok)
	ctx := context.Background()
	req := common.ReadinessRequest{Version: 1, RequestID: "r1", Generation: "g1", Domain: "d1", Project: "test", Image: fingerprint, Flavor: "runner", OSArch: "amd64", InstanceType: "virtual-machine", Resources: common.ReadinessResources{CPU: 2, MemoryBytes: 2 << 30, DiskBytes: 10 << 30, VMSlots: 1}}
	ready, err := readiness.Readiness(ctx, req)
	require.NoError(t, err)
	require.True(t, ready.Ready, "readiness failed: %+v", ready)
	require.NoError(t, common.ValidateReadiness(req, ready, time.Now()))
	require.NotEmpty(t, ready.BackendIdentity)
	inventoryReq := common.AdmissionInventoryRequest{Version: 1, RequestID: "r2", ExpectedBackendIdentity: ready.BackendIdentity, Project: "test"}
	got, err := inv.AdmissionInventory(ctx, inventoryReq)
	require.NoError(t, err)
	require.True(t, got.Complete, got.Reason)
	require.Equal(t, inventoryReq, got.Request)
	require.Equal(t, ready.BackendIdentity, got.BackendIdentity)
	require.Empty(t, got.Instances)
	present.Store(true)
	got, err = inv.AdmissionInventory(ctx, inventoryReq)
	require.NoError(t, err)
	require.True(t, got.Complete, got.Reason)
	require.Equal(t, []common.AdmissionInventoryInstance{{Name: "guest-one", ProviderID: "guest-one"}}, got.Instances)
	badInventoryReq := inventoryReq
	badInventoryReq.ExpectedBackendIdentity = strings.Repeat("b", 64)
	badInventory, err := inv.AdmissionInventory(ctx, badInventoryReq)
	require.NoError(t, err)
	require.False(t, badInventory.Complete)
	absenceReq := common.AbsenceRequest{Version: 1, RequestID: "r3", Generation: "g1", Domain: "d1", Project: "test", InstanceName: "missing", ExpectedBackendIdentity: ready.BackendIdentity}
	missing, err := absence.ConfirmAuthoritativeAbsence(ctx, absenceReq)
	require.NoError(t, err)
	require.NoError(t, common.ValidateAbsence(absenceReq, missing, time.Now()))
	absenceReq.ExpectedBackendIdentity = strings.Repeat("b", 64)
	mismatch, err := absence.ConfirmAuthoritativeAbsence(ctx, absenceReq)
	require.NoError(t, err)
	require.False(t, mismatch.Absent)
	require.Error(t, common.ValidateAbsence(absenceReq, mismatch, time.Now()))
}
