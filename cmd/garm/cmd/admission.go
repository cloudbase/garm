// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/config"
	dbcommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

func admissionVector(r config.AdmissionResources) params.AdmissionVector {
	return params.AdmissionVector{CPU: r.CPU, RAM: r.MemoryBytes, Disk: r.DiskBytes, VMs: r.VMs, Jobs: r.Jobs}
}

// initializeAdmission runs before any provisioning worker. An enrollment that
// cannot be fully audited remains drained and cannot authorize a create.
func initializeAdmission(ctx context.Context, cfg config.Admission, db dbcommon.Store, providers map[string]common.Provider) error {
	store, ok := db.(dbcommon.AdmissionStore)
	if !ok {
		if cfg.Enabled {
			return fmt.Errorf("database does not support admission")
		}
		return nil
	}
	for name := range providers {
		policy, enrolled, err := store.AdmissionForProvider(ctx, name)
		if err != nil {
			return err
		}
		if enrolled && (!cfg.Enabled || cfg.Provider != name || cfg.Domain != policy.Domain) {
			return fmt.Errorf("persisted admission enrollment %s requires matching configuration", name)
		}
	}
	if !cfg.Enabled {
		return nil
	}
	provider, ok := providers[cfg.Provider]
	if !ok {
		return fmt.Errorf("admission provider %s not found", cfg.Provider)
	}
	probe, ok := provider.(common.ReadinessProvider)
	if !ok {
		return fmt.Errorf("admission provider does not support readiness_v1")
	}
	policy := params.AdmissionConfig{Domain: cfg.Domain, Provider: cfg.Provider, Generation: cfg.Generation, Protocol: cfg.Protocol, RequestedActivationGeneration: cfg.ActivateGeneration, Project: cfg.Project, Image: cfg.Image, Flavor: cfg.Flavor, OSArch: cfg.OSArch, InstanceType: cfg.InstanceType, Capacity: admissionVector(cfg.Capacity), Class: admissionVector(cfg.Runner)}
	// Invalidate any health retained from a previous process before workers start.
	if err := store.ConfigureAdmission(ctx, policy); err != nil {
		return fmt.Errorf("configure admission: %w", err)
	}
	policy, _, err := store.AdmissionForProvider(ctx, cfg.Provider)
	if err != nil {
		return fmt.Errorf("reading admission policy: %w", err)
	}
	healthFailure := func(err error) error {
		policy.Ready, policy.HealthyUntil, policy.HealthObservedAt = false, time.Time{}, time.Now()
		if updateErr := store.UpdateAdmissionHealth(ctx, policy); updateErr != nil {
			return fmt.Errorf("%v; closing health: %w", err, updateErr)
		}
		slog.WarnContext(ctx, "admission health unavailable; enrolled provider remains closed", "error", err)
		return nil
	}
	closeDomain := func(err error) error {
		if drainErr := store.SetAdmissionDrain(ctx, cfg.Domain, true); drainErr != nil {
			return fmt.Errorf("%v; draining domain: %w", err, drainErr)
		}
		slog.ErrorContext(ctx, "admission domain is drained", "error", err)
		return nil
	}
	var gate common.ReadinessGate
	for index := 0; index < 2; index++ {
		req := common.ReadinessRequest{ExpectedBackendIdentity: policy.BackendIdentity, Version: 1, RequestID: util.NewID(), Generation: fmt.Sprint(cfg.Generation), Domain: cfg.Domain, Project: cfg.Project, Image: cfg.Image, Flavor: cfg.Flavor, OSArch: cfg.OSArch, InstanceType: cfg.InstanceType, Resources: common.ReadinessResources{CPU: cfg.Runner.CPU, MemoryBytes: cfg.Runner.MemoryBytes, DiskBytes: cfg.Runner.DiskBytes, VMSlots: cfg.Runner.VMs}}
		observedAt := time.Now()
		response, err := probe.Readiness(ctx, req)
		if err != nil {
			return healthFailure(fmt.Errorf("admission readiness: %w", err))
		}
		gateErr := gate.Observe(req, response, time.Now())
		if index == 0 {
			if common.ValidateReadiness(req, response, time.Now()) != nil {
				return healthFailure(fmt.Errorf("invalid first readiness probe"))
			}
			policy.BackendIdentity = response.BackendIdentity
			fault, err := auditAdmissionBootstrap(ctx, cfg, db, store, provider, policy)
			if err != nil {
				if fault {
					return closeDomain(err)
				}
				return healthFailure(err)
			}
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return closeDomain(ctx.Err())
			case <-timer.C:
			}
			continue
		}
		if gateErr != nil {
			return healthFailure(fmt.Errorf("admission readiness: %w", gateErr))
		}
		policy.Ready, policy.HealthyUntil, policy.HealthObservedAt = true, response.ValidUntil, observedAt
	}
	if err := store.UpdateAdmissionHealth(ctx, policy); err != nil {
		return closeDomain(err)
	}
	return resumeBootstrapAdmission(ctx, cfg, store)
}

