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

package params

import "time"

// AdmissionVector is the immutable resource charge for a reservation.
type AdmissionVector struct {
	CPU, RAM, Disk, VMs, Jobs int64
}

type AdmissionConfig struct {
	Domain                                       string
	Protocol                                     string
	Provider                                     string
	BackendIdentity                              string
	Project, Image, Flavor, OSArch, InstanceType string
	Class                                        AdmissionVector
	Capacity                                     AdmissionVector
	Generation                                   int64
	LastResumeGeneration                         int64
	RequestedActivationGeneration                int64
	Drained                                      bool
	HealthyUntil                                 time.Time
	HealthObservedAt                             time.Time
	Ready                                        bool
}

type AdmissionReservation struct {
	ID, Domain, Owner, RequestKey, InstanceName, State string
	BackendIdentity                                    string
	PollEpoch                                          int64
	PollPending                                        bool
	Vector                                             AdmissionVector
}

// AdmissionCreateShape is the provider worker snapshot checked before create.
type AdmissionCreateShape struct {
	Image, Flavor, OSArch string
	Generation            uint64
}

// AdmissionJob is a durable assignment tombstone and its optional VM binding.
type AdmissionJob struct {
	ID, Domain, Owner, RequestID, JobID, InstanceName string
	Terminal, Absent                                  bool
	State                                             string
}
