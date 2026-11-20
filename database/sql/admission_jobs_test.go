//go:build testing

// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

package sql

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/database/sql/migrations"
	"github.com/cloudbase/garm/params"
)

func TestAdmissionCreditLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.db") + "?_txlock=immediate&_busy_timeout=5000"
	open := func() *sqlDatabase {
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
		require.NoError(t, err)
		raw, err := db.DB()
		require.NoError(t, err)
		raw.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = raw.Close() })
		return &sqlDatabase{conn: db, ctx: context.Background()}
	}
	s, other := open(), open()
	require.NoError(t, initSchema(s.conn))
	ctx := context.Background()
	first := ScaleSet{Name: "set-one", ProviderName: "provider", Enabled: true, MaxRunners: 1}
	second := ScaleSet{Name: "set-two", ProviderName: "provider", Enabled: true, MaxRunners: 1}
	require.NoError(t, s.conn.Create(&first).Error)
	require.NoError(t, s.conn.Create(&second).Error)
	v := params.AdmissionVector{CPU: 2, RAM: 3, Disk: 4, VMs: 1, Jobs: 1}
	c := params.AdmissionConfig{Domain: "domain", Provider: "provider", Protocol: admissionJobProtocol, Generation: 1, RequestedActivationGeneration: 1, Class: v, Capacity: v}
	require.NoError(t, s.ConfigureAdmission(ctx, c))
	c.BackendIdentity = strings.Repeat("a", 64)
	c.Ready = true
	c.HealthObservedAt = time.Now().Add(-time.Second)
	c.HealthyUntil = time.Now().Add(time.Hour)
	require.NoError(t, s.UpdateAdmissionHealth(ctx, c))
	require.NoError(t, s.ResumeAdmission(ctx, c.Domain, 1))
	owners := []string{fmt.Sprintf("scaleset:%d", first.ID), fmt.Sprintf("scaleset:%d", second.ID)}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var won []int
	for i := range owners {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			db := s
			if i == 1 {
				db = other
			}
			if _, err := db.EnsureAdmissionCredit(ctx, c.Domain, owners[i], v, 1); err == nil {
				mu.Lock()
				won = append(won, i)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	require.Len(t, won, 1)
	owner := owners[won[0]]
	setID := first.ID
	if won[0] == 1 {
		setID = second.ID
	}
	for _, m := range migrations.All() {
		if m.ID == "0013_admission_jobs" {
			require.Error(t, m.Rollback(s.conn))
		}
	}
	credit, _, err := s.GetAdmissionCredit(ctx, c.Domain, owner)
	require.NoError(t, err)
	require.Error(t, s.ReleaseUnsentAdmission(ctx, credit.ID))
	same, err := other.EnsureAdmissionCredit(ctx, c.Domain, owner, v, 1)
	require.NoError(t, err)
	require.Equal(t, credit.ID, same.ID)
	require.NoError(t, s.conn.Model(&ScaleSet{}).Where("id = ?", setID).Update("max_runners", 0).Error)
	_, err = s.AuthorizeAdmissionPoll(ctx, c.Domain, owner, 1)
	require.Error(t, err)
	retained, _, err := s.GetAdmissionCredit(ctx, c.Domain, owner)
	require.NoError(t, err)
	require.Equal(t, credit.ID, retained.ID)
	require.NoError(t, s.conn.Model(&ScaleSet{}).Where("id = ?", setID).Update("max_runners", 1).Error)
	epoch, err := s.AuthorizeAdmissionPoll(ctx, c.Domain, owner, 1)
	require.NoError(t, err)
	_, err = other.AuthorizeAdmissionPoll(ctx, c.Domain, owner, 1)
	require.Error(t, err)
	stored, _, err := other.GetAdmissionCredit(ctx, c.Domain, owner)
	require.NoError(t, err)
	require.True(t, stored.PollPending)
	require.Equal(t, epoch, stored.PollEpoch)
	_, err = s.RecordAdmissionAssignment(ctx, c.Domain, owner, "99", "stale-offer")
	require.NoError(t, err)
	job, err := s.RecordAdmissionAssignment(ctx, c.Domain, owner, "100", "job")
	require.NoError(t, err)
	dup, err := other.RecordAdmissionAssignment(ctx, c.Domain, owner, "100", "job")
	require.NoError(t, err)
	require.Equal(t, job.ID, dup.ID)
	_, err = other.RecordAdmissionAssignment(ctx, c.Domain, owner, "100", "other")
	require.Error(t, err)
	require.NoError(t, s.conn.Model(&ScaleSet{}).Where("id = ?", setID).Update("max_runners", 0).Error)
	blocked, err := s.BeginAdmissionAcquire(ctx, c.Domain, owner, "100", "job")
	require.Error(t, err)
	require.False(t, blocked)
	require.NoError(t, s.conn.Model(&ScaleSet{}).Where("id = ?", setID).Update("max_runners", 1).Error)
	sent, err := s.BeginAdmissionAcquire(ctx, c.Domain, owner, "100", "job")
	require.NoError(t, err)
	require.True(t, sent)
	_, err = other.BeginAdmissionAcquire(ctx, c.Domain, owner, "99", "stale-offer")
	require.Error(t, err)
	sent, err = other.BeginAdmissionAcquire(ctx, c.Domain, owner, "100", "job")
	require.NoError(t, err)
	require.False(t, sent)
	require.NoError(t, s.MarkAdmissionAcquired(ctx, c.Domain, owner, "100", "job"))
	require.NoError(t, s.FinishAdmissionPoll(ctx, c.Domain, owner, epoch))
	p := params.CreateInstanceParams{Name: "runner-one", Status: commonParams.InstancePendingCreate, RunnerStatus: params.RunnerPending}
	_, err = s.CreateAdmissionJobInstance(ctx, c.Domain, owner, "100", "job", setID, p)
	require.NoError(t, err)
	_, err = s.CreateAdmissionJobInstance(ctx, c.Domain, owner, "100", "job", setID, params.CreateInstanceParams{Name: "duplicate"})
	require.Error(t, err)
	require.NoError(t, s.conn.Model(&ScaleSet{}).Where("id = ?", setID).Updates(map[string]interface{}{"max_runners": 0, "generation": 2}).Error)
	_, _, err = s.BeginAdmittedCreate(ctx, c.Provider, owner, p.Name, params.AdmissionCreateShape{Generation: 2})
	require.Error(t, err)
	stillReserved, _, err := s.GetAdmissionCredit(ctx, c.Domain, owner)
	require.NoError(t, err)
	require.Equal(t, admissionReserved, stillReserved.State)
	require.NoError(t, s.conn.Model(&ScaleSet{}).Where("id = ?", setID).Updates(map[string]interface{}{"max_runners": 1, "generation": 3}).Error)
	_, _, err = s.BeginAdmittedCreate(ctx, c.Provider, owner, p.Name, params.AdmissionCreateShape{Generation: 3})
	require.NoError(t, err)
	require.NoError(t, s.MarkAdmissionPresent(ctx, c.Provider, p.Name))
	require.NoError(t, s.CompleteAdmissionJob(ctx, c.Domain, owner, "100", "job"))
	require.NoError(t, s.BeginAdmissionDelete(ctx, c.Provider, p.Name))
	next, err := s.RecordAdmissionAssignment(ctx, c.Domain, owner, "101", "job")
	require.NoError(t, err)
	require.NotEqual(t, job.ID, next.ID)
	require.NoError(t, s.MarkAdmissionAcquired(ctx, c.Domain, owner, "101", "job"))
	_, err = s.CreateAdmissionJobInstance(ctx, c.Domain, owner, "101", "job", setID, params.CreateInstanceParams{Name: "runner-two"})
	require.Error(t, err)
	require.NoError(t, s.ConfirmAdmissionAbsentByInstance(ctx, c.Provider, p.Name))
	require.NoError(t, s.CompleteAdmissionJob(ctx, c.Domain, owner, "100", "job"))
	require.NoError(t, s.ConfirmAdmissionAbsentByInstance(ctx, c.Provider, p.Name))
	_, err = s.CreateAdmissionJobInstance(ctx, c.Domain, owner, "101", "job", setID, params.CreateInstanceParams{Name: "runner-two", Status: commonParams.InstancePendingCreate, RunnerStatus: params.RunnerPending})
	require.NoError(t, err)
	stored, jobs, err := other.GetAdmissionCredit(ctx, c.Domain, owner)
	require.NoError(t, err)
	require.Equal(t, credit.ID, stored.ID)
	require.Equal(t, "runner-two", stored.InstanceName)
	require.NoError(t, s.ConfirmAdmissionAbsentByInstance(ctx, c.Provider, "runner-one"))
	stale, err := s.CancelUnsentBoundAdmission(ctx, c.Provider, "runner-one")
	require.NoError(t, err)
	require.True(t, stale)
	after, _, err := s.GetAdmissionCredit(ctx, c.Domain, owner)
	require.NoError(t, err)
	require.Equal(t, "runner-two", after.InstanceName)
	require.Error(t, s.ConfirmAdmissionAbsent(ctx, credit.ID))
	require.Len(t, jobs, 1)
	require.Error(t, other.ReleaseUnsentAdmission(ctx, credit.ID))
	// Unknown acquisition is durable across connections, and drain prevents new sends.
	require.NoError(t, s.CompleteAdmissionJob(ctx, c.Domain, owner, "101", "job"))
	_, err = s.RecordAdmissionAssignment(ctx, c.Domain, owner, "102", "new-offer")
	require.NoError(t, err)
	require.NoError(t, s.SetAdmissionDrain(ctx, c.Domain, true))
	sent, err = other.BeginAdmissionAcquire(ctx, c.Domain, owner, "102", "new-offer")
	require.Error(t, err)
	require.False(t, sent)
	_, _, err = s.GetAdmissionCredit(ctx, c.Domain, owner)
	require.NoError(t, err)
	changed := c
	changed.Protocol = "vm-only-experimental"
	changed.Generation = 2
	changed.RequestedActivationGeneration = 2
	require.Error(t, s.ConfigureAdmission(ctx, changed))
}

func TestAdmissionJobMigrationPreservesLegacyEnrollment(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")+"?_txlock=immediate&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	raw, err := db.DB()
	require.NoError(t, err)
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	checkAdmissionJobMigrationPreservesLegacyEnrollment(t, db)
}

func checkAdmissionJobMigrationPreservesLegacyEnrollment(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&migrations.AdmissionDomain0012{}, &migrations.AdmissionReservation0012{}, &Pool{}, &ScaleSet{}, &Instance{}))
	d := migrations.AdmissionDomain0012{Domain: "legacy", Provider: "provider", Generation: 1, ClassCPU: 1, ClassRAM: 1, ClassDisk: 1, ClassVMs: 1, CPU: 1, RAM: 1, Disk: 1, VMs: 1}
	require.NoError(t, db.Create(&d).Error)
	r := migrations.AdmissionReservation0012{ID: "reservation", Domain: "legacy", Owner: "owner", RequestKey: "request", State: admissionReserved, CPU: 1, RAM: 1, Disk: 1, VMs: 1}
	require.NoError(t, db.Create(&r).Error)
	found := false
	for _, m := range migrations.All() {
		if m.ID == "0013_admission_jobs" {
			require.NoError(t, m.Migrate(db))
			found = true
		}
	}
	require.True(t, found)
	s := &sqlDatabase{conn: db}
	c := params.AdmissionConfig{Domain: "legacy", Provider: "provider", Protocol: "vm-only-experimental", Generation: 1, Class: params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1}, Capacity: params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1}}
	require.NoError(t, s.ConfigureAdmission(context.Background(), c))
	var saved admissionReservation
	require.NoError(t, db.First(&saved, "id = ?", r.ID).Error)
	require.Equal(t, admissionReserved, saved.State)
}

