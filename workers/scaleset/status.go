package scaleset

import (
	"time"

	"github.com/cloudbase/garm/params"
)

type scaleSetStatus struct {
	err       error
	heartbeat time.Time
	scaleSet  params.ScaleSet
}
