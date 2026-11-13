//go:build testing

// Copyright 2026 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

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
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/sql/migrations"
	"github.com/cloudbase/garm/params"
)

func TestAdmissionSQLiteDurabilityAndConcurrency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admission.db") + "?_txlock=immediate&_busy_timeout=5000"
	open := func() *sqlDatabase {
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
		require.NoError(t, err)
		raw, err := db.DB()
		require.NoError(t, err)
		raw.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = raw.Close() })
		return &sqlDatabase{conn: db}
	}
	a, b := open(), open()
	require.NoError(t, a.conn.AutoMigrate(&admissionDomain{}, &admissionReservation{}, &migrations.AdmissionDomain0013{}, &admissionCredit{}, &admissionJob{}, &Pool{}, &ScaleSet{}, &Instance{}))
	ctx := context.Background()
	c := params.AdmissionConfig{Domain: "host", Provider: "synthetic", Class: params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1}, Capacity: params.AdmissionVector{CPU: 10, RAM: 10, Disk: 10, VMs: 10, Jobs: 10}, Generation: 1, RequestedActivationGeneration: 1, HealthyUntil: time.Now().Add(time.Hour), Ready: true}
	require.NoError(t, a.ConfigureAdmission(ctx, c))
	c.BackendIdentity = strings.Repeat("a", 64)
	c.HealthObservedAt = time.Now().Add(-time.Second)
	require.NoError(t, a.UpdateAdmissionHealth(ctx, c))
	require.NoError(t, a.ResumeAdmission(ctx, "host", 1))
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			db := a
			if i%2 == 1 {
				db = b
			}
			v := params.AdmissionVector{CPU: 2, RAM: 1, Disk: 1, VMs: 1, Jobs: 1}
			if i%2 == 1 {
				v.CPU = 1
				v.RAM = 2
			}
			_, err := db.ReserveAdmission(ctx, "host", fmt.Sprintf("owner-%d", i), "request", v, 1)
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
	require.NoError(t, a.conn.Find(&rows).Error)
	var cpu, ram int64
	for _, r := range rows {
		cpu += r.CPU
		ram += r.RAM
	}
	require.LessOrEqual(t, cpu, int64(10))
	require.LessOrEqual(t, ram, int64(10))
	first := rows[0]
	again, err := b.ReserveAdmission(ctx, "host", first.Owner, first.RequestKey, vector(first), 1)
	require.NoError(t, err)
	require.Equal(t, first.ID, again.ID)
	require.NoError(t, b.MarkAdmissionSending(ctx, first.ID))
	require.Error(t, a.ReleaseUnsentAdmission(ctx, first.ID))
	require.NoError(t, a.BindAdmission(ctx, first.ID, "runner"))
	c.Capacity = params.AdmissionVector{}
	c.Generation = 2
	c.RequestedActivationGeneration = 2
	require.NoError(t, b.ConfigureAdmission(ctx, c))
	_, err = a.ReserveAdmission(ctx, "host", "new", "request", params.AdmissionVector{CPU: 1}, 2)
	require.Error(t, err)
	require.NoError(t, a.SetAdmissionDrain(ctx, "host", true))
	c.Capacity = params.AdmissionVector{CPU: 100, RAM: 100, Disk: 100, VMs: 100, Jobs: 100}
	c.Generation = 3
	c.RequestedActivationGeneration = 3
	require.NoError(t, b.ConfigureAdmission(ctx, c))
	_, err = a.ReserveAdmission(ctx, "host", "new", "request", params.AdmissionVector{CPU: 1}, 2)
	require.Error(t, err)
	require.Error(t, b.ConfirmAdmissionAbsent(ctx, first.ID))
	require.NoError(t, b.MarkAdmissionPresent(ctx, "synthetic", "runner"))
	require.NoError(t, b.BeginAdmissionDelete(ctx, "synthetic", "runner"))
	require.NoError(t, b.ConfirmAdmissionAbsentByInstance(ctx, "synthetic", "runner"))
	var saved admissionReservation
	require.NoError(t, a.conn.First(&saved, "id = ?", first.ID).Error)
	require.Equal(t, "released", saved.State)
}

