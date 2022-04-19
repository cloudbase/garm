package params

import "runner-manager/config"

type InstanceRequest struct {
	Name      string        `json:"name"`
	OSType    config.OSType `json:"os_type"`
	OSVersion string        `json:"os_version"`
}
