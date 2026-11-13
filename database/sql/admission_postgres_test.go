//go:build testing

// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

package sql

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

func TestAdmissionPostgresConcurrency(t *testing.T) {
	if os.Getenv("GARM_TEST_POSTGRES_DSN") == "" {
		t.Skip("GARM_TEST_POSTGRES_DSN is not set")
	}
	raw, _ := garmTesting.OpenTestPostgresDB(t)
	raw.SetMaxOpenConns(20)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: raw}), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, initSchema(db))
	s := &sqlDatabase{conn: db, ctx: context.Background()}
	ctx := context.Background()
	class := params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1}
	policy := params.AdmissionConfig{Domain: "domain", Provider: "provider", Generation: 1, RequestedActivationGeneration: 1, Class: class, Capacity: params.AdmissionVector{CPU: 10, RAM: 10, Disk: 20, VMs: 20}}
	require.NoError(t, s.ConfigureAdmission(ctx, policy))
	policy.Ready = true
	policy.BackendIdentity = strings.Repeat("a", 64)
	policy.HealthObservedAt = time.Now().Add(-time.Second)
	policy.HealthyUntil = time.Now().Add(time.Hour)
	require.NoError(t, s.UpdateAdmissionHealth(ctx, policy))
	require.NoError(t, s.ResumeAdmission(ctx, "domain", 1))
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v := params.AdmissionVector{CPU: 2, RAM: 1, Disk: 1, VMs: 1}
			if i%2 == 1 {
				v.CPU = 1
				v.RAM = 2
			}
			_, err := s.ReserveAdmission(ctx, "domain", fmt.Sprintf("owner-%d", i), "key", v, 1)
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	require.Greater(t, successes, 0)
	require.LessOrEqual(t, successes, 6)
	var rows []admissionReservation
	require.NoError(t, db.Where("domain = ?", "domain").Find(&rows).Error)
	var cpu, ram int64
	for _, r := range rows {
		cpu += r.CPU
		ram += r.RAM
	}
	require.LessOrEqual(t, cpu, int64(10))
	require.LessOrEqual(t, ram, int64(10))
	other := policy
	other.Generation = 2
	other.RequestedActivationGeneration = 2
	other.BackendIdentity = strings.Repeat("b", 64)
	other.Capacity.CPU = 5
	require.NoError(t, s.ConfigureAdmission(ctx, other))
	other.HealthObservedAt = time.Now()
	require.Error(t, s.UpdateAdmissionHealth(ctx, other))
	current, _, err := s.AdmissionForProvider(ctx, "provider")
	require.NoError(t, err)
	require.Equal(t, policy.BackendIdentity, current.BackendIdentity)
	// A separate enrolled provider exercises the transaction that binds a
	// charge and inserts the scale-set instance under the domain lock.
	set := ScaleSet{Name: "admitted", ProviderName: "provider-2", Enabled: true}
	require.NoError(t, db.Create(&set).Error)
	second := params.AdmissionConfig{Domain: "domain-2", Provider: "provider-2", Generation: 1, RequestedActivationGeneration: 1, Class: class, Capacity: class}
	require.NoError(t, s.ConfigureAdmission(ctx, second))
	second.Ready = true
	second.BackendIdentity = policy.BackendIdentity
	second.HealthObservedAt = time.Now().Add(-time.Second)
	second.HealthyUntil = time.Now().Add(time.Hour)
	require.NoError(t, s.UpdateAdmissionHealth(ctx, second))
	require.NoError(t, s.ResumeAdmission(ctx, second.Domain, 1))
	owner := fmt.Sprintf("scaleset:%d", set.ID)
	charge, err := s.ReserveAdmission(ctx, second.Domain, owner, "request", class, 1)
	require.NoError(t, err)
	_, err = s.CreateScaleSetInstance(ctx, set.ID, params.CreateInstanceParams{Name: "unbound"})
	require.Error(t, err)
	_, err = s.CreateAdmittedScaleSetInstance(ctx, charge.ID, set.ID, params.CreateInstanceParams{Name: "bound"})
	require.NoError(t, err)
	authorized, enrolled, err := s.BeginAdmittedCreate(ctx, second.Provider, owner, "bound", params.AdmissionCreateShape{Generation: set.Generation})
	require.NoError(t, err)
	require.True(t, enrolled)
	require.Equal(t, class, authorized.Vector)
	require.NoError(t, s.SetAdmissionDrain(ctx, "domain", true))
	require.Error(t, s.ResumeAdmission(ctx, "domain", 2))
}

