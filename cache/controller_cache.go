package cache

import (
	"sync"

	"github.com/cloudbase/garm/params"
)

var garmControllerCache *ControllerCache

func init() {
	ctrlCache := &ControllerCache{}
	garmControllerCache = ctrlCache
}

type ControllerCache struct {
	controllerInfo params.ControllerInfo

	mux sync.Mutex
}

func (c *ControllerCache) SetControllerCache(ctrl params.ControllerInfo) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.controllerInfo = ctrl
}

func (c *ControllerCache) ControllerInfo() params.ControllerInfo {
	c.mux.Lock()
	defer c.mux.Unlock()

	return c.controllerInfo
}

func ControllerInfo() params.ControllerInfo {
	return garmControllerCache.ControllerInfo()
}

func SetControllerCache(ctrl params.ControllerInfo) {
	garmControllerCache.SetControllerCache(ctrl)
}
