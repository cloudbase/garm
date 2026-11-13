// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

package scaleset

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

type readinessStore struct {
	dbCommon.Store
	dbCommon.AdmissionStore
	last     params.AdmissionConfig
	released string
}

func (*readinessStore) ListAllScaleSets(context.Context) ([]params.ScaleSet, error) { return nil, nil }

func (s *readinessStore) ReleaseUnsentAdmission(_ context.Context, id string) error {
	s.released = id
	return nil
}

func (s *readinessStore) UpdateAdmissionHealth(_ context.Context, c params.AdmissionConfig) error {
	s.last = c
	return nil
}

type readinessProvider struct {
	common.Provider
	now    *time.Time
	failed bool
}

func (p *readinessProvider) Readiness(_ context.Context, req common.ReadinessRequest) (common.ReadinessResponse, error) {
	if p.failed {
		return common.ReadinessResponse{}, errors.New("offline")
	}
	return common.ReadinessResponse{Request: req, Ready: true, CheckedAt: *p.now, ValidUntil: p.now.Add(30 * time.Second), ImageFingerprint: req.Image, BackendIdentity: strings.Repeat("b", 64), ProvisioningContract: "incus-single-node-sync-v1", Checks: common.ReadinessChecks{Project: true, Profile: true, Image: true, Architecture: true, Resources: true, ResourcesEnforced: true, CreateWaitsCompletion: true, Server: true, ProjectLimits: true}}, nil
}

func (p *readinessProvider) AdmissionInventory(_ context.Context, req common.AdmissionInventoryRequest) (common.AdmissionInventoryResponse, error) {
	return common.AdmissionInventoryResponse{Request: req, BackendIdentity: req.ExpectedBackendIdentity, Complete: true}, nil
}

func TestAdmissionReadinessClosesAndRecovers(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	image := strings.Repeat("a", 64)
	policy := params.AdmissionConfig{Domain: "d", Provider: "p", Generation: 1, Project: "project", Image: image, Flavor: "profile", OSArch: "amd64", InstanceType: "virtual-machine", Class: params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1, Jobs: 1}}
	provider := &readinessProvider{now: &now}
	store := &readinessStore{}
	worker := &Worker{ctx: context.Background(), provider: provider, store: store, scaleSet: params.ScaleSet{Image: image, Flavor: "profile", OSArch: "amd64"}, readinessNow: func() time.Time { return now }}
	if err := worker.checkAdmissionReadiness(store, policy); err == nil || store.last.Ready {
		t.Fatal("first probe must remain closed")
	}
	now = now.Add(time.Second)
	if err := worker.checkAdmissionReadiness(store, policy); err != nil || !store.last.Ready {
		t.Fatalf("second probe: ready=%v err=%v", store.last.Ready, err)
	}
	provider.failed = true
	if err := worker.checkAdmissionReadiness(store, policy); err == nil || store.last.Ready {
		t.Fatal("probe failure must close readiness")
	}
	provider.failed = false
	now = now.Add(time.Second)
	if err := worker.checkAdmissionReadiness(store, policy); err == nil || store.last.Ready {
		t.Fatal("recovery must wait for another probe")
	}
}

func TestFailedJITReleasesOnlyUnsentReservation(t *testing.T) {
	store := &readinessStore{}
	worker := &Worker{ctx: context.Background(), store: store}
	_, err := worker.generateJITWithAdmission("reservation", func() (params.RunnerScaleSetJitRunnerConfig, error) {
		return params.RunnerScaleSetJitRunnerConfig{}, errors.New("forge unavailable")
	})
	if err == nil || store.released != "reservation" {
		t.Fatalf("failed JIT: error=%v released=%q", err, store.released)
	}
	store.released = ""
	_, err = worker.generateJITWithAdmission("reservation", func() (params.RunnerScaleSetJitRunnerConfig, error) {
		return params.RunnerScaleSetJitRunnerConfig{}, nil
	})
	if err != nil || store.released != "" {
		t.Fatalf("successful JIT released reservation: %q", store.released)
	}
}
