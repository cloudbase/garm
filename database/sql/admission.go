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
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/sql/migrations"
	"github.com/cloudbase/garm/params"
)

const (
	admissionReserved = "reserved"
	admissionReleased = "released"
	admissionSending  = "sending"
)

type (
	admissionDomain      = migrations.AdmissionDomain0012
	admissionReservation = migrations.AdmissionReservation0012
)

func validVector(v params.AdmissionVector) bool {
	return v.CPU >= 0 && v.RAM >= 0 && v.Disk >= 0 && v.VMs >= 0 && v.Jobs >= 0
}

func vector(r admissionReservation) params.AdmissionVector {
	return params.AdmissionVector{CPU: r.CPU, RAM: r.RAM, Disk: r.Disk, VMs: r.VMs, Jobs: r.Jobs}
}

func capacity(d admissionDomain) params.AdmissionVector {
	return params.AdmissionVector{CPU: d.CPU, RAM: d.RAM, Disk: d.Disk, VMs: d.VMs, Jobs: d.Jobs}
}

func result(r admissionReservation) params.AdmissionReservation {
	name := ""
	if r.InstanceName != nil {
		name = *r.InstanceName
	}
	return params.AdmissionReservation{ID: r.ID, Domain: r.Domain, Owner: r.Owner, RequestKey: r.RequestKey, InstanceName: name, State: r.State, BackendIdentity: r.BackendIdentity, Vector: vector(r)}
}

func lockDomain(tx *gorm.DB, name string) (admissionDomain, error) {
	var d admissionDomain
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("domain = ?", name).First(&d).Error
	return d, err
}

// providerLock serializes enrollment with all local instance creation paths.
func providerLock(tx *gorm.DB, provider string) error {
	if tx.Name() == "postgres" {
		return tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", provider).Error
	}
	return nil
}

func admissionForProvider(tx *gorm.DB, provider string) (admissionDomain, error) {
	var domain admissionDomain
	err := tx.Where("provider = ?", provider).First(&domain).Error
	return domain, err
}

func (s *sqlDatabase) AdmissionForProvider(ctx context.Context, provider string) (params.AdmissionConfig, bool, error) {
	d, err := admissionForProvider(s.conn.WithContext(ctx), provider)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return params.AdmissionConfig{}, false, nil
	}
	if err != nil {
		return params.AdmissionConfig{}, false, err
	}
	var protocol migrations.AdmissionDomain0013
	if err := s.conn.WithContext(ctx).First(&protocol, "domain = ?", d.Domain).Error; err != nil {
		return params.AdmissionConfig{}, false, err
	}
	return params.AdmissionConfig{Protocol: protocol.Protocol, Domain: d.Domain, Provider: d.Provider, BackendIdentity: d.BackendIdentity, RequestedActivationGeneration: d.RequestedActivationGeneration, Project: d.Project, Image: d.Image, Flavor: d.Flavor, OSArch: d.OSArch, InstanceType: d.InstanceType, Capacity: capacity(d), Class: params.AdmissionVector{CPU: d.ClassCPU, RAM: d.ClassRAM, Disk: d.ClassDisk, VMs: d.ClassVMs, Jobs: d.ClassJobs}, Generation: d.Generation, LastResumeGeneration: d.LastResumeGeneration, Drained: d.Drained, HealthyUntil: d.HealthyUntil, HealthObservedAt: d.HealthObservedAt, Ready: d.Ready}, true, nil
}

func validateAdmissionConfig(c params.AdmissionConfig) error {
	if c.Domain == "" || c.Provider == "" || c.Generation <= 0 || !validVector(c.Capacity) || !validVector(c.Class) || c.Class.VMs != 1 || c.Class.CPU < 1 || c.Class.RAM < 1 || c.Class.Disk < 1 {
		return errors.New("invalid admission configuration")
	}
	if c.RequestedActivationGeneration < 0 || (c.RequestedActivationGeneration != 0 && c.RequestedActivationGeneration != c.Generation) {
		return errors.New("invalid requested activation generation")
	}
	return nil
}

