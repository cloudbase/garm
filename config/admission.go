// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

package config

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// Admission opts one provider into experimental resource accounting. The
// operator must reserve an envelope for every workload outside this controller.
type Admission struct {
	Enabled                  bool               `toml:"enabled" json:"enabled"`
	Domain                   string             `toml:"domain" json:"domain"`
	Project                  string             `toml:"project" json:"project"`
	Image                    string             `toml:"image" json:"image"`
	Flavor                   string             `toml:"flavor" json:"flavor"`
	OSArch                   string             `toml:"os_arch" json:"os_arch"`
	InstanceType             string             `toml:"instance_type" json:"instance_type"`
	Provider                 string             `toml:"provider" json:"provider"`
	ActivateGeneration       int64              `toml:"activate_generation" json:"activate_generation"`
	Generation               int64              `toml:"generation" json:"generation"`
	Protocol                 string             `toml:"protocol" json:"protocol"`
	ExclusiveController      bool               `toml:"exclusive_controller" json:"exclusive_controller"`
	HostCapabilityApproved   bool               `toml:"host_capability_approved" json:"host_capability_approved"`
	ExternalEnvelopeReserved bool               `toml:"external_envelope_reserved" json:"external_envelope_reserved"`
	Capacity                 AdmissionResources `toml:"capacity" json:"capacity"`
	Runner                   AdmissionResources `toml:"runner" json:"runner"`
}

type AdmissionResources struct {
	CPU         int64 `toml:"cpu" json:"cpu"`
	MemoryBytes int64 `toml:"memory_bytes" json:"memory_bytes"`
	DiskBytes   int64 `toml:"disk_bytes" json:"disk_bytes"`
	VMs         int64 `toml:"vms" json:"vms"`
	Jobs        int64 `toml:"jobs" json:"jobs"`
}

func (a Admission) Validate() error {
	if !a.Enabled {
		return nil
	}
	if a.Domain == "" || a.Provider == "" || a.Generation <= 0 {
		return fmt.Errorf("admission requires domain, provider, and positive generation")
	}
	image, err := hex.DecodeString(a.Image)
	if err != nil || len(image) != 32 || a.Image != strings.ToLower(a.Image) || a.InstanceType != "virtual-machine" || a.Project == "" || a.Flavor == "" || a.OSArch == "" || a.InstanceType == "" {
		return fmt.Errorf("admission requires project, exact image fingerprint, flavor, architecture, and instance type")
	}
	if a.ActivateGeneration != 0 && a.ActivateGeneration != a.Generation {
		return fmt.Errorf("activate_generation must equal the configured generation")
	}
	if a.Protocol != "vm-only-experimental" {
		return fmt.Errorf("unsupported admission protocol %q; automatic assignment and legacy acquisition are not yet verified", a.Protocol)
	}
	if !a.ExclusiveController || !a.HostCapabilityApproved || !a.ExternalEnvelopeReserved {
		return fmt.Errorf("admission requires exclusive controller, host capability approval, and a reserved external workload envelope")
	}
	for _, r := range []AdmissionResources{a.Capacity, a.Runner} {
		if r.CPU <= 0 || r.MemoryBytes <= 0 || r.DiskBytes <= 0 || r.VMs <= 0 || r.Jobs <= 0 {
			return fmt.Errorf("admission resources must all be positive")
		}
	}
	if a.Runner.VMs != 1 {
		return fmt.Errorf("admission runner must consume exactly one VM slot")
	}
	return nil
}

func (c *Config) validateAdmissionProvider() error {
	if !c.Admission.Enabled {
		return nil
	}
	for _, provider := range c.Providers {
		if provider.Name != c.Admission.Provider {
			continue
		}
		if provider.External.ExecTimeoutSeconds == 0 || provider.External.ExecTimeoutSeconds > 1200 {
			return fmt.Errorf("admission provider exec_timeout_seconds must be between 1 and 1200")
		}
		return nil
	}
	return fmt.Errorf("admission provider %q is not configured", c.Admission.Provider)
}
