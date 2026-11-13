// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.
package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cloudbase/garm/config"
	dbcommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

type bootstrapDB struct {
	dbcommon.Store
	dbcommon.AdmissionStore
	drained bool
	policy  params.AdmissionConfig
}

func (s *bootstrapDB) AdmissionForProvider(context.Context, string) (params.AdmissionConfig, bool, error) {
	return s.policy, s.policy.Domain != "", nil
}

func (s *bootstrapDB) ConfigureAdmission(_ context.Context, p params.AdmissionConfig) error {
	s.policy = p
	return nil
}
func (*bootstrapDB) UpdateAdmissionHealth(context.Context, params.AdmissionConfig) error { return nil }
func (s *bootstrapDB) SetAdmissionDrain(_ context.Context, _ string, d bool) error {
	s.drained = d
	return nil
}
func (*bootstrapDB) ListAllPools(context.Context) ([]params.Pool, error)         { return nil, nil }
func (*bootstrapDB) ListAllScaleSets(context.Context) ([]params.ScaleSet, error) { return nil, nil }

type bootstrapProvider struct {
	common.Provider
	listErr error
	guests  []common.AdmissionInventoryInstance
}

func (p *bootstrapProvider) Readiness(_ context.Context, req common.ReadinessRequest) (common.ReadinessResponse, error) {
	now := time.Now()
	return common.ReadinessResponse{Request: req, Ready: true, CheckedAt: now, ValidUntil: now.Add(30 * time.Second), ImageFingerprint: req.Image, BackendIdentity: strings.Repeat("b", 64), ProvisioningContract: "incus-single-node-sync-v1", Checks: common.ReadinessChecks{Project: true, Profile: true, Image: true, Architecture: true, Resources: true, ResourcesEnforced: true, CreateWaitsCompletion: true, Server: true, ProjectLimits: true}}, nil
}

func (p *bootstrapProvider) AdmissionInventory(_ context.Context, req common.AdmissionInventoryRequest) (common.AdmissionInventoryResponse, error) {
	return common.AdmissionInventoryResponse{Request: req, BackendIdentity: req.ExpectedBackendIdentity, Complete: true, Instances: p.guests}, p.listErr
}

func TestAdmissionBootstrapRejectsUnknownOrUnavailableInventory(t *testing.T) {
	cfg := config.Admission{Enabled: true, Domain: "d", Provider: "p", Generation: 1, Project: "project", Image: strings.Repeat("a", 64), Flavor: "profile", OSArch: "amd64", InstanceType: "virtual-machine", Runner: config.AdmissionResources{CPU: 1, MemoryBytes: 1, DiskBytes: 1, VMs: 1, Jobs: 1}}
	cases := []struct {
		name     string
		provider *bootstrapProvider
		drained  bool
	}{
		{"unknown guest", &bootstrapProvider{guests: []common.AdmissionInventoryInstance{{Name: "unknown"}}}, true},
		{"inventory unavailable", &bootstrapProvider{listErr: errors.New("offline")}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := &bootstrapDB{}
			err := initializeAdmission(context.Background(), cfg, db, map[string]common.Provider{"p": tc.provider})
			if err != nil || db.drained != tc.drained {
				t.Fatalf("err=%v drained=%v", err, db.drained)
			}
		})
	}
}
