package scaleset

import (
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/github/scalesets"
)

type scaleSetHelper interface {
	ScaleSetCLI() *scalesets.ScaleSetClient
	GetScaleSet() params.ScaleSet
	Owner() string
}