func (s *sqlDatabase) ConfigureAdmission(ctx context.Context, c params.AdmissionConfig) error {
	if err := validateAdmissionConfig(c); err != nil {
		return err
	}
	return s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return configureAdmission(tx, c) })
}

func configureAdmission(tx *gorm.DB, c params.AdmissionConfig) error {
	if err := providerLock(tx, c.Provider); err != nil {
		return err
	}
	d, err := lockDomain(tx, c.Domain)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		d = admissionDomain{Domain: c.Domain, Provider: c.Provider, Drained: true}
		if err := tx.Create(&d).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := configureAdmissionProtocol(tx, d, c); err != nil {
		return err
	}
	if err := auditAdmissionInstances(tx, c.Domain, c.Provider); err != nil {
		return err
	}
	if d.Provider != c.Provider {
		return errors.New("admission domain provider cannot change")
	}
	if err := checkAdmissionIdentityChange(tx, d, c); err != nil {
		return err
	}
	if d.Generation > 0 && c.Generation == d.Generation && (d.Project != c.Project || d.Image != c.Image || d.Flavor != c.Flavor || d.OSArch != c.OSArch || d.InstanceType != c.InstanceType || (params.AdmissionVector{CPU: d.ClassCPU, RAM: d.ClassRAM, Disk: d.ClassDisk, VMs: d.ClassVMs, Jobs: d.ClassJobs}) != c.Class) {
		return errors.New("admission identity changes require a new generation")
	}
	if d.Generation > 0 && c.Generation == d.Generation && capacity(d) != c.Capacity {
		return errors.New("admission capacity changes require a new generation")
	}
	if c.Generation < d.Generation {
		return errors.New("admission generation cannot decrease")
	}
	if d.Generation > 0 && c.Generation > d.Generation {
		c.Ready = false
		c.HealthyUntil = time.Time{}
	}
	v := c.Capacity
	return tx.Model(&d).Updates(map[string]interface{}{"requested_activation_generation": c.RequestedActivationGeneration, "project": c.Project, "image": c.Image, "flavor": c.Flavor, "os_arch": c.OSArch, "instance_type": c.InstanceType, "class_cpu": c.Class.CPU, "class_ram": c.Class.RAM, "class_disk": c.Class.Disk, "class_vms": c.Class.VMs, "class_jobs": c.Class.Jobs, "cpu": v.CPU, "ram": v.RAM, "disk": v.Disk, "vms": v.VMs, "jobs": v.Jobs, "generation": c.Generation, "healthy_until": time.Time{}, "ready": false}).Error
}

func checkAdmissionIdentityChange(tx *gorm.DB, d admissionDomain, c params.AdmissionConfig) error {
	identityChanged := d.Project != c.Project || d.Image != c.Image || d.Flavor != c.Flavor || d.OSArch != c.OSArch || d.InstanceType != c.InstanceType || (params.AdmissionVector{CPU: d.ClassCPU, RAM: d.ClassRAM, Disk: d.ClassDisk, VMs: d.ClassVMs, Jobs: d.ClassJobs}) != c.Class
	if d.Generation > 0 && identityChanged {
		var held int64
		if err := tx.Model(&admissionReservation{}).Where("domain = ? AND state <> ?", c.Domain, admissionReleased).Count(&held).Error; err != nil {
			return err
		}
		if held != 0 {
			return errors.New("cannot change admission identity while reservations are held")
		}
	}
	return nil
}

func configureAdmissionProtocol(tx *gorm.DB, d admissionDomain, c params.AdmissionConfig) error {
	var protocol migrations.AdmissionDomain0013
	if err := tx.First(&protocol, "domain = ?", c.Domain).Error; err != nil {
		return err
	}
	if protocol.Protocol != c.Protocol {
		var credits int64
		if err := tx.Model(&migrations.AdmissionCredit0013{}).Where("domain = ?", c.Domain).Count(&credits).Error; err != nil {
			return err
		}
		if credits != 0 {
			return errors.New("cannot change protocol with durable credits")
		}
		if d.Generation > 0 && c.Generation <= d.Generation {
			return errors.New("protocol change requires a new generation")
		}
		if err := tx.Model(&protocol).Update("protocol", c.Protocol).Error; err != nil {
			return err
		}
	}
	return nil
}

