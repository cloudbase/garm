// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

package sql

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/sql/migrations"
	"github.com/cloudbase/garm/params"
)

type (
	admissionCredit = migrations.AdmissionCredit0013
	admissionJob    = migrations.AdmissionJob0013
)

const admissionJobProtocol = "job-capacity-experimental"

func jobResult(j admissionJob) params.AdmissionJob {
	return params.AdmissionJob{ID: j.ID, Domain: j.Domain, Owner: j.Owner, RequestID: j.RequestID, JobID: j.JobID, InstanceName: j.InstanceName, State: j.State, Terminal: j.Terminal, Absent: j.Absent}
}

func getCredit(tx *gorm.DB, domain, owner string) (admissionCredit, error) {
	var c admissionCredit
	err := tx.Where("domain = ? AND owner = ?", domain, owner).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, common.ErrAdmissionNotFound
	}
	return c, err
}

func getJob(tx *gorm.DB, domain, owner, requestID, jobID string) (admissionJob, error) {
	var j admissionJob
	err := tx.Where("domain = ? AND owner = ? AND request_id = ? AND job_id = ?", domain, owner, requestID, jobID).First(&j).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return j, common.ErrAdmissionNotFound
	}
	return j, err
}

func creditProtocol(tx *gorm.DB, domain string) error {
	var p migrations.AdmissionDomain0013
	if err := tx.First(&p, "domain = ?", domain).Error; err != nil {
		return err
	}
	if p.Protocol != admissionJobProtocol {
		return errors.New("domain is not enrolled in job admission")
	}
	return nil
}

func (s *sqlDatabase) EnsureAdmissionCredit(ctx context.Context, domain, owner string, v params.AdmissionVector, generation int64) (params.AdmissionReservation, error) {
	var out params.AdmissionReservation
	if domain == "" || owner == "" || !validVector(v) || v.VMs != 1 || v.Jobs != 1 {
		return out, errors.New("invalid admission credit")
	}
	err := s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		d, err := lockDomain(tx, domain)
		if err != nil {
			return err
		}
		if err := creditProtocol(tx, domain); err != nil {
			return err
		}
		c, err := getCredit(tx, domain, owner)
		if err == nil {
			var r admissionReservation
			if err := tx.First(&r, "id = ?", c.ReservationID).Error; err != nil {
				return err
			}
			if vector(r) != v || r.State == admissionReleased {
				return errors.New("credit configuration mismatch")
			}
			out = result(r)
			return nil
		}
		if !errors.Is(err, common.ErrAdmissionNotFound) {
			return err
		}
		if vectorFromDomain(d) != v {
			return errors.New("credit must match configured class")
		}
		if err := reserveAdmission(tx, domain, owner, "job-capacity-credit", v, generation, &out); err != nil {
			return err
		}
		return tx.Create(&admissionCredit{Domain: domain, Owner: owner, ReservationID: out.ID}).Error
	})
	return out, err
}

func vectorFromDomain(d admissionDomain) params.AdmissionVector {
	return params.AdmissionVector{CPU: d.ClassCPU, RAM: d.ClassRAM, Disk: d.ClassDisk, VMs: d.ClassVMs, Jobs: d.ClassJobs}
}

func (s *sqlDatabase) GetAdmissionCredit(ctx context.Context, domain, owner string) (params.AdmissionReservation, []params.AdmissionJob, error) {
	var out params.AdmissionReservation
	var jobs []params.AdmissionJob
	err := s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockDomain(tx, domain); err != nil {
			return err
		}
		c, err := getCredit(tx, domain, owner)
		if err != nil {
			return err
		}
		var r admissionReservation
		if err := tx.First(&r, "id = ?", c.ReservationID).Error; err != nil {
			return err
		}
		out = result(r)
		out.PollEpoch = c.PollEpoch
		out.PollPending = c.PollPending
		var rows []admissionJob
		if err := tx.Where("domain = ? AND owner = ? AND ((terminal = ? AND state IN ?) OR (terminal = ? AND instance_name <> ? AND absent = ?))", domain, owner, false, []string{"acquire_sending", "acquired"}, true, "", false).Find(&rows).Error; err != nil {
			return err
		}
		for _, j := range rows {
			jobs = append(jobs, jobResult(j))
		}
		return nil
	})
	return out, jobs, err
}

