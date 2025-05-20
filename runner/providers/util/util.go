// Copyright 2025 Cloudbase Solutions SRL
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

package util

import (
	commonParams "github.com/cloudbase/garm-provider-common/params"
)

// IsProviderValidStatus checks if the given status is valid for the provider.
// A provider should only return a status indicating that the instance is in a
// lifecycle state that it can influence. The sole purpose of a provider is to
// manage the lifecycle of an instance. Statuses that indicate an instance should
// be created or removed, will be set by the controller.
func IsValidProviderStatus(status commonParams.InstanceStatus) bool {
	switch status {
	case commonParams.InstanceRunning, commonParams.InstanceError,
		commonParams.InstanceStopped, commonParams.InstanceStatusUnknown:

		return true
	default:
		return false
	}
}
