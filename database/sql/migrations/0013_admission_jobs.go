// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

package migrations

import (
	"errors"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

type AdmissionDomain0013 struct {
	AdmissionDomain0012
	Protocol string
}

func (AdmissionDomain0013) TableName() string { return "admission_domains" }

type AdmissionCredit0013 struct {
	Domain        string `gorm:"primaryKey;size:255"`
	Owner         string `gorm:"primaryKey;size:255"`
	ReservationID string `gorm:"size:36;uniqueIndex"`
	PollEpoch     int64
	PollPending   bool
}

func (AdmissionCredit0013) TableName() string { return "admission_credits" }

type AdmissionJob0013 struct {
	ID           string `gorm:"primaryKey;size:36"`
	Domain       string `gorm:"size:255;uniqueIndex:idx_admission_job_request,priority:1"`
	Owner        string `gorm:"size:255;uniqueIndex:idx_admission_job_request,priority:2"`
	RequestID    string `gorm:"size:255;uniqueIndex:idx_admission_job_request,priority:3"`
	JobID        string `gorm:"size:255;index"`
	InstanceName string
	State        string
	Terminal     bool
	Absent       bool
}

func (AdmissionJob0013) TableName() string { return "admission_jobs" }

func init() {
	Register(&gormigrate.Migration{ID: "0013_admission_jobs", Migrate: func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&AdmissionDomain0013{}, &AdmissionCredit0013{}, &AdmissionJob0013{}); err != nil {
			return err
		}
		return tx.Model(&AdmissionDomain0013{}).Where("protocol IS NULL OR protocol = ?", "").Update("protocol", "vm-only-experimental").Error
	}, Rollback: func(tx *gorm.DB) error {
		var credits int64
		if err := tx.Model(&AdmissionCredit0013{}).Count(&credits).Error; err != nil {
			return err
		}
		if credits > 0 {
			return errors.New("cannot roll back admission jobs with durable credits")
		}
		if err := tx.Migrator().DropTable(&AdmissionJob0013{}, &AdmissionCredit0013{}); err != nil {
			return err
		}
		return tx.Migrator().DropColumn(&AdmissionDomain0013{}, "Protocol")
	}})
}
