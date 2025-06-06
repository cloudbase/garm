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

package common

import (
	garmErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/runner/providers/util"
)

func ValidateResult(inst commonParams.ProviderInstance) error {
	if inst.ProviderID == "" {
		return garmErrors.NewProviderError("missing provider ID")
	}

	if inst.Name == "" {
		return garmErrors.NewProviderError("missing instance name")
	}

	if !util.IsValidProviderStatus(inst.Status) {
		return garmErrors.NewProviderError("invalid status returned (%s)", inst.Status)
	}

	return nil
}