func TestAdmissionGuardsInstanceCreation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "guards.db")+"?_txlock=immediate&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, initSchema(db))
	raw, err := db.DB()
	require.NoError(t, err)
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	s := &sqlDatabase{conn: db, ctx: context.Background()}
	ctx := context.Background()
	set := ScaleSet{Name: "synthetic", ProviderName: "provider", Enabled: true, MaxRunners: 1}
	require.NoError(t, db.Create(&set).Error)
	class := params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1}
	cfg := params.AdmissionConfig{Domain: "domain", Provider: "provider", Class: class, Capacity: class, Generation: 1, RequestedActivationGeneration: 1, Ready: true, HealthyUntil: time.Now().Add(time.Hour)}
	require.NoError(t, s.ConfigureAdmission(ctx, cfg))
	cfg.BackendIdentity = strings.Repeat("a", 64)
	cfg.HealthObservedAt = time.Now().Add(-time.Second)
	require.NoError(t, s.UpdateAdmissionHealth(ctx, cfg))
	require.NoError(t, s.ResumeAdmission(ctx, "domain", 1))
	pool := Pool{ProviderName: "provider", MaxRunners: 1}
	require.NoError(t, db.Create(&pool).Error)
	_, err = s.CreateInstance(ctx, pool.ID.String(), params.CreateInstanceParams{Name: "pool-bypass"})
	require.Error(t, err)
	_, err = s.CreateScaleSetInstance(ctx, set.ID, params.CreateInstanceParams{Name: "bypass"})
	require.Error(t, err)
	r, err := s.ReserveAdmission(ctx, "domain", fmt.Sprintf("scaleset:%d", set.ID), "request", class, 1)
	require.NoError(t, err)
	_, err = s.CreateAdmittedScaleSetInstance(ctx, r.ID, set.ID, params.CreateInstanceParams{Name: "runner"})
	require.NoError(t, err)
	_, enrolled, err := s.BeginAdmittedCreate(ctx, "provider", fmt.Sprintf("scaleset:%d", set.ID), "runner", params.AdmissionCreateShape{Generation: set.Generation})
	require.True(t, enrolled)
	require.NoError(t, err)
	_, _, err = s.BeginAdmittedCreate(ctx, "provider", fmt.Sprintf("scaleset:%d", set.ID), "runner", params.AdmissionCreateShape{Generation: set.Generation})
	require.Error(t, err)
	_, err = s.ReserveAdmission(ctx, "domain", "other", "request", class, 1)
	require.Error(t, err)
}

func TestAdmissionResumeAndHealthOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resume.db") + "?_txlock=immediate&_busy_timeout=5000"
	open := func() *sqlDatabase {
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
		require.NoError(t, err)
		raw, err := db.DB()
		require.NoError(t, err)
		raw.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = raw.Close() })
		return &sqlDatabase{conn: db}
	}
	a := open()
	require.NoError(t, initSchema(a.conn))
	ctx := context.Background()
	c := params.AdmissionConfig{Domain: "domain", Provider: "provider", Generation: 1, RequestedActivationGeneration: 1, Class: params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1}, Capacity: params.AdmissionVector{CPU: 10, RAM: 10, Disk: 10, VMs: 10}, Ready: false, HealthyUntil: time.Now().Add(time.Hour)}
	require.NoError(t, a.ConfigureAdmission(ctx, c))
	require.Error(t, a.ResumeAdmission(ctx, "domain", 1))
	c.Ready = true
	c.BackendIdentity = strings.Repeat("a", 64)
	c.HealthObservedAt = time.Now().Add(-time.Second)
	require.NoError(t, a.UpdateAdmissionHealth(ctx, c))
	require.NoError(t, a.ResumeAdmission(ctx, "domain", 1))
	require.NoError(t, a.SetAdmissionDrain(ctx, "domain", true))
	b := open()
	require.Error(t, b.ResumeAdmission(ctx, "domain", 1))
	state, enrolled, err := b.AdmissionForProvider(ctx, "provider")
	require.NoError(t, err)
	require.True(t, enrolled)
	require.True(t, state.Drained)
	require.Equal(t, int64(1), state.LastResumeGeneration)
	c.Generation = 2
	c.RequestedActivationGeneration = 2
	c.Ready = false
	c.HealthyUntil = time.Time{}
	require.NoError(t, b.ConfigureAdmission(ctx, c))
	require.Error(t, b.ResumeAdmission(ctx, "domain", 2))
	c.Ready = true
	c.BackendIdentity = strings.Repeat("a", 64)
	c.HealthObservedAt = time.Now().Add(-500 * time.Millisecond)
	c.HealthyUntil = time.Now().Add(-time.Second)
	require.NoError(t, b.UpdateAdmissionHealth(ctx, c))
	require.Error(t, b.ResumeAdmission(ctx, "domain", 2))
	c.HealthObservedAt = time.Now().Add(-400 * time.Millisecond)
	c.HealthyUntil = time.Now().Add(time.Hour)
	require.NoError(t, b.UpdateAdmissionHealth(ctx, c))
	require.NoError(t, b.ResumeAdmission(ctx, "domain", 2))
	oldProbe := c
	oldProbe.HealthObservedAt = time.Now().Add(-300 * time.Millisecond)
	reduced := c
	reduced.Capacity.CPU = 2
	reduced.Generation = 3
	reduced.RequestedActivationGeneration = 3
	require.NoError(t, b.ConfigureAdmission(ctx, reduced))
	failure := c
	failure.Generation = 3
	failure.Ready = false
	failure.HealthyUntil = time.Time{}
	failure.HealthObservedAt = time.Now().Add(-200 * time.Millisecond)
	require.NoError(t, b.UpdateAdmissionHealth(ctx, failure))
	require.Error(t, a.UpdateAdmissionHealth(ctx, oldProbe))
	state, _, err = a.AdmissionForProvider(ctx, "provider")
	require.NoError(t, err)
	require.Equal(t, int64(2), state.Capacity.CPU)
	require.False(t, state.Ready)
}

