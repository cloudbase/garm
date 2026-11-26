// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

package common

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ReadinessRequest identifies the exact configuration being checked. Generation
// must change whenever any admission-relevant provider configuration changes.
type ReadinessRequest struct {
	ExpectedBackendIdentity string             `json:"expected_backend_identity"`
	Version                 int                `json:"version"`
	RequestID               string             `json:"request_id"`
	Generation              string             `json:"generation"`
	Domain                  string             `json:"domain"`
	Project                 string             `json:"project"`
	Image                   string             `json:"image"`
	Flavor                  string             `json:"flavor"`
	OSArch                  string             `json:"os_arch"`
	InstanceType            string             `json:"instance_type"`
	Resources               ReadinessResources `json:"resources"`
}

type ReadinessResources struct {
	CPU         int64 `json:"cpu"`
	MemoryBytes int64 `json:"memory_bytes"`
	DiskBytes   int64 `json:"disk_bytes"`
	VMSlots     int64 `json:"vm_slots"`
}

// ReadinessChecks reports the facts established by a provider probe.
// A positive result is eligibility at observation time, not a guarantee that
// a later create will succeed.
type ReadinessChecks struct {
	Project               bool `json:"project"`
	Profile               bool `json:"profile"`
	Image                 bool `json:"image"`
	Architecture          bool `json:"architecture"`
	CreateWaitsCompletion bool `json:"create_waits_completion"`
	ResourcesEnforced     bool `json:"resources_enforced"`
	Resources             bool `json:"resources"`
	Server                bool `json:"server"`
	ProjectLimits         bool `json:"project_limits"`
}

type ReadinessResponse struct {
	ProvisioningContract string           `json:"provisioning_contract"`
	BackendIdentity      string           `json:"backend_identity"`
	Checks               ReadinessChecks  `json:"checks"`
	Request              ReadinessRequest `json:"request"`
	Ready                bool             `json:"ready"`
	CheckedAt            time.Time        `json:"checked_at"`
	ValidUntil           time.Time        `json:"valid_until"`
	ImageFingerprint     string           `json:"image_fingerprint"`
	Reason               string           `json:"reason,omitempty"`
}

// ReadinessProvider is an optional capability. Implementations unable to
// establish readiness must return an error or Ready=false.
type ReadinessProvider interface {
	Readiness(ctx context.Context, req ReadinessRequest) (ReadinessResponse, error)
}

// ValidateReadiness applies an admission-time freshness bound. It does not
// assert that the provider will remain healthy after this check.
func ValidateReadiness(req ReadinessRequest, result ReadinessResponse, now time.Time) error {
	image, imageErr := hex.DecodeString(req.Image)
	if !validReadinessIdentity(req) || imageErr != nil || len(image) != 32 || req.Image != strings.ToLower(req.Image) || !validReadinessResources(req.Resources) {
		return fmt.Errorf("invalid readiness request")
	}
	backend, backendErr := hex.DecodeString(result.BackendIdentity)
	if backendErr != nil || len(backend) != 32 || result.BackendIdentity != strings.ToLower(result.BackendIdentity) || (req.ExpectedBackendIdentity != "" && req.ExpectedBackendIdentity != result.BackendIdentity) {
		return fmt.Errorf("provider backend identity does not match")
	}
	if result.Request != req {
		return fmt.Errorf("readiness response does not match request")
	}
	if !result.Ready || result.ProvisioningContract != "incus-single-node-sync-v1" || result.ImageFingerprint != req.Image || result.Checks != (ReadinessChecks{true, true, true, true, true, true, true, true, true}) {
		return fmt.Errorf("provider is not ready")
	}
	if result.CheckedAt.IsZero() || result.CheckedAt.After(now) || now.Sub(result.CheckedAt) > 45*time.Second || !result.ValidUntil.After(now) || result.ValidUntil.After(result.CheckedAt.Add(45*time.Second)) {
		return fmt.Errorf("readiness response is stale or has invalid timestamps")
	}
	return nil
}

// ReadinessGate requires two distinct valid probes separated by at least one
// second after startup or any failed/stale probe. Its state is deliberately
// volatile: restarting requires recovery to be demonstrated again.
type ReadinessGate struct {
	mu              sync.Mutex
	generation      string
	backendIdentity string
	firstAt         time.Time
	firstRequestID  string
	validUntil      time.Time
	open            bool
}

// Observe validates a probe and advances recovery hysteresis. Callers must
// make the subsequent admission decision against a durable freshness bound.
func (g *ReadinessGate) Observe(req ReadinessRequest, response ReadinessResponse, now time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := ValidateReadiness(req, response, now); err != nil {
		g.generation = ""
		g.backendIdentity = ""
		g.firstAt = time.Time{}
		g.firstRequestID = ""
		g.validUntil = time.Time{}
		g.open = false
		return err
	}
	if g.generation != req.Generation || g.backendIdentity != response.BackendIdentity || !g.validUntil.After(now) {
		g.generation = req.Generation
		g.backendIdentity = response.BackendIdentity
		g.firstAt = now
		g.firstRequestID = req.RequestID
		g.validUntil = response.ValidUntil
		g.open = false
		return fmt.Errorf("readiness recovery pending")
	}
	g.validUntil = response.ValidUntil
	if !g.open && req.RequestID != g.firstRequestID && now.Sub(g.firstAt) >= time.Second {
		g.open = true
	}
	if !g.open {
		return fmt.Errorf("readiness recovery pending")
	}
	return nil
}

// Reset closes the gate after an independent inventory or capability failure.
func (g *ReadinessGate) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.generation = ""
	g.backendIdentity = ""
	g.firstAt = time.Time{}
	g.firstRequestID = ""
	g.validUntil = time.Time{}
	g.open = false
}

func validReadinessIdentity(req ReadinessRequest) bool {
	return req.Version == 1 && req.RequestID != "" && req.Generation != "" && req.Domain != "" && req.Project != "" && req.Flavor != "" && req.OSArch != "" && req.InstanceType == "virtual-machine"
}

func validReadinessResources(resources ReadinessResources) bool {
	return resources.CPU > 0 && resources.MemoryBytes > 0 && resources.DiskBytes > 0 && resources.VMSlots > 0
}