// UpdateAdmissionHealth records only the latest probe for the exact configured identity.
// The observation time is the probe start, so a late reply cannot undo a newer failure.
func (s *sqlDatabase) UpdateAdmissionHealth(ctx context.Context, c params.AdmissionConfig) error {
	if c.HealthObservedAt.IsZero() || c.HealthObservedAt.After(time.Now()) {
		return errors.New("invalid health observation time")
	}
	return s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		d, err := lockDomain(tx, c.Domain)
		if err != nil {
			return err
		}
		if d.Provider != c.Provider || d.Generation != c.Generation || d.Project != c.Project || d.Image != c.Image || d.Flavor != c.Flavor || d.OSArch != c.OSArch || d.InstanceType != c.InstanceType || (params.AdmissionVector{CPU: d.ClassCPU, RAM: d.ClassRAM, Disk: d.ClassDisk, VMs: d.ClassVMs, Jobs: d.ClassJobs}) != c.Class {
			return errors.New("health observation does not match admission configuration")
		}
		if c.Ready {
			b, err := hex.DecodeString(c.BackendIdentity)
			if err != nil || len(b) != 32 || strings.ToLower(c.BackendIdentity) != c.BackendIdentity {
				return errors.New("invalid backend identity")
			}
			if d.BackendIdentity == "" {
				var held int64
				if err := tx.Model(&admissionReservation{}).Where("domain = ? AND state <> ?", c.Domain, admissionReleased).Count(&held).Error; err != nil {
					return err
				}
				if held != 0 {
					return errors.New("cannot pin backend with held reservations")
				}
			} else if d.BackendIdentity != c.BackendIdentity {
				return errors.New("backend identity changed")
			}
		}
		if !c.HealthObservedAt.After(d.HealthObservedAt) {
			if c.Ready {
				return errors.New("health observation superseded")
			}
			// A failure always closes admission, including a delayed failure.
			// Keep the newer timestamp so an older success cannot reopen it.
			return tx.Model(&d).Updates(map[string]interface{}{"ready": false, "healthy_until": time.Time{}}).Error
		}
		updates := map[string]interface{}{"ready": c.Ready, "healthy_until": c.HealthyUntil, "health_observed_at": c.HealthObservedAt}
		if c.Ready && d.BackendIdentity == "" {
			updates["backend_identity"] = c.BackendIdentity
		}
		return tx.Model(&d).Updates(updates).Error
	})
}

func (s *sqlDatabase) SetAdmissionDrain(ctx context.Context, domain string, drained bool) error {
	if !drained {
		return errors.New("use generation-gated ResumeAdmission")
	}
	return s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		d, err := lockDomain(tx, domain)
		if err != nil {
			return err
		}
		return tx.Model(&d).Updates(map[string]interface{}{"drained": true, "last_resume_generation": max(d.LastResumeGeneration, d.Generation)}).Error
	})
}

// auditAdmissionInstances runs under the provider and domain locks.
func auditAdmissionInstances(tx *gorm.DB, domain, provider string) error {
	var unknown int64
	if err := tx.Raw("SELECT COUNT(*) FROM instances i LEFT JOIN pools p ON i.pool_id = p.id LEFT JOIN scale_sets s ON i.scale_set_fk_id = s.id LEFT JOIN admission_reservations a ON a.instance_name = i.name AND a.domain = ? AND a.owner = ('scaleset:' || CAST(s.id AS TEXT)) AND a.state <> 'released' WHERE p.provider_name = ? OR (s.provider_name = ? AND a.id IS NULL)", domain, provider, provider).Scan(&unknown).Error; err != nil {
		return err
	}
	if unknown != 0 {
		return errors.New("provider has unaccounted instances")
	}
	return nil
}