func TestAdmissionBackendIdentityPinned(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pin.db")+"?_txlock=immediate&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, initSchema(db))
	raw, err := db.DB()
	require.NoError(t, err)
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	s := &sqlDatabase{conn: db}
	ctx := context.Background()
	vector := params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1}
	c := params.AdmissionConfig{Domain: "domain", Provider: "provider", Generation: 1, RequestedActivationGeneration: 1, Class: vector, Capacity: vector}
	require.NoError(t, s.ConfigureAdmission(ctx, c))
	c.Ready = true
	c.BackendIdentity = strings.Repeat("a", 64)
	c.HealthObservedAt = time.Now().Add(-time.Second)
	c.HealthyUntil = time.Now().Add(time.Hour)
	require.NoError(t, s.UpdateAdmissionHealth(ctx, c))
	require.NoError(t, s.ResumeAdmission(ctx, c.Domain, 1))
	r, err := s.ReserveAdmission(ctx, c.Domain, "owner", "key", vector, 1)
	require.NoError(t, err)
	require.Equal(t, c.BackendIdentity, r.BackendIdentity)
	switched := c
	switched.BackendIdentity = strings.Repeat("b", 64)
	switched.HealthObservedAt = time.Now()
	require.Error(t, s.UpdateAdmissionHealth(ctx, switched))
	switched.Generation = 2
	switched.RequestedActivationGeneration = 2
	switched.Project = "other"
	require.Error(t, s.ConfigureAdmission(ctx, switched))
	// A budget-only generation change cannot replace the pinned server either.
	switched.Project = c.Project
	require.NoError(t, s.ConfigureAdmission(ctx, switched))
	state, _, err := s.AdmissionForProvider(ctx, c.Provider)
	require.NoError(t, err)
	require.Equal(t, c.BackendIdentity, state.BackendIdentity)
	switched.HealthObservedAt = time.Now()
	require.Error(t, s.UpdateAdmissionHealth(ctx, switched))
	found, err := s.AdmissionByInstance(ctx, c.Provider, "missing")
	_ = found
	require.ErrorIs(t, err, common.ErrAdmissionNotFound)
}

