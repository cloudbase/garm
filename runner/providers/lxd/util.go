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

package lxd

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"garm/config"
	"garm/params"
	"garm/runner/providers/common"
	"garm/util"

	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/pkg/errors"
)

var (
	errInstanceIsStopped error = fmt.Errorf("The instance is already stopped")
)

var httpResponseErrors = map[int][]error{
	http.StatusNotFound: {os.ErrNotExist, sql.ErrNoRows},
}

// isNotFoundError returns true if the error is considered a Not Found error.
func isNotFoundError(err error) bool {
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return true
	}

	for _, checkErr := range httpResponseErrors[http.StatusNotFound] {
		if errors.Is(err, checkErr) {
			return true
		}
	}

	return false
}

func lxdInstanceToAPIInstance(instance *api.InstanceFull) params.Instance {
	os, ok := instance.ExpandedConfig["image.os"]
	if !ok {
		log.Printf("failed to find OS in instance config")
	}

	osType, err := util.OSToOSType(os)
	if err != nil {
		log.Printf("failed to find OS type for OS %s", os)
	}
	osRelease, ok := instance.ExpandedConfig["image.release"]
	if !ok {
		log.Printf("failed to find OS release instance config")
	}

	state := instance.State
	addresses := []params.Address{}
	if state.Network != nil {
		for _, details := range state.Network {
			for _, addr := range details.Addresses {
				if addr.Scope != "global" {
					continue
				}
				addresses = append(addresses, params.Address{
					Address: addr.Address,
					Type:    params.PublicAddress,
				})
			}
		}
	}

	instanceArch, ok := lxdToConfigArch[instance.Architecture]
	if !ok {
		log.Printf("failed to find OS architecture")
	}

	return params.Instance{
		OSArch:     instanceArch,
		ProviderID: instance.Name,
		Name:       instance.Name,
		OSType:     osType,
		OSName:     strings.ToLower(os),
		OSVersion:  osRelease,
		Addresses:  addresses,
		Status:     lxdStatusToProviderStatus(state.Status),
	}
}

func lxdStatusToProviderStatus(status string) common.InstanceStatus {
	switch status {
	case "Running":
		return common.InstanceRunning
	case "Stopped":
		return common.InstanceStopped
	default:
		return common.InstanceStatusUnknown
	}
}

func getClientFromConfig(ctx context.Context, cfg *config.LXD) (cli lxd.InstanceServer, err error) {
	if cfg.UnixSocket != "" {
		return lxd.ConnectLXDUnixWithContext(ctx, cfg.UnixSocket, nil)
	}

	var srvCrtContents, tlsCAContents, clientCertContents, clientKeyContents []byte

	if cfg.TLSServerCert != "" {
		srvCrtContents, err = ioutil.ReadFile(cfg.TLSServerCert)
		if err != nil {
			return nil, errors.Wrap(err, "reading TLSServerCert")
		}
	}

	if cfg.TLSCA != "" {
		tlsCAContents, err = ioutil.ReadFile(cfg.TLSCA)
		if err != nil {
			return nil, errors.Wrap(err, "reading TLSCA")
		}
	}

	if cfg.ClientCertificate != "" {
		clientCertContents, err = ioutil.ReadFile(cfg.ClientCertificate)
		if err != nil {
			return nil, errors.Wrap(err, "reading ClientCertificate")
		}
	}

	if cfg.ClientKey != "" {
		clientKeyContents, err = ioutil.ReadFile(cfg.ClientKey)
		if err != nil {
			return nil, errors.Wrap(err, "reading ClientKey")
		}
	}

	connectArgs := lxd.ConnectionArgs{
		TLSServerCert: string(srvCrtContents),
		TLSCA:         string(tlsCAContents),
		TLSClientCert: string(clientCertContents),
		TLSClientKey:  string(clientKeyContents),
	}
	return lxd.ConnectLXD(cfg.URL, &connectArgs)
}

func projectName(cfg config.LXD) string {
	if cfg.ProjectName != "" {
		return cfg.ProjectName
	}
	return DefaultProjectName
}

func resolveArchitecture(osArch config.OSArch) (string, error) {
	if string(osArch) == "" {
		return configToLXDArchMap[config.Amd64], nil
	}
	arch, ok := configToLXDArchMap[osArch]
	if !ok {
		return "", fmt.Errorf("architecture %s is not supported", osArch)
	}
	return arch, nil
}
