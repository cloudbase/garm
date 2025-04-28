package provider

import (
	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
)

func composeProviderWatcher() dbCommon.PayloadFilterFunc {
	return watcher.WithAny(
		watcher.WithInstanceStatusFilter(
			commonParams.InstancePendingCreate,
			commonParams.InstancePendingDelete,
			commonParams.InstancePendingForceDelete,
		),
		watcher.WithEntityTypeFilter(dbCommon.ScaleSetEntityType),
	)
}
