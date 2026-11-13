// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.
package config

import (
	"strings"
	"testing"
)

func TestAdmissionValidate(t *testing.T) {
	r := AdmissionResources{CPU: 1, MemoryBytes: 1024, DiskBytes: 1024, VMs: 1, Jobs: 1}
	valid := Admission{Enabled: true, Domain: "test", Project: "project", Image: strings.Repeat("a", 64), Flavor: "flavor", OSArch: "amd64", InstanceType: "virtual-machine", Provider: "provider", Generation: 1, Protocol: "vm-only-experimental", ExclusiveController: true, HostCapabilityApproved: true, ExternalEnvelopeReserved: true, Capacity: r, Runner: r}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Admission){
		"protocol":   func(a *Admission) { a.Protocol = "automatic" },
		"controller": func(a *Admission) { a.ExclusiveController = false },
		"capability": func(a *Admission) { a.HostCapabilityApproved = false },
		"envelope":   func(a *Admission) { a.ExternalEnvelopeReserved = false },
		"size":       func(a *Admission) { a.Runner.VMs = 2 },
	} {
		t.Run(name, func(t *testing.T) {
			value := valid
			change(&value)
			if value.Validate() == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
	if err := (Admission{}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionProviderTimeout(t *testing.T) {
	cfg := Config{Admission: Admission{Enabled: true, Provider: "provider"}, Providers: []Provider{{Name: "provider"}}}
	if cfg.validateAdmissionProvider() == nil {
		t.Fatal("zero timeout must be rejected")
	}
	cfg.Providers[0].External.ExecTimeoutSeconds = 1201
	if cfg.validateAdmissionProvider() == nil {
		t.Fatal("timeout over twenty minutes must be rejected")
	}
	cfg.Providers[0].External.ExecTimeoutSeconds = 1200
	if err := cfg.validateAdmissionProvider(); err != nil {
		t.Fatal(err)
	}
	cfg.Admission.Enabled = false
	cfg.Providers[0].External.ExecTimeoutSeconds = 0
	if err := cfg.validateAdmissionProvider(); err != nil {
		t.Fatal(err)
	}
}
