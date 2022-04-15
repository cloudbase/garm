package params

type InstanceRequest struct {
	Name      string `json:"name"`
	OSType    OSType `json:"os_type"`
	OSVersion string `json:"os_version"`
}