// ResumeAdmission consumes a new explicit activation generation. The caller
// must independently verify provider inventory before asking to resume.
func (s *sqlDatabase) ResumeAdmission(ctx context.Context, domain string, generation int64) error {
	if generation <= 0 {
		return errors.New("invalid activation generation")
	}
	return s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var initial admissionDomain
		if err := tx.First(&initial, "domain = ?", domain).Error; err != nil {
			return err
		}
		if err := providerLock(tx, initial.Provider); err != nil {
			return err
		}
		d, err := lockDomain(tx, domain)
		if err != nil {
			return err
		}
		if d.Generation != generation || d.RequestedActivationGeneration != generation || generation <= d.LastResumeGeneration {
			return errors.New("activation generation already used or does not match configuration")
		}
		if !d.Ready || d.BackendIdentity == "" || !d.HealthyUntil.After(time.Now()) {
			return errors.New("admission health unavailable or stale")
		}
		if err := auditAdmissionInstances(tx, domain, d.Provider); err != nil {
			return err
		}
		return tx.Model(&d).Updates(map[string]interface{}{"drained": false, "last_resume_generation": generation}).Error
	})
}

func fits(used, add, limit int64) bool {
	return used >= 0 && add >= 0 && used <= math.MaxInt64-add && used+add <= limit
}

func (s *sqlDatabase) ReserveAdmission(ctx context.Context, domain, owner, key string, v params.AdmissionVector, generation int64) (params.AdmissionReservation, error) {
	var out params.AdmissionReservation
	if domain == "" || owner == "" || key == "" || !validVector(v) {
		return out, errors.New("invalid admission reservation")
	}
	err := s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return reserveAdmission(tx, domain, owner, key, v, generation, &out) })
	return out, err
}

