// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

package common

import (
	"strings"
	"testing"
	"time"
)

func TestValidateReadiness(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	req := ReadinessRequest{Version: 1, RequestID: "request", Generation: "generation", Domain: "domain", Project: "p", Flavor: "f", OSArch: "amd64", InstanceType: "virtual-machine", Image: strings.Repeat("a", 64), Resources: ReadinessResources{CPU: 1, MemoryBytes: 1, DiskBytes: 1, VMSlots: 1}}
	valid := ReadinessResponse{Request: req, Ready: true, CheckedAt: now.Add(-time.Second), ValidUntil: now.Add(10 * time.Second), ImageFingerprint: req.Image, BackendIdentity: strings.Repeat("b", 64), ProvisioningContract: "incus-single-node-sync-v1", Checks: ReadinessChecks{true, true, true, true, true, true, true, true, true}}
	tests := []struct {
		name   string
		mutate func(*ReadinessResponse)
		ok     bool
	}{
		{"valid", func(*ReadinessResponse) {}, true},
		{"negative", func(r *ReadinessResponse) { r.Ready = false }, false},
		{"wrong request", func(r *ReadinessResponse) { r.Request.Generation = "old" }, false},
		{"stale", func(r *ReadinessResponse) { r.CheckedAt = now.Add(-46 * time.Second) }, false},
		{"future", func(r *ReadinessResponse) { r.CheckedAt = now.Add(time.Second) }, false},
		{"expired", func(r *ReadinessResponse) { r.ValidUntil = now }, false},
		{"excessive validity", func(r *ReadinessResponse) { r.ValidUntil = now.Add(time.Minute) }, false},
		{"unknown image", func(r *ReadinessResponse) { r.ImageFingerprint = "" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valid
			tt.mutate(&result)
			err := ValidateReadiness(req, result, now)
			if (err == nil) != tt.ok {
				t.Fatalf("ValidateReadiness() error = %v, want success %v", err, tt.ok)
			}
		})
	}
}

func TestReadinessRecovery(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	req := ReadinessRequest{Version: 1, RequestID: "first", Generation: "a", Domain: "d", Project: "p", Flavor: "f", OSArch: "amd64", InstanceType: "virtual-machine", Image: strings.Repeat("a", 64), Resources: ReadinessResources{CPU: 1, MemoryBytes: 1, DiskBytes: 1, VMSlots: 1}}
	response := ReadinessResponse{Request: req, Ready: true, CheckedAt: now, ValidUntil: now.Add(10 * time.Second), ImageFingerprint: req.Image, BackendIdentity: strings.Repeat("b", 64), ProvisioningContract: "incus-single-node-sync-v1", Checks: ReadinessChecks{true, true, true, true, true, true, true, true, true}}
	var gate ReadinessGate
	if gate.Observe(req, response, now) == nil {
		t.Fatal("first check must not open gate")
	}
	if gate.Observe(req, response, now.Add(time.Second)) == nil {
		t.Fatal("replayed check must not open gate")
	}
	req.RequestID = "second"
	response.Request = req
	if err := gate.Observe(req, response, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	response.Ready = false
	if gate.Observe(req, response, now.Add(2*time.Second)) == nil {
		t.Fatal("negative check must close gate")
	}
	response.Ready = true
	if gate.Observe(req, response, now.Add(3*time.Second)) == nil {
		t.Fatal("recovery must require two checks")
	}
}

func TestValidateAbsence(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	req := AbsenceRequest{Version: 1, RequestID: "request", Generation: "1", Domain: "domain", Project: "project", InstanceName: "runner", ExpectedBackendIdentity: strings.Repeat("b", 64)}
	valid := AbsenceResponse{Request: req, Absent: true, BackendIdentity: req.ExpectedBackendIdentity, Contract: "incus-single-node-absence-v1", CheckedAt: now, ValidUntil: now.Add(10 * time.Second)}
	if err := ValidateAbsence(req, valid, now); err != nil {
		t.Fatal(err)
	}
	bad := valid
	bad.BackendIdentity = strings.Repeat("c", 64)
	if ValidateAbsence(req, bad, now) == nil {
		t.Fatal("different backend must be rejected")
	}
	bad = valid
	bad.ValidUntil = now.Add(16 * time.Second)
	if ValidateAbsence(req, bad, now) == nil {
		t.Fatal("excessive validity must be rejected")
	}
	bad = valid
	bad.Absent = false
	if ValidateAbsence(req, bad, now) == nil {
		t.Fatal("unknown absence must be rejected")
	}
}