func TestAdmissionPostgresEnrollmentRace(t *testing.T) {
	if os.Getenv("GARM_TEST_POSTGRES_DSN") == "" {
		t.Skip("GARM_TEST_POSTGRES_DSN is not set")
	}
	raw, _ := garmTesting.OpenTestPostgresDB(t)
	raw.SetMaxOpenConns(10)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: raw}), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, initSchema(db))
	s := &sqlDatabase{conn: db, ctx: context.Background()}
	ctx := context.Background()
	set := ScaleSet{Name: "synthetic", ProviderName: "provider", Enabled: true}
	require.NoError(t, db.Create(&set).Error)
	v := params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1}
	c := params.AdmissionConfig{Domain: "domain", Provider: "provider", Generation: 1, RequestedActivationGeneration: 1, Class: v, Capacity: v}
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	var createErr, configErr error
	go func() {
		defer wg.Done()
		<-start
		_, createErr = s.CreateScaleSetInstance(ctx, set.ID, params.CreateInstanceParams{Name: "runner", Status: commonParams.InstancePendingCreate, RunnerStatus: params.RunnerPending})
	}()
	go func() { defer wg.Done(); <-start; configErr = s.ConfigureAdmission(ctx, c) }()
	close(start)
	wg.Wait()
	require.NotEqual(t, createErr == nil, configErr == nil, "enrollment and an uncharged create cannot both succeed")
}

func TestAdmissionPostgresCancelVersusSend(t *testing.T) {
	if os.Getenv("GARM_TEST_POSTGRES_DSN") == "" {
		t.Skip("GARM_TEST_POSTGRES_DSN is not set")
	}
	raw, _ := garmTesting.OpenTestPostgresDB(t)
	raw.SetMaxOpenConns(10)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: raw}), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, initSchema(db))
	s := &sqlDatabase{conn: db, ctx: context.Background()}
	ctx := context.Background()
	set := ScaleSet{Name: "synthetic", ProviderName: "provider", Enabled: true}
	require.NoError(t, db.Create(&set).Error)
	v := params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1}
	c := params.AdmissionConfig{Domain: "domain", Provider: "provider", Generation: 1, RequestedActivationGeneration: 1, Class: v, Capacity: v}
	require.NoError(t, s.ConfigureAdmission(ctx, c))
	c.Ready = true
	c.BackendIdentity = strings.Repeat("a", 64)
	c.HealthObservedAt = time.Now().Add(-time.Second)
	c.HealthyUntil = time.Now().Add(time.Hour)
	require.NoError(t, s.UpdateAdmissionHealth(ctx, c))
	require.NoError(t, s.ResumeAdmission(ctx, c.Domain, 1))
	owner := fmt.Sprintf("scaleset:%d", set.ID)
	r, err := s.ReserveAdmission(ctx, c.Domain, owner, "request", v, 1)
	require.NoError(t, err)
	_, err = s.CreateAdmittedScaleSetInstance(ctx, r.ID, set.ID, params.CreateInstanceParams{Name: "runner", Status: commonParams.InstancePendingCreate, RunnerStatus: params.RunnerPending})
	require.NoError(t, err)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	var canceled bool
	var cancelErr, sendErr error
	go func() {
		defer wg.Done()
		<-start
		canceled, cancelErr = s.CancelUnsentBoundAdmission(ctx, c.Provider, "runner")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, _, sendErr = s.BeginAdmittedCreate(ctx, c.Provider, owner, "runner", params.AdmissionCreateShape{Generation: set.Generation})
	}()
	close(start)
	wg.Wait()
	require.NoError(t, cancelErr)
	require.Equal(t, canceled, sendErr != nil)
	var count int64
	require.NoError(t, db.Model(&Instance{}).Where("name = ?", "runner").Count(&count).Error)
	saved, err := s.AdmissionByInstance(ctx, c.Provider, "runner")
	require.NoError(t, err)
	if canceled {
		require.Zero(t, count)
		require.Equal(t, admissionReleased, saved.State)
	} else {
		require.Equal(t, int64(1), count)
		require.Equal(t, admissionSending, saved.State)
	}
}
