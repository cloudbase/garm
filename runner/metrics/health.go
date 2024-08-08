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