func TestAdmissionBudgetDecreaseBlocksNewJobWork(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "budget.db")+"?_txlock=immediate&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, initSchema(db))
	raw, err := db.DB()
	require.NoError(t, err)
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	s := &sqlDatabase{conn: db, ctx: context.Background()}
	ctx := context.Background()
	one := ScaleSet{Name: "one", ProviderName: "provider", Enabled: true, MaxRunners: 1}
	two := ScaleSet{Name: "two", ProviderName: "provider", Enabled: true, MaxRunners: 1}
	require.NoError(t, db.Create(&one).Error)
	require.NoError(t, db.Create(&two).Error)
	v := params.AdmissionVector{CPU: 2, RAM: 3, Disk: 4, VMs: 1, Jobs: 1}
	double := params.AdmissionVector{CPU: 4, RAM: 6, Disk: 8, VMs: 2, Jobs: 2}
	c := params.AdmissionConfig{Domain: "domain", Provider: "provider", Protocol: admissionJobProtocol, Generation: 1, RequestedActivationGeneration: 1, Class: v, Capacity: double}
	require.NoError(t, s.ConfigureAdmission(ctx, c))
	c.BackendIdentity = strings.Repeat("a", 64)
	c.Ready = true
	c.HealthObservedAt = time.Now().Add(-time.Second)
	c.HealthyUntil = time.Now().Add(time.Hour)
	require.NoError(t, s.UpdateAdmissionHealth(ctx, c))
	require.NoError(t, s.ResumeAdmission(ctx, c.Domain, 1))
	a, b := fmt.Sprintf("scaleset:%d", one.ID), fmt.Sprintf("scaleset:%d", two.ID)
	_, err = s.EnsureAdmissionCredit(ctx, c.Domain, a, v, 1)
	require.NoError(t, err)
	_, err = s.EnsureAdmissionCredit(ctx, c.Domain, b, v, 1)
	require.NoError(t, err)
	_, err = s.RecordAdmissionAssignment(ctx, c.Domain, a, "1", "job-a")
	require.NoError(t, err)
	require.NoError(t, s.MarkAdmissionAcquired(ctx, c.Domain, a, "1", "job-a"))
	_, err = s.RecordAdmissionAssignment(ctx, c.Domain, b, "2", "job-b")
	require.NoError(t, err)
	setBudget := func(generation int64, budget params.AdmissionVector) {
		t.Helper()
		c.Generation = generation
		c.RequestedActivationGeneration = generation
		c.Capacity = budget
		require.NoError(t, s.ConfigureAdmission(ctx, c))
		c.Ready = true
		c.HealthObservedAt = time.Now()
		c.HealthyUntil = time.Now().Add(time.Hour)
		require.NoError(t, s.UpdateAdmissionHealth(ctx, c))
	}
	setBudget(2, v)
	_, err = s.AuthorizeAdmissionPoll(ctx, c.Domain, b, 2)
	require.Error(t, err)
	sent, err := s.BeginAdmissionAcquire(ctx, c.Domain, b, "2", "job-b")
	require.Error(t, err)
	require.False(t, sent)
	_, err = s.CreateAdmissionJobInstance(ctx, c.Domain, a, "1", "job-a", one.ID, params.CreateInstanceParams{Name: "runner", Status: commonParams.InstancePendingCreate, RunnerStatus: params.RunnerPending})
	require.Error(t, err)
	for _, owner := range []string{a, b} {
		credit, _, err := s.GetAdmissionCredit(ctx, c.Domain, owner)
		require.NoError(t, err)
		require.NotEqual(t, admissionReleased, credit.State)
	}
	setBudget(3, double)
	_, err = s.CreateAdmissionJobInstance(ctx, c.Domain, a, "1", "job-a", one.ID, params.CreateInstanceParams{Name: "runner", Status: commonParams.InstancePendingCreate, RunnerStatus: params.RunnerPending})
	require.NoError(t, err)
	setBudget(4, v)
	_, _, err = s.BeginAdmittedCreate(ctx, c.Provider, a, "runner", params.AdmissionCreateShape{Generation: one.Generation})
	require.Error(t, err)
	setBudget(5, double)
	_, _, err = s.BeginAdmittedCreate(ctx, c.Provider, a, "runner", params.AdmissionCreateShape{Generation: one.Generation})
	require.NoError(t, err)
	sent, err = s.BeginAdmissionAcquire(ctx, c.Domain, b, "2", "job-b")
	require.NoError(t, err)
	require.True(t, sent)
}
