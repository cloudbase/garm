package common

import (
	commonParams "github.com/cloudbase/garm-provider-common/params"
)

type ToolsGetter interface {
	GetTools() ([]commonParams.RunnerApplicationDownload, error)
}