// checkAdmissionHeldBudget runs under the domain lock and includes all active
// reservations, including VM-only allocations and durable credits.
func checkAdmissionHeldBudget(tx *gorm.DB, d admissionDomain) error {
	var rows []admissionReservation
	if err := tx.Where("domain = ? AND state <> ?", d.Domain, admissionReleased).Find(&rows).Error; err != nil {
		return err
	}
	var used params.AdmissionVector
	for _, r := range rows {
		x := vector(r)
		if !fits(used.CPU, x.CPU, math.MaxInt64) || !fits(used.RAM, x.RAM, math.MaxInt64) || !fits(used.Disk, x.Disk, math.MaxInt64) || !fits(used.VMs, x.VMs, math.MaxInt64) || !fits(used.Jobs, x.Jobs, math.MaxInt64) {
			return errors.New("admission accounting overflow")
		}
		used.CPU += x.CPU
		used.RAM += x.RAM
		used.Disk += x.Disk
		used.VMs += x.VMs
		used.Jobs += x.Jobs
	}
	limit := capacity(d)
	if !fits(used.CPU, 0, limit.CPU) || !fits(used.RAM, 0, limit.RAM) || !fits(used.Disk, 0, limit.Disk) || !fits(used.VMs, 0, limit.VMs) || !fits(used.Jobs, 0, limit.Jobs) {
		return errors.New("held admission reservations exceed current budget")
	}
	return nil
}

func (s *sqlDatabase) AuthorizeAdmissionPoll(ctx context.Context, domain, owner string, generation int64) (int64, error) {
	var epoch int64
	err := s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		d, err := lockDomain(tx, domain)
		if err != nil {
			return err
		}
		if err := checkAdmissionHeldBudget(tx, d); err != nil {
			return err
		}
		if err := creditProtocol(tx, domain); err != nil {
			return err
		}
		c, err := getCredit(tx, domain, owner)
		if err != nil {
			return err
		}
		var creditReservation admissionReservation
		if err := tx.First(&creditReservation, "id = ?", c.ReservationID).Error; err != nil {
			return err
		}
		if creditReservation.State == admissionReleased || creditReservation.BackendIdentity != d.BackendIdentity || vector(creditReservation) != vectorFromDomain(d) {
			return errors.New("credit does not match domain")
		}
		if c.PollPending || c.PollEpoch == math.MaxInt64 {
			return errors.New("poll outcome uncertain")
		}
		if d.Drained || !d.Ready || d.Generation != generation || d.BackendIdentity == "" || !d.HealthyUntil.After(time.Now()) {
			return errors.New("admission unavailable")
		}
		var id uint
		if _, err := fmt.Sscanf(owner, "scaleset:%d", &id); err != nil || owner != fmt.Sprintf("scaleset:%d", id) {
			return errors.New("invalid credit owner")
		}
		set, err := s.getScaleSetByID(tx, id)
		if err != nil {
			return err
		}
		if !set.Enabled || set.MaxRunners < 1 || set.MinIdleRunners != 0 || set.ProviderName != d.Provider {
			return errors.New("scale set is not eligible")
		}
		var active int64
		if err := tx.Model(&admissionJob{}).Where("domain = ? AND owner = ? AND terminal = ? AND state IN ?", domain, owner, false, []string{"acquire_sending", "acquired"}).Count(&active).Error; err != nil {
			return err
		}
		if active != 0 {
			return fmt.Errorf("%w: credit already has an outstanding job", common.ErrAdmissionConflict)
		}
		epoch = c.PollEpoch + 1
		return tx.Model(&c).Updates(map[string]interface{}{"poll_epoch": epoch, "poll_pending": true}).Error
	})
	return epoch, err
}

func (s *sqlDatabase) FinishAdmissionPoll(ctx context.Context, domain, owner string, epoch int64) error {
	return s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockDomain(tx, domain); err != nil {
			return err
		}
		c, err := getCredit(tx, domain, owner)
		if err != nil {
			return err
		}
		if !c.PollPending || c.PollEpoch != epoch {
			return errors.New("poll epoch mismatch")
		}
		return tx.Model(&c).Update("poll_pending", false).Error
	})
}

