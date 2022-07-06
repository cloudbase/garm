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

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func getDefaultLXDImageRemoteConfig() LXDImageRemote {
	return LXDImageRemote{
		Address:            "https://cloud-images.ubuntu.com/releases",
		Public:             true,
		Protocol:           SimpleStreams,
		InsecureSkipVerify: false,
	}
}

func getDefaultLXDConfig() LXD {
	remote := getDefaultLXDImageRemoteConfig()
	return LXD{
		URL:                   "https://example.com:8443",
		ProjectName:           "default",
		IncludeDefaultProfile: false,
		ClientCertificate:     "../testdata/lxd/certs/client.crt",
		ClientKey:             "../testdata/lxd/certs/client.key",
		TLSServerCert:         "../testdata/lxd/certs/servercert.crt",
		ImageRemotes: map[string]LXDImageRemote{
			"default": remote,
		},
		SecureBoot: false,
	}
}

func TestLXDRemote(t *testing.T) {
	cfg := getDefaultLXDImageRemoteConfig()

	err := cfg.Validate()
	require.Nil(t, err)
}

func TestLXDRemoteEmptyAddress(t *testing.T) {
	cfg := getDefaultLXDImageRemoteConfig()

	cfg.Address = ""

	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "missing address")
}

func TestLXDRemoteInvalidAddress(t *testing.T) {
	cfg := getDefaultLXDImageRemoteConfig()

	cfg.Address = "bogus address"
	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "validating address: parse \"bogus address\": invalid URI for request")
}

func TestLXDRemoteIvalidAddressScheme(t *testing.T) {
	cfg := getDefaultLXDImageRemoteConfig()

	cfg.Address = "ftp://whatever"
	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "address must be http or https")
}

func TestLXDConfig(t *testing.T) {
	cfg := getDefaultLXDConfig()
	err := cfg.Validate()
	require.Nil(t, err)
}

func TestLXDWithInvalidUnixSocket(t *testing.T) {
	cfg := getDefaultLXDConfig()

	cfg.UnixSocket = "bogus unix socket"
	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "could not access unix socket bogus unix socket: \"stat bogus unix socket: no such file or directory\"")
}

func TestMissingUnixSocketAndMissingURL(t *testing.T) {
	cfg := getDefaultLXDConfig()

	cfg.URL = ""
	cfg.UnixSocket = ""

	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "unix_socket or address must be specified")
}

func TestInvalidLXDURL(t *testing.T) {
	cfg := getDefaultLXDConfig()
	cfg.URL = "bogus"

	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "invalid LXD URL")
}

func TestLXDURLIsHTTPS(t *testing.T) {
	cfg := getDefaultLXDConfig()
	cfg.URL = "http://example.com"

	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "address must be https")
}

func TestMissingClientCertOrKey(t *testing.T) {
	cfg := getDefaultLXDConfig()
	cfg.ClientKey = ""
	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "client_certificate and client_key are mandatory")

	cfg = getDefaultLXDConfig()
	cfg.ClientCertificate = ""
	err = cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "client_certificate and client_key are mandatory")
}

func TestLXDIvalidCertOrKeyPaths(t *testing.T) {
	cfg := getDefaultLXDConfig()
	cfg.ClientCertificate = "/i/am/not/here"
	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "failed to access client certificate /i/am/not/here: \"stat /i/am/not/here: no such file or directory\"")

	cfg.ClientCertificate = "../testdata/lxd/certs/client.crt"
	cfg.ClientKey = "/me/neither"

	err = cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "failed to access client key /me/neither: \"stat /me/neither: no such file or directory\"")
}

func TestLXDInvalidServerCertPath(t *testing.T) {
	cfg := getDefaultLXDConfig()
	cfg.TLSServerCert = "/not/a/valid/server/cert/path"

	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "failed to access tls_server_certificate /not/a/valid/server/cert/path: \"stat /not/a/valid/server/cert/path: no such file or directory\"")
}

func TestInvalidLXDImageRemotes(t *testing.T) {
	cfg := getDefaultLXDConfig()

	cfg.ImageRemotes["default"] = LXDImageRemote{
		Protocol: LXDRemoteProtocol("bogus"),
	}

	err := cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, "remote default is invalid: invalid remote protocol bogus. Supported protocols: simplestreams")
}
