package lxd

import (
	"context"
	"io/ioutil"
	"log"
	"runner-manager/config"
	"runner-manager/params"
	"runner-manager/util"
	"strings"

	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/pkg/errors"
)

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
	addresses := []string{}
	if state.Network != nil {
		for _, details := range state.Network {
			for _, addr := range details.Addresses {
				if addr.Scope != "global" {
					continue
				}
				addresses = append(addresses, addr.Address)
			}
		}
	}
	return params.Instance{
		OSArch:     instance.Architecture,
		ProviderID: instance.Name,
		Name:       instance.Name,
		OSType:     osType,
		OSName:     strings.ToLower(os),
		OSVersion:  osRelease,
		Addresses:  addresses,
		Status:     state.Status,
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