func TestAdmissionAtomicBoundReleaseAndCancellation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "atomic.db")+"?_txlock=immediate&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, initSchema(db))
	raw, err := db.DB()
	require.NoError(t, err)
	raw.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = raw.Close() })
	s := &sqlDatabase{conn: db, ctx: context.Background()}
	ctx := context.Background()
	set := ScaleSet{Name: "synthetic", ProviderName: "provider", Enabled: true, MaxRunners: 1}
	require.NoError(t, db.Create(&set).Error)
	v := params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1}
	c := params.AdmissionConfig{Domain: "domain", Provider: "provider", Generation: 1, RequestedActivationGeneration: 1, Class: v, Capacity: params.AdmissionVector{CPU: 4, RAM: 4, Disk: 4, VMs: 4}}
	require.NoError(t, s.ConfigureAdmission(ctx, c))
	c.BackendIdentity = strings.Repeat("a", 64)
	c.Ready = true
	c.HealthyUntil = time.Now().Add(time.Hour)
	c.HealthObservedAt = time.Now().Add(-time.Second)
	require.NoError(t, s.UpdateAdmissionHealth(ctx, c))
	require.NoError(t, s.ResumeAdmission(ctx, c.Domain, 1))
	owner := fmt.Sprintf("scaleset:%d", set.ID)
	empty, err := s.ReserveAdmission(ctx, c.Domain, owner, "empty", v, 1)
	require.NoError(t, err)
	require.NoError(t, s.ReleaseUnsentAdmission(ctx, empty.ID))
	require.NoError(t, s.ReleaseUnsentAdmission(ctx, empty.ID))
	reserve := func(key string) params.AdmissionReservation {
		t.Helper()
		r, e := s.ReserveAdmission(ctx, c.Domain, owner, key, v, 1)
		require.NoError(t, e)
		_, e = s.CreateAdmittedScaleSetInstance(ctx, r.ID, set.ID, params.CreateInstanceParams{Name: key, Status: commonParams.InstancePendingCreate, RunnerStatus: params.RunnerPending})
		require.NoError(t, e)
		require.Error(t, s.ReleaseUnsentAdmission(ctx, r.ID))
		return r
	}
	bound := reserve("cancel-race")
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	var canceled bool
	var cancelErr, sendErr error
	go func() {
		defer wg.Done()
		<-start
		canceled, cancelErr = s.CancelUnsentBoundAdmission(ctx, c.Provider, "cancel-race")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, _, sendErr = s.BeginAdmittedCreate(ctx, c.Provider, owner, "cancel-race", params.AdmissionCreateShape{Generation: set.Generation})
	}()
	close(start)
	wg.Wait()
	require.NoError(t, cancelErr)
	require.Equal(t, canceled, sendErr != nil)
	var count int64
	require.NoError(t, db.Model(&Instance{}).Where("name = ?", "cancel-race").Count(&count).Error)
	state, err := s.AdmissionByInstance(ctx, c.Provider, "cancel-race")
	require.NoError(t, err)
	if canceled {
		require.Equal(t, int64(0), count)
		require.Equal(t, admissionReleased, state.State)
		again, err := s.CancelUnsentBoundAdmission(ctx, c.Provider, "cancel-race")
		require.NoError(t, err)
		require.True(t, again)
	} else {
		require.Equal(t, int64(1), count)
		require.Equal(t, "sending", state.State)
		require.Error(t, s.ReleaseUnsentAdmission(ctx, bound.ID))
	}
	// Inconsistent external status is not evidence that a runner is safe to retire.
	_ = reserve("running")
	require.NoError(t, db.Model(&Instance{}).Where("name = ?", "running").Updates(map[string]interface{}{"status": commonParams.InstanceRunning, "runner_status": params.RunnerActive}).Error)
	held, err := s.CancelUnsentBoundAdmission(ctx, c.Provider, "running")
	require.NoError(t, err)
	require.False(t, held)
	complete := reserve("completed")
	_, _, err = s.BeginAdmittedCreate(ctx, c.Provider, owner, "completed", params.AdmissionCreateShape{Generation: set.Generation})
	require.NoError(t, err)
	no, err := s.CancelUnsentBoundAdmission(ctx, c.Provider, "completed")
	require.NoError(t, err)
	require.False(t, no)
	require.NoError(t, s.MarkAdmissionPresent(ctx, c.Provider, "completed"))
	require.NoError(t, s.BeginAdmissionDelete(ctx, c.Provider, "completed"))
	require.NoError(t, s.ConfirmAdmissionAbsentByInstance(ctx, c.Provider, "completed"))
	require.NoError(t, s.ConfirmAdmissionAbsentByInstance(ctx, c.Provider, "completed"))
	require.NoError(t, db.Model(&Instance{}).Where("name = ?", "completed").Count(&count).Error)
	require.Zero(t, count)
	saved, err := s.AdmissionByInstance(ctx, c.Provider, "completed")
	require.NoError(t, err)
	require.Equal(t, admissionReleased, saved.State)
	require.NoError(t, s.ConfigureAdmission(ctx, c))
	_ = complete
}
