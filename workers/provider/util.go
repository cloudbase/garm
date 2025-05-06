package provider

import (
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
)

func composeProviderWatcher() dbCommon.PayloadFilterFunc {
	return watcher.WithAny(
		watcher.WithEntityTypeFilter(dbCommon.InstanceEntityType),
		watcher.WithEntityTypeFilter(dbCommon.ScaleSetEntityType),
	)
}
