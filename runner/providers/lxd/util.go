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
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"

	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/config"

	"github.com/juju/clock"
	"github.com/juju/retry"
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/pkg/errors"
)

var (
	//lint:ignore ST1005 imported error from lxd
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

func lxdInstanceToAPIInstance(instance *api.InstanceFull) commonParams.ProviderInstance {
	lxdOS, ok := instance.ExpandedConfig["image.os"]
	if !ok {
		log.Printf("failed to find OS in instance config")
	}

	osType, err := util.OSToOSType(lxdOS)
	if err != nil {
		log.Printf("failed to find OS type for OS %s", lxdOS)
	}

	if osType == "" {
		osTypeFromTag, ok := instance.ExpandedConfig[osTypeKeyName]
		if !ok {
			log.Printf("failed to find OS type in fallback location")
		}
		osType = commonParams.OSType(osTypeFromTag)
	}

	osRelease, ok := instance.ExpandedConfig["image.release"]
	if !ok {
		log.Printf("failed to find OS release instance config")
	}

	state := instance.State
	addresses := []commonParams.Address{}
	if state.Network != nil {
		for _, details := range state.Network {
			for _, addr := range details.Addresses {
				if addr.Scope != "global" {
					continue
				}
				addresses = append(addresses, commonParams.Address{
					Address: addr.Address,
					Type:    commonParams.PublicAddress,
				})
			}
		}
	}

	instanceArch, ok := lxdToConfigArch[instance.Architecture]
	if !ok {
		log.Printf("failed to find OS architecture")
	}

	return commonParams.ProviderInstance{
		OSArch:     instanceArch,
		ProviderID: instance.Name,
		Name:       instance.Name,
		OSType:     osType,
		OSName:     strings.ToLower(lxdOS),
		OSVersion:  osRelease,
		Addresses:  addresses,
		Status:     lxdStatusToProviderStatus(state.Status),
	}
}

func lxdStatusToProviderStatus(status string) commonParams.InstanceStatus {
	switch status {
	case "Running":
		return commonParams.InstanceRunning
	case "Stopped":
		return commonParams.InstanceStopped
	default:
		return commonParams.InstanceStatusUnknown
	}
}

func getClientFromConfig(ctx context.Context, cfg *config.LXD) (cli lxd.InstanceServer, err error) {
	if cfg.UnixSocket != "" {
		return lxd.ConnectLXDUnixWithContext(ctx, cfg.UnixSocket, nil)
	}

	var srvCrtContents, tlsCAContents, clientCertContents, clientKeyContents []byte

	if cfg.TLSServerCert != "" {
		srvCrtContents, err = os.ReadFile(cfg.TLSServerCert)
		if err != nil {
			return nil, errors.Wrap(err, "reading TLSServerCert")
		}
	}

	if cfg.TLSCA != "" {
		tlsCAContents, err = os.ReadFile(cfg.TLSCA)
		if err != nil {
			return nil, errors.Wrap(err, "reading TLSCA")
		}
	}

	if cfg.ClientCertificate != "" {
		clientCertContents, err = os.ReadFile(cfg.ClientCertificate)
		if err != nil {
			return nil, errors.Wrap(err, "reading ClientCertificate")
		}
	}

	if cfg.ClientKey != "" {
		clientKeyContents, err = os.ReadFile(cfg.ClientKey)
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

	lxdCLI, err := lxd.ConnectLXD(cfg.URL, &connectArgs)
	if err != nil {
		return nil, errors.Wrap(err, "connecting to LXD")
	}

	return lxdCLI, nil
}

func projectName(cfg config.LXD) string {
	if cfg.ProjectName != "" {
		return cfg.ProjectName
	}
	return DefaultProjectName
}

func resolveArchitecture(osArch commonParams.OSArch) (string, error) {
	if string(osArch) == "" {
		return configToLXDArchMap[commonParams.Amd64], nil
	}
	arch, ok := configToLXDArchMap[osArch]
	if !ok {
		return "", fmt.Errorf("architecture %s is not supported", osArch)
	}
	return arch, nil
}

// waitDeviceActive is a function capable of figuring out when a Equinix Metal
// device is active
func (l *LXD) waitInstanceHasIP(ctx context.Context, instanceName string) (commonParams.ProviderInstance, error) {
	var p commonParams.ProviderInstance
	var errIPNotFound error = fmt.Errorf("ip not found")
	err := retry.Call(retry.CallArgs{
		Func: func() error {
			var err error
			p, err = l.GetInstance(ctx, instanceName)
			if err != nil {
				return errors.Wrap(err, "fetching instance")
			}
			for _, addr := range p.Addresses {
				ip := net.ParseIP(addr.Address)
				if ip == nil {
					continue
				}
				if ip.To4() == nil {
					continue
				}
				return nil
			}
			return errIPNotFound
		},
		Attempts: 20,
		Delay:    5 * time.Second,
		Clock:    clock.WallClock,
	})

	if err != nil && err != errIPNotFound {
		return commonParams.ProviderInstance{}, err
	}

	return p, nil
}
