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

type InstanceStatus string
type RunnerStatus string

const (
	InstanceRunning       InstanceStatus = "running"
	InstanceStopped       InstanceStatus = "stopped"
	InstanceError         InstanceStatus = "error"
	InstancePendingDelete InstanceStatus = "pending_delete"
	InstanceDeleting      InstanceStatus = "deleting"
	InstancePendingCreate InstanceStatus = "pending_create"
	InstanceCreating      InstanceStatus = "creating"
	InstanceStatusUnknown InstanceStatus = "unknown"

	RunnerIdle       RunnerStatus = "idle"
	RunnerPending    RunnerStatus = "pending"
	RunnerTerminated RunnerStatus = "terminated"
	RunnerInstalling RunnerStatus = "installing"
	RunnerFailed     RunnerStatus = "failed"
	RunnerActive     RunnerStatus = "active"
)

func IsValidStatus(status InstanceStatus) bool {
	switch status {
	case InstanceRunning, InstanceError, InstancePendingCreate,
		InstancePendingDelete, InstanceStatusUnknown, InstanceStopped,
		InstanceCreating, InstanceDeleting:

		return true
	default:
		return false
	}
}