func reserveAdmission(tx *gorm.DB, domain, owner, key string, v params.AdmissionVector, generation int64, out *params.AdmissionReservation) error {
	d, err := lockDomain(tx, domain)
	if err != nil {
		return err
	}
	var existing admissionReservation
	err = tx.Where("domain = ? AND owner = ? AND request_key = ?", domain, owner, key).First(&existing).Error
	if err == nil {
		if existing.State == admissionReleased {
			return errors.New("reservation has already been released")
		}
		if vector(existing) != v {
			return errors.New("reservation key reused with different vector")
		}
		*out = result(existing)
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if d.Drained || !d.Ready || d.BackendIdentity == "" || d.Generation != generation || !d.HealthyUntil.After(time.Now()) {
		return errors.New("admission unavailable or health stale")
	}
	var rows []admissionReservation
	if err := tx.Where("domain = ? AND state <> ?", domain, admissionReleased).Find(&rows).Error; err != nil {
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
	budget := capacity(d)
	if !fits(used.CPU, v.CPU, budget.CPU) || !fits(used.RAM, v.RAM, budget.RAM) || !fits(used.Disk, v.Disk, budget.Disk) || !fits(used.VMs, v.VMs, budget.VMs) || !fits(used.Jobs, v.Jobs, budget.Jobs) {
		return errors.New("admission capacity exhausted")
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	r := admissionReservation{ID: id.String(), Domain: domain, Owner: owner, RequestKey: key, State: admissionReserved, Generation: generation, BackendIdentity: d.BackendIdentity, CPU: v.CPU, RAM: v.RAM, Disk: v.Disk, VMs: v.VMs, Jobs: v.Jobs}
	if err := tx.Create(&r).Error; err != nil {
		return err
	}
	*out = result(r)
	return nil
}

// admissionChange locks the domain first so state changes serialize with reservations and drain.
func (s *sqlDatabase) admissionChange(ctx context.Context, id string, change func(*gorm.DB, *admissionDomain, *admissionReservation) error) error {
	return s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var r admissionReservation
		if err := tx.First(&r, "id = ?", id).Error; err != nil {
			return err
		}
		d, err := lockDomain(tx, r.Domain)
		if err != nil {
			return err
		}
		if err := tx.First(&r, "id = ?", id).Error; err != nil {
			return err
		}
		return change(tx, &d, &r)
	})
}

func (s *sqlDatabase) BindAdmission(ctx context.Context, id, name string) error {
	if name == "" {
		return errors.New("empty instance name")
	}
	return s.admissionChange(ctx, id, func(tx *gorm.DB, _ *admissionDomain, r *admissionReservation) error {
		var creditCount int64
		if err := tx.Model(&admissionCredit{}).Where("reservation_id = ?", r.ID).Count(&creditCount).Error; err != nil {
			return err
		}
		if creditCount != 0 {
			return errors.New("job credit requires a job-bound instance")
		}
		if r.InstanceName != nil && *r.InstanceName == name && r.State != admissionReleased {
			return nil
		}
		if (r.State != admissionReserved && r.State != admissionSending) || r.InstanceName != nil {
			return errors.New("reservation cannot be bound")
		}
		return tx.Model(r).Update("instance_name", name).Error
	})
}

func (s *sqlDatabase) MarkAdmissionSending(ctx context.Context, id string) error {
	return s.admissionChange(ctx, id, func(tx *gorm.DB, d *admissionDomain, r *admissionReservation) error {
		if r.State == admissionSending {
			return errors.New("reservation may already have been sent")
		}
		if r.State != admissionReserved {
			return errors.New("reservation cannot be sent")
		}
		if d.Drained || !d.Ready || d.BackendIdentity == "" || d.BackendIdentity != r.BackendIdentity || d.Generation != r.Generation || !d.HealthyUntil.After(time.Now()) {
			return errors.New("admission unavailable or health stale")
		}
		return tx.Model(r).Update("state", admissionSending).Error
	})
}

func (s *sqlDatabase) ReleaseUnsentAdmission(ctx context.Context, id string) error {
	return s.admissionChange(ctx, id, func(tx *gorm.DB, _ *admissionDomain, r *admissionReservation) error {
		var credits int64
		if err := tx.Model(&admissionCredit{}).Where("reservation_id = ?", r.ID).Count(&credits).Error; err != nil {
			return err
		}
		if credits > 0 {
			return errors.New("durable credit cannot be released")
		}
		if r.State == admissionReleased {
			return nil
		}
		if r.State != admissionReserved || r.InstanceName != nil {
			return errors.New("reservation may have external effects")
		}
		return tx.Model(r).Update("state", admissionReleased).Error
	})
}

// releaseBoundAdmission removes the charged instance and its reservation in one
// transaction. Repeated calls after a committed removal are idempotent.
func (s *sqlDatabase) releaseBoundAdmission(ctx context.Context, id string, cancelUnsent bool, expectedName string) (bool, error) {
	var notification *params.Instance
	released := false
	err := s.admissionChange(ctx, id, func(tx *gorm.DB, _ *admissionDomain, r *admissionReservation) error {
		if r.State == admissionReleased {
			released = true
			return nil
		}
		if r.InstanceName == nil {
			return errors.New("reservation is not bound")
		}
		if expectedName != "" && *r.InstanceName != expectedName {
			return errors.New("reservation instance changed")
		}
		if expectedName == "" {
			var credits int64
			if err := tx.Model(&admissionCredit{}).Where("reservation_id = ?", r.ID).Count(&credits).Error; err != nil {
				return err
			}
			if credits > 0 {
				return errors.New("credit cleanup requires the bound instance identity")
			}
		}

		if cancelUnsent {
			if r.State != admissionReserved {
				return nil
			}
		} else if r.State != "deleting" {
			return errors.New("authoritative absence requires a deleting instance")
		}
		var instance Instance
		err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ?", *r.InstanceName).First(&instance).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) && cancelUnsent {
			return errors.New("bound instance is missing")
		}
		if err == nil {
			if instance.ScaleSetFkID == nil || r.Owner != fmt.Sprintf("scaleset:%d", *instance.ScaleSetFkID) {
				return errors.New("bound instance owner mismatch")
			}
			if cancelUnsent {
				if instance.ProviderID != nil && *instance.ProviderID != "" {
					return nil
				}
				if instance.RunnerStatus != params.RunnerPending {
					return nil
				}
				switch instance.Status {
				case commonParams.InstancePendingCreate, commonParams.InstanceCreating, commonParams.InstancePendingDelete, commonParams.InstancePendingForceDelete, commonParams.InstanceDeleting:
				default:
					return nil
				}
			}
			payload := params.Instance{ID: instance.ID.String(), Name: instance.Name, AgentID: instance.AgentID, ScaleSetID: *instance.ScaleSetFkID}
			if instance.ProviderID != nil {
				payload.ProviderID = *instance.ProviderID
			}
			if err := tx.Unscoped().Delete(&instance).Error; err != nil {
				return err
			}
			notification = &payload
		}
		if err := releaseAdmissionCharge(tx, r); err != nil {
			return err
		}
		released = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if notification != nil {
		if err := s.sendNotify(common.InstanceEntityType, common.DeleteOperation, *notification); err != nil {
			slog.With(slog.Any("error", err)).Error("failed to send notify")
		}
	}
	return released, nil
}

func releaseAdmissionCharge(tx *gorm.DB, r *admissionReservation) error {
	var credit admissionCredit
	creditErr := tx.Where("reservation_id = ?", r.ID).First(&credit).Error
	if creditErr == nil {
		var job admissionJob
		if err := tx.Where("domain = ? AND owner = ? AND instance_name = ?", r.Domain, r.Owner, *r.InstanceName).First(&job).Error; err != nil {
			return err
		}
		if err := tx.Model(&job).Update("absent", true).Error; err != nil {
			return err
		}
		job.Absent = true
		if err := parkCreditIfDone(tx, credit, job); err != nil {
			return err
		}
	} else if !errors.Is(creditErr, gorm.ErrRecordNotFound) {
		return creditErr
	} else if err := tx.Model(r).Update("state", admissionReleased).Error; err != nil {
		return err
	}
	return nil
}

// ConfirmAdmissionAbsent requires authoritative absence after a completed delete.
func (s *sqlDatabase) ConfirmAdmissionAbsent(ctx context.Context, id string) error {
	_, err := s.releaseBoundAdmission(ctx, id, false, "")
	return err
}

// CancelUnsentBoundAdmission atomically retires a bound create never sent to a provider.
func (s *sqlDatabase) CancelUnsentBoundAdmission(ctx context.Context, provider, name string) (bool, error) {
	r, err := s.AdmissionByInstance(ctx, provider, name)
	if errors.Is(err, common.ErrAdmissionNotFound) {
		return s.jobInstanceAlreadyAbsent(ctx, provider, name)
	}
	if err != nil {
		return false, err
	}
	return s.releaseBoundAdmission(ctx, r.ID, true, name)
}

// BeginAdmittedCreate marks the provider operation uncertain before it is sent.
// A sending reservation cannot be retried without authoritative reconciliation.
func (s *sqlDatabase) BeginAdmittedCreate(ctx context.Context, provider, owner, runnerName string, shape params.AdmissionCreateShape) (params.AdmissionReservation, bool, error) {
	var authorized params.AdmissionReservation
	enrolled := false
	err := s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := providerLock(tx, provider); err != nil {
			return err
		}
		d, err := admissionForProvider(tx, provider)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		enrolled = true
		d, err = lockDomain(tx, d.Domain)
		if err != nil {
			return err
		}
		var r admissionReservation
		if err := tx.Where("domain = ? AND owner = ? AND instance_name = ?", d.Domain, owner, runnerName).First(&r).Error; err != nil {
			return err
		}
		isCredit, err := checkJobCreditSendBudget(tx, d, r.ID)
		if err != nil {
			return err
		}
		var instance Instance
		if err := tx.Where("name = ?", runnerName).First(&instance).Error; err != nil {
			return err
		}
		if instance.ScaleSetFkID == nil || owner != fmt.Sprintf("scaleset:%d", *instance.ScaleSetFkID) {
			return errors.New("instance does not match admission owner")
		}
		var scaleSet ScaleSet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&scaleSet, *instance.ScaleSetFkID).Error; err != nil {
			return err
		}
		if err := checkProviderCreateShape(shape, scaleSet, d, provider); err != nil {
			return err
		}
		if r.State != admissionReserved {
			return errors.New("provider create already sent or reservation released")
		}
		if d.Drained || !d.Ready || d.BackendIdentity == "" || d.BackendIdentity != r.BackendIdentity || (d.Generation != r.Generation && !isCredit) || !d.HealthyUntil.After(time.Now()) {
			return errors.New("admission unavailable or health stale")
		}
		if vector(r) != (params.AdmissionVector{CPU: d.ClassCPU, RAM: d.ClassRAM, Disk: d.ClassDisk, VMs: d.ClassVMs, Jobs: d.ClassJobs}) {
			return errors.New("reservation class changed")
		}
		if err := tx.Model(&r).Updates(map[string]interface{}{"state": admissionSending, "generation": d.Generation}).Error; err != nil {
			return err
		}
		r.State = admissionSending
		authorized = result(r)
		return nil
	})
	return authorized, enrolled, err
}

