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

package metrics

import (
	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
)

func CollectHealthMetric(controllerInfo params.ControllerInfo) error {
	metrics.GarmHealth.WithLabelValues(
		controllerInfo.MetadataURL,           // label: metadata_url
		controllerInfo.CallbackURL,           // label: callback_url
		controllerInfo.WebhookURL,            // label: webhook_url
		controllerInfo.ControllerWebhookURL,  // label: controller_webhook_url
		controllerInfo.ControllerID.String(), // label: controller_id
	).Set(1)
	return nil
}
