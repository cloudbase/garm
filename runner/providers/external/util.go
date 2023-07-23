package external

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

func providerInstanceToParamsInstance(inst commonParams.ProviderInstance) commonParams.ProviderInstance {
	return commonParams.ProviderInstance{
		ProviderID: inst.ProviderID,
		Name:       inst.Name,
		OSName:     inst.OSName,
		OSArch:     inst.OSArch,
		OSType:     inst.OSType,
		Status:     inst.Status,
	}
}