func auditAdmissionBootstrap(ctx context.Context, cfg config.Admission, db dbcommon.Store, store dbcommon.AdmissionStore, provider common.Provider, policy params.AdmissionConfig) (bool, error) {
	pools, err := db.ListAllPools(ctx)
	if err != nil {
		return true, (err)
	}
	for _, pool := range pools {
		if pool.ProviderName == cfg.Provider {
			return true, (fmt.Errorf("admission does not support pools on enrolled provider"))
		}
	}
	sets, err := db.ListAllScaleSets(ctx)
	if err != nil {
		return true, (err)
	}
	known := make(map[string]params.Instance)
	for _, set := range sets {
		if set.ProviderName != cfg.Provider {
			continue
		}
		if set.MinIdleRunners != 0 || set.Image != cfg.Image || set.Flavor != cfg.Flavor || string(set.OSArch) != cfg.OSArch {
			return true, (fmt.Errorf("scale set %d has unsupported admission shape", set.ID))
		}
		instances, err := db.ListScaleSetInstances(ctx, set.ID, false)
		if err != nil {
			return true, (err)
		}
		for _, instance := range instances {
			if _, exists := known[instance.Name]; exists {
				return true, (fmt.Errorf("duplicate instance name in admission inventory"))
			}
			reservation, err := store.AdmissionByInstance(ctx, cfg.Provider, instance.Name)
			if err != nil && !errors.Is(err, dbcommon.ErrAdmissionNotFound) {
				return false, fmt.Errorf("reading admission reservation: %w", err)
			}
			if err != nil || reservation.State == "released" || reservation.BackendIdentity == "" || reservation.BackendIdentity != policy.BackendIdentity || reservation.Owner != fmt.Sprintf("scaleset:%d", set.ID) {
				return true, fmt.Errorf("unaccounted instance %s", instance.Name)
			}
			known[instance.Name] = instance
		}
	}
	remote, err := fetchAdmissionInventory(ctx, provider, policy)
	if err != nil {
		return false, err
	}
	for _, guest := range remote {
		instance, ok := known[guest.Name]
		if !ok || (instance.ProviderID != "" && guest.ProviderID != instance.ProviderID) {
			return true, (fmt.Errorf("unaccounted provider instance %s", guest.Name))
		}
		delete(known, guest.Name)
	}
	if len(known) != 0 {
		return true, fmt.Errorf("admission inventory is incomplete or has unresolved creates")
	}
	return false, nil
}

func resumeBootstrapAdmission(ctx context.Context, cfg config.Admission, store dbcommon.AdmissionStore) error {
	stored, _, err := store.AdmissionForProvider(ctx, cfg.Provider)
	if err != nil {
		return err
	}
	if stored.Drained {
		if cfg.ActivateGeneration != cfg.Generation || cfg.ActivateGeneration <= stored.LastResumeGeneration {
			slog.WarnContext(ctx, "admission domain remains drained; a higher explicit activate_generation is required", "domain", cfg.Domain)
			return nil
		}
		if err := store.ResumeAdmission(ctx, cfg.Domain, cfg.Generation); err != nil {
			return err
		}
	}
	return nil
}

func fetchAdmissionInventory(ctx context.Context, provider common.Provider, policy params.AdmissionConfig) ([]common.AdmissionInventoryInstance, error) {
	inventory, ok := provider.(common.AdmissionInventoryProvider)
	if !ok {
		return nil, fmt.Errorf("provider does not support identity-bound inventory")
	}
	req := common.AdmissionInventoryRequest{Version: 1, RequestID: util.NewID(), ExpectedBackendIdentity: policy.BackendIdentity, Project: policy.Project}
	response, err := inventory.AdmissionInventory(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("listing admission inventory: %w", err)
	}
	if response.Request != req || !response.Complete || response.BackendIdentity != req.ExpectedBackendIdentity || req.ExpectedBackendIdentity == "" {
		return nil, fmt.Errorf("invalid admission inventory response")
	}
	return response.Instances, nil
}