func checkProviderCreateShape(shape params.AdmissionCreateShape, scaleSet ScaleSet, d admissionDomain, provider string) error {
	if shape.Generation != scaleSet.Generation || shape.Image != scaleSet.Image || shape.Flavor != scaleSet.Flavor || shape.OSArch != string(scaleSet.OSArch) {
		return errors.New("provider create snapshot is stale")
	}
	if !scaleSet.Enabled || scaleSet.MaxRunners < 1 || scaleSet.ProviderName != provider || scaleSet.MinIdleRunners != 0 || scaleSet.Image != d.Image || scaleSet.Flavor != d.Flavor || string(scaleSet.OSArch) != d.OSArch {
		return errors.New("instance does not match enrolled provider")
	}
	return nil
}

func checkJobCreditSendBudget(tx *gorm.DB, d admissionDomain, id string) (bool, error) {
	var count int64
	if err := tx.Model(&admissionCredit{}).Where("reservation_id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		if err := checkAdmissionHeldBudget(tx, d); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// AdmissionByInstance resolves the durable charge even after the instance row
// has been removed. A missing reservation is never proof of provider absence.
func (s *sqlDatabase) AdmissionByInstance(ctx context.Context, provider, name string) (params.AdmissionReservation, error) {
	var out params.AdmissionReservation
	d, err := admissionForProvider(s.conn.WithContext(ctx), provider)
	if err != nil {
		return out, err
	}
	var r admissionReservation
	if err := s.conn.WithContext(ctx).Where("domain = ? AND instance_name = ?", d.Domain, name).First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return out, common.ErrAdmissionNotFound
		}
		return out, err
	}
	return result(r), nil
}

