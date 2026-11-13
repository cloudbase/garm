// Copyright 2022 Cloudbase Solutions SRL
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
	"fmt"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

//go:generate go run github.com/vektra/mockery/v2@latest
type Provider interface {
	// CreateInstance creates a new compute instance in the provider.
	CreateInstance(ctx context.Context, bootstrapParams commonParams.BootstrapInstance, createInstanceParams CreateInstanceParams) (commonParams.ProviderInstance, error)
	// Delete instance will delete the instance in a provider.
	DeleteInstance(ctx context.Context, instance string, deleteInstanceParams DeleteInstanceParams) error
	// GetInstance will return details about one instance.
	GetInstance(ctx context.Context, instance string, getInstanceParams GetInstanceParams) (commonParams.ProviderInstance, error)
	// ListInstances will list all instances for a provider.
	ListInstances(ctx context.Context, poolID string, listInstancesParams ListInstancesParams) ([]commonParams.ProviderInstance, error)
	// RemoveAllInstances will remove all instances created by this provider.
	RemoveAllInstances(ctx context.Context, removeAllInstancesParams RemoveAllInstancesParams) error
	// Stop shuts down the instance.
	Stop(ctx context.Context, instance string, stopParams StopParams) error
	// Start boots up an instance.
	Start(ctx context.Context, instance string, startParams StartParams) error
	// DisableJITConfig tells us if the provider explicitly disables JIT configuration and
	// forces runner registration tokens to be used. This may happen if a provider has not yet
	// been updated to support JIT configuration.
	DisableJITConfig() bool

	AsParams() params.Provider
}

// AbsenceRequest identifies the exact provider domain and guest.
type AbsenceRequest struct {
	ExpectedBackendIdentity string `json:"expected_backend_identity"`
	Version                 int    `json:"version"`
	RequestID               string `json:"request_id"`
	Generation              string `json:"generation"`
	Domain                  string `json:"domain"`
	Project                 string `json:"project"`
	InstanceName            string `json:"instance_name"`
}
type AbsenceResponse struct {
	BackendIdentity string         `json:"backend_identity"`
	Request         AbsenceRequest `json:"request"`
	Absent          bool           `json:"absent"`
	CheckedAt       time.Time      `json:"checked_at"`
	ValidUntil      time.Time      `json:"valid_until"`
	Contract        string         `json:"contract"`
	Reason          string         `json:"reason,omitempty"`
}

// AuthoritativeAbsenceProvider proves absence for the exact known operation.
type AuthoritativeAbsenceProvider interface {
	ConfirmAuthoritativeAbsence(context.Context, AbsenceRequest) (AbsenceResponse, error)
}

// ValidateAbsence accepts only the single-node Incus absence contract.
func ValidateAbsence(req AbsenceRequest, res AbsenceResponse, now time.Time) error {
	if req.Version != 1 || req.RequestID == "" || req.Generation == "" || req.Domain == "" || req.Project == "" || req.InstanceName == "" || req.ExpectedBackendIdentity == "" || res.BackendIdentity != req.ExpectedBackendIdentity || res.Request != req || !res.Absent || res.Contract != "incus-single-node-absence-v1" {
		return fmt.Errorf("provider did not confirm authoritative absence")
	}
	if res.CheckedAt.IsZero() || res.CheckedAt.After(now) || now.Sub(res.CheckedAt) > 15*time.Second || !res.ValidUntil.After(now) || res.ValidUntil.After(res.CheckedAt.Add(15*time.Second)) {
		return fmt.Errorf("absence confirmation is stale")
	}
	return nil
}

// AdmittedDeleteRequest binds a one-shot delete to an enrolled backend.
type AdmittedDeleteRequest struct {
	Version                 int    `json:"version"`
	RequestID               string `json:"request_id"`
	ExpectedBackendIdentity string `json:"expected_backend_identity"`
	Project                 string `json:"project"`
	InstanceName            string `json:"instance_name"`
}
type AdmittedDeleteResponse struct {
	Reason          string                `json:"reason,omitempty"`
	Request         AdmittedDeleteRequest `json:"request"`
	Deleted         bool                  `json:"deleted"`
	BackendIdentity string                `json:"backend_identity"`
}
type AdmittedDeleteProvider interface {
	DeleteAdmitted(context.Context, AdmittedDeleteRequest) (AdmittedDeleteResponse, error)
}

// AdmissionInventoryRequest requires an inventory from the pinned backend.
type AdmissionInventoryRequest struct {
	Version                 int    `json:"version"`
	RequestID               string `json:"request_id"`
	ExpectedBackendIdentity string `json:"expected_backend_identity"`
	Project                 string `json:"project"`
}
type AdmissionInventoryInstance struct {
	Name       string `json:"name"`
	ProviderID string `json:"provider_id"`
}
type AdmissionInventoryResponse struct {
	Reason          string                       `json:"reason,omitempty"`
	Request         AdmissionInventoryRequest    `json:"request"`
	BackendIdentity string                       `json:"backend_identity"`
	Complete        bool                         `json:"complete"`
	Instances       []AdmissionInventoryInstance `json:"instances"`
}
type AdmissionInventoryProvider interface {
	AdmissionInventory(context.Context, AdmissionInventoryRequest) (AdmissionInventoryResponse, error)
}
