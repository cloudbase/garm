// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

func TestAdmissionResourcesAreCopiedAndReserved(t *testing.T) {
	input := json.RawMessage(`{"custom":"value"}`)
	output, err := withAdmissionResources(input, params.AdmissionVector{CPU: 2, RAM: 3, Disk: 4, VMs: 1}, "backend")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatal(err)
	}
	if string(input) != `{"custom":"value"}` || string(got["custom"]) != `"value"` {
		t.Fatal("original extras were modified")
	}
	var resources struct {
		CPU    int64 `json:"cpu"`
		Memory int64 `json:"memory_bytes"`
		Disk   int64 `json:"disk_bytes"`
		VMs    int64 `json:"vm_slots"`
	}
	if err := json.Unmarshal(got["admission_resources"], &resources); err != nil {
		t.Fatal(err)
	}
	if resources.CPU != 2 || resources.Memory != 3 || resources.Disk != 4 || resources.VMs != 1 {
		t.Fatalf("unexpected resources: %+v", resources)
	}
	if _, err := withAdmissionResources(json.RawMessage(`{"admission_resources":{}}`), params.AdmissionVector{}, "backend"); err == nil {
		t.Fatal("expected reserved key rejection")
	}
}

type forcedDeleteHelper struct {
	providerHelper
	enrolled bool
}

func (h forcedDeleteHelper) admissionEnrolled(context.Context, string) (bool, error) {
	return h.enrolled, nil
}

func TestEnrolledForcedDeleteDoesNotIgnoreProviderFailure(t *testing.T) {
	manager := &instanceManager{ctx: context.Background(), helper: forcedDeleteHelper{enrolled: true}}
	forced, err := manager.effectiveForceDelete(commonParams.InstancePendingForceDelete)
	if err != nil || forced {
		t.Fatalf("enrolled deletion must not suppress provider errors: forced=%v err=%v", forced, err)
	}
	manager.helper = forcedDeleteHelper{}
	forced, err = manager.effectiveForceDelete(commonParams.InstancePendingForceDelete)
	if err != nil || !forced {
		t.Fatalf("legacy forced deletion changed: forced=%v err=%v", forced, err)
	}
}

type cancellationHelper struct {
	providerHelper
	canceled   bool
	beginErr   error
	beginCalls int
}

func (*cancellationHelper) admissionEnrolled(context.Context, string) (bool, error) { return true, nil }
func (h *cancellationHelper) cancelUnsentAdmission(context.Context, string, string) (bool, error) {
	return h.canceled, nil
}

func (h *cancellationHelper) beginAdmissionDelete(context.Context, string, string) error {
	h.beginCalls++
	return h.beginErr
}

func TestCancelUnsentSkipsProviderAndUncertainCreateIsHeld(t *testing.T) {
	helper := &cancellationHelper{canceled: true}
	manager := &instanceManager{ctx: context.Background(), helper: helper, instance: params.Instance{Name: "runner", Status: commonParams.InstancePendingDelete}}
	manager.running.Store(true)
	if err := manager.consolidateState(); err != nil {
		t.Fatal(err)
	}
	if helper.beginCalls != 0 {
		t.Fatal("canceled allocation attempted provider deletion")
	}
	helper.canceled = false
	helper.beginErr = errors.New("create outcome is uncertain")
	if err := manager.consolidateState(); err == nil {
		t.Fatal("uncertain create must remain held")
	}
	if helper.beginCalls != 1 {
		t.Fatal("expected delete authorization to reject uncertain create")
	}
}
