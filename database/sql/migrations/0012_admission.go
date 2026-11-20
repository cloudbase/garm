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

package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AdmissionDomain0012 and AdmissionReservation0012 are frozen migration models.
type AdmissionDomain0012 struct {
	Domain                                             string `gorm:"primaryKey;size:255"`
	Provider                                           string `gorm:"size:255;uniqueIndex"`
	Project, Image, Flavor, OSArch, InstanceType       string
	BackendIdentity                                    string
	ClassCPU, ClassRAM, ClassDisk, ClassVMs, ClassJobs int64
	CPU, RAM, Disk, VMs, Jobs                          int64
	Generation                                         int64
	LastResumeGeneration                               int64
	RequestedActivationGeneration                      int64
	HealthyUntil                                       time.Time
	HealthObservedAt                                   time.Time
	Ready                                              bool
	Drained                                            bool
}

func (AdmissionDomain0012) TableName() string { return "admission_domains" }

type AdmissionReservation0012 struct {
	ID                        string `gorm:"primaryKey;size:36"`
	Domain                    string `gorm:"size:255;uniqueIndex:idx_admission_request,priority:1;index"`
	Owner                     string `gorm:"size:255;uniqueIndex:idx_admission_request,priority:2"`
	RequestKey                string `gorm:"size:255;uniqueIndex:idx_admission_request,priority:3"`
	BackendIdentity           string
	InstanceName              *string `gorm:"size:255;uniqueIndex"`
	Generation                int64
	State                     string
	CPU, RAM, Disk, VMs, Jobs int64
}

func (AdmissionReservation0012) TableName() string { return "admission_reservations" }

func init() {
	Register(&gormigrate.Migration{ID: "0012_admission", Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(&AdmissionDomain0012{}, &AdmissionReservation0012{})
	}, Rollback: func(tx *gorm.DB) error {
		return tx.Migrator().DropTable(&AdmissionReservation0012{}, &AdmissionDomain0012{})
	}})
}
