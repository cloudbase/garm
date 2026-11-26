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

	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

func TestAdmissionJobCreditPostgresConcurrency(t *testing.T) {
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
	first := ScaleSet{Name: "set-one", ProviderName: "provider", Enabled: true, MaxRunners: 1}
	second := ScaleSet{Name: "set-two", ProviderName: "provider", Enabled: true, MaxRunners: 1}
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)
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
			if _, err := s.EnsureAdmissionCredit(ctx, c.Domain, owners[i], v, 1); err == nil {
				mu.Lock()
				won = append(won, i)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	require.Len(t, won, 1)
	owner := owners[won[0]]
	_, err = s.RecordAdmissionAssignment(ctx, c.Domain, owner, "a", "job-a")
	require.NoError(t, err)
	_, err = s.RecordAdmissionAssignment(ctx, c.Domain, owner, "b", "job-b")
	require.NoError(t, err)
	var sent int
	for _, key := range []string{"a", "b"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			ok, err := s.BeginAdmissionAcquire(ctx, c.Domain, owner, key, "job-"+key)
			if err == nil && ok {
				mu.Lock()
				sent++
				mu.Unlock()
			}
		}(key)
	}
	wg.Wait()
	require.Equal(t, 1, sent)
	credit, _, err := s.GetAdmissionCredit(ctx, c.Domain, owner)
	require.NoError(t, err)
	require.Error(t, s.ReleaseUnsentAdmission(ctx, credit.ID))
}

func TestAdmissionJobMigrationPostgresPreservesLegacy(t *testing.T) {
	if os.Getenv("GARM_TEST_POSTGRES_DSN") == "" {
		t.Skip("GARM_TEST_POSTGRES_DSN is not set")
	}
	raw, _ := garmTesting.OpenTestPostgresDB(t)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: raw}), &gorm.Config{})
	require.NoError(t, err)
	checkAdmissionJobMigrationPreservesLegacyEnrollment(t, db)
}
