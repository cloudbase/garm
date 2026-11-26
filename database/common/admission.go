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

package common

import (
	"context"
	"errors"

	"github.com/cloudbase/garm/params"
)

var ErrAdmissionNotFound = errors.New("admission reservation not found")

// AdmissionStore is optional. All writers in a physical domain must participate.
type AdmissionStore interface {
	CreateAdmittedScaleSetInstance(context.Context, string, uint, params.CreateInstanceParams) (params.Instance, error)
	BeginAdmittedCreate(context.Context, string, string, string, params.AdmissionCreateShape) (params.AdmissionReservation, bool, error)
	AdmissionForProvider(context.Context, string) (params.AdmissionConfig, bool, error)
	ConfigureAdmission(context.Context, params.AdmissionConfig) error
	UpdateAdmissionHealth(context.Context, params.AdmissionConfig) error
	SetAdmissionDrain(context.Context, string, bool) error
	ResumeAdmission(context.Context, string, int64) error
	AdmissionByInstance(context.Context, string, string) (params.AdmissionReservation, error)
	MarkAdmissionPresent(context.Context, string, string) error
	BeginAdmissionDelete(context.Context, string, string) error
	CancelUnsentBoundAdmission(context.Context, string, string) (bool, error)
	ConfirmAdmissionAbsentByInstance(context.Context, string, string) error
	ReserveAdmission(context.Context, string, string, string, params.AdmissionVector, int64) (params.AdmissionReservation, error)
	BindAdmission(context.Context, string, string) error
	MarkAdmissionSending(context.Context, string) error
	// ReleaseUnsentAdmission only releases an unbound reservation never marked sending.
	ReleaseUnsentAdmission(context.Context, string) error
	// ConfirmAdmissionAbsent requires authoritative proof that no guest or in-flight create exists.
	ConfirmAdmissionAbsent(context.Context, string) error
}