// MarkAdmissionPresent records a successful provider create response. A lost or
// failed response must remain sending until independently reconciled.
func (s *sqlDatabase) MarkAdmissionPresent(ctx context.Context, provider, name string) error {
	r, err := s.AdmissionByInstance(ctx, provider, name)
	if err != nil {
		return err
	}
	return s.admissionChange(ctx, r.ID, func(tx *gorm.DB, _ *admissionDomain, row *admissionReservation) error {
		if row.State == "present" {
			return nil
		}
		if row.State != admissionSending {
			return errors.New("instance has no completed create operation")
		}
		return tx.Model(row).Update("state", "present").Error
	})
}

// BeginAdmissionDelete serializes the delete intent before the provider call.
// An uncertain create or an already issued delete must not be retried here.
func (s *sqlDatabase) BeginAdmissionDelete(ctx context.Context, provider, name string) error {
	r, err := s.AdmissionByInstance(ctx, provider, name)
	if err != nil {
		return err
	}
	return s.admissionChange(ctx, r.ID, func(tx *gorm.DB, _ *admissionDomain, row *admissionReservation) error {
		if row.State != "present" {
			return errors.New("instance create is uncertain or delete already sent")
		}
		return tx.Model(row).Update("state", "deleting").Error
	})
}

// ConfirmAdmissionAbsentByInstance requires a successful delete and a provider
// GetInstance response explicitly establishing absence, with no in-flight create.
func (s *sqlDatabase) ConfirmAdmissionAbsentByInstance(ctx context.Context, provider, name string) error {
	r, err := s.AdmissionByInstance(ctx, provider, name)
	if errors.Is(err, common.ErrAdmissionNotFound) {
		absent, lookupErr := s.jobInstanceAlreadyAbsent(ctx, provider, name)
		if lookupErr != nil {
			return lookupErr
		}
		if absent {
			return nil
		}
		return common.ErrAdmissionNotFound
	}
	if err != nil {
		return err
	}
	_, err = s.releaseBoundAdmission(ctx, r.ID, false, name)
	return err
}