func (s *sqlDatabase) RecordAdmissionAssignment(ctx context.Context, domain, owner, requestID, jobID string) (params.AdmissionJob, error) {
	var out params.AdmissionJob
	if requestID == "" || jobID == "" {
		return out, fmt.Errorf("%w: missing job identity", common.ErrAdmissionConflict)
	}
	err := s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockDomain(tx, domain); err != nil {
			return err
		}
		if _, err := getCredit(tx, domain, owner); err != nil {
			return err
		}
		j, err := getJob(tx, domain, owner, requestID, jobID)
		if err == nil {
			out = jobResult(j)
			return nil
		}
		if !errors.Is(err, common.ErrAdmissionNotFound) {
			return err
		}
		var prior admissionJob
		priorErr := tx.Where("domain = ? AND owner = ? AND request_id = ?", domain, owner, requestID).First(&prior).Error
		if priorErr == nil {
			return fmt.Errorf("%w: request identity reused", common.ErrAdmissionConflict)
		}
		if !errors.Is(priorErr, gorm.ErrRecordNotFound) {
			return priorErr
		}
		id, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		j = admissionJob{ID: id.String(), Domain: domain, Owner: owner, RequestID: requestID, JobID: jobID, State: "offered"}
		if err := tx.Create(&j).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return fmt.Errorf("%w: request identity reused", common.ErrAdmissionConflict)
			}
			return err
		}
		out = jobResult(j)
		return nil
	})
	return out, err
}

func checkAdmissionJobAvailable(tx *gorm.DB, domain, owner, exceptID string) error {
	var active int64
	if err := tx.Model(&admissionJob{}).Where("domain = ? AND owner = ? AND terminal = ? AND state IN ? AND id <> ?", domain, owner, false, []string{"acquire_sending", "acquired"}, exceptID).Count(&active).Error; err != nil {
		return err
	}
	if active != 0 {
		return fmt.Errorf("%w: credit already has an outstanding job", common.ErrAdmissionConflict)
	}
	return nil
}

func (s *sqlDatabase) BeginAdmissionAcquire(ctx context.Context, domain, owner, requestID, jobID string) (bool, error) {
	sent := false
	err := s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		d, err := lockDomain(tx, domain)
		if err != nil {
			return err
		}
		if err := checkAdmissionHeldBudget(tx, d); err != nil {
			return err
		}
		c, err := getCredit(tx, domain, owner)
		if err != nil {
			return err
		}
		var reservation admissionReservation
		if err := tx.First(&reservation, "id = ?", c.ReservationID).Error; err != nil {
			return err
		}
		if d.Drained || !d.Ready || d.BackendIdentity == "" || d.BackendIdentity != reservation.BackendIdentity || vector(reservation) != vectorFromDomain(d) || !d.HealthyUntil.After(time.Now()) {
			return errors.New("admission unavailable")
		}
		var id uint
		if _, err := fmt.Sscanf(owner, "scaleset:%d", &id); err != nil || owner != fmt.Sprintf("scaleset:%d", id) {
			return errors.New("invalid credit owner")
		}
		set, err := s.getScaleSetByID(tx, id)
		if err != nil {
			return err
		}
		if !set.Enabled || set.MaxRunners < 1 || set.MinIdleRunners != 0 || set.ProviderName != d.Provider {
			return errors.New("scale set is not eligible")
		}

		j, err := getJob(tx, domain, owner, requestID, jobID)
		if err != nil {
			return err
		}
		if j.Terminal || j.State != "offered" {
			return nil
		}
		if err := checkAdmissionJobAvailable(tx, domain, owner, j.ID); err != nil {
			return err
		}
		if err := tx.Model(&j).Update("state", "acquire_sending").Error; err != nil {
			return err
		}
		sent = true
		return nil
	})
	return sent, err
}

func (s *sqlDatabase) MarkAdmissionAcquired(ctx context.Context, domain, owner, requestID, jobID string) error {
	return s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockDomain(tx, domain); err != nil {
			return err
		}
		j, err := getJob(tx, domain, owner, requestID, jobID)
		if err != nil {
			return err
		}
		if j.Terminal {
			return nil
		}
		if j.State == "acquired" {
			return nil
		}
		if j.State != "offered" && j.State != "acquire_sending" {
			return errors.New("invalid job state")
		}
		if err := checkAdmissionJobAvailable(tx, domain, owner, j.ID); err != nil {
			return err
		}
		return tx.Model(&j).Update("state", "acquired").Error
	})
}

func parkCreditIfDone(tx *gorm.DB, c admissionCredit, j admissionJob) error {
	if !j.Terminal || !j.Absent || j.InstanceName == "" {
		return nil
	}
	var r admissionReservation
	if err := tx.First(&r, "id = ?", c.ReservationID).Error; err != nil {
		return err
	}
	if r.InstanceName == nil || *r.InstanceName != j.InstanceName {
		return errors.New("credit binding mismatch")
	}
	return tx.Model(&r).Updates(map[string]interface{}{"state": admissionReserved, "instance_name": nil}).Error
}

func (s *sqlDatabase) CompleteAdmissionJob(ctx context.Context, domain, owner, requestID, jobID string) error {
	return s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockDomain(tx, domain); err != nil {
			return err
		}
		c, err := getCredit(tx, domain, owner)
		if err != nil {
			return err
		}
		j, err := getJob(tx, domain, owner, requestID, jobID)
		if err != nil {
			return err
		}
		if j.Terminal {
			return nil
		}
		if err := tx.Model(&j).Updates(map[string]interface{}{"terminal": true, "state": "terminal"}).Error; err != nil {
			return err
		}
		j.Terminal = true
		return parkCreditIfDone(tx, c, j)
	})
}

// CreateAdmissionJobInstance binds the exclusive credit to one acquired job and
// inserts its instance under the same domain lock.
func (s *sqlDatabase) CreateAdmissionJobInstance(ctx context.Context, domain, owner, requestID, jobID string, scaleSetID uint, p params.CreateInstanceParams) (instance params.Instance, err error) {
	if p.Name == "" || owner != fmt.Sprintf("scaleset:%d", scaleSetID) {
		return instance, errors.New("invalid job instance identity")
	}
	err = s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		set, err := s.getScaleSetByID(tx, scaleSetID)
		if err != nil {
			return err
		}
		if err := providerLock(tx, set.ProviderName); err != nil {
			return err
		}
		d, err := lockDomain(tx, domain)
		if err != nil {
			return err
		}
		if err := checkAdmissionHeldBudget(tx, d); err != nil {
			return err
		}
		if err := creditProtocol(tx, domain); err != nil {
			return err
		}
		c, err := getCredit(tx, domain, owner)
		if err != nil {
			return err
		}
		j, err := getJob(tx, domain, owner, requestID, jobID)
		if err != nil {
			return err
		}
		var r admissionReservation
		if err := tx.First(&r, "id = ?", c.ReservationID).Error; err != nil {
			return err
		}
		if err := validateAdmissionJobBinding(d, set, r); err != nil {
			return err
		}
		var usedName int64
		if err := tx.Model(&admissionJob{}).Where("domain = ? AND instance_name = ?", domain, p.Name).Count(&usedName).Error; err != nil {
			return err
		}
		if usedName != 0 {
			return errors.New("job instance name has already been used")
		}
		if j.Terminal || j.State != "acquired" || j.InstanceName != "" || r.InstanceName != nil || r.State != admissionReserved {
			return errors.New("credit not available for this job")
		}
		if err := tx.Model(&r).Updates(map[string]interface{}{"instance_name": p.Name, "generation": d.Generation}).Error; err != nil {
			return err
		}
		if err := tx.Model(&j).Update("instance_name", p.Name).Error; err != nil {
			return err
		}
		instance, err = s.createScaleSetInstance(tx, scaleSetID, p)
		return err
	})
	if err == nil {
		s.sendNotify(common.InstanceEntityType, common.CreateOperation, instance)
	}
	return instance, err
}

func validateAdmissionJobBinding(d admissionDomain, set ScaleSet, r admissionReservation) error {
	if d.Drained || !d.Ready || d.BackendIdentity == "" || r.BackendIdentity != d.BackendIdentity || !d.HealthyUntil.After(time.Now()) {
		return errors.New("admission unavailable")
	}
	if !set.Enabled || set.MaxRunners < 1 || set.MinIdleRunners != 0 || set.ProviderName != d.Provider || set.Image != d.Image || set.Flavor != d.Flavor || string(set.OSArch) != d.OSArch || vector(r) != vectorFromDomain(d) {
		return errors.New("scale set does not match credit")
	}
	return nil
}

// jobInstanceAlreadyAbsent makes replay of a completed VM cleanup safe even
// after the durable credit has been rebound to a later attempt.
func (s *sqlDatabase) jobInstanceAlreadyAbsent(ctx context.Context, provider, name string) (bool, error) {
	d, err := admissionForProvider(s.conn.WithContext(ctx), provider)
	if err != nil {
		return false, err
	}
	var j admissionJob
	if err := s.conn.WithContext(ctx).Where("domain = ? AND instance_name = ?", d.Domain, name).First(&j).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, common.ErrAdmissionNotFound
		}
		return false, err
	}
	return j.Absent, nil
}
