package scaleset

import (
	"strings"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
)

func composeControllerWatcherFilters(entity params.GithubEntity) dbCommon.PayloadFilterFunc {
	return watcher.WithAny(
		watcher.WithAll(
			watcher.WithEntityScaleSetFilter(entity),
			watcher.WithAny(
				watcher.WithOperationTypeFilter(dbCommon.CreateOperation),
				watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
				watcher.WithOperationTypeFilter(dbCommon.DeleteOperation),
			),
		),
		watcher.WithAll(
			watcher.WithEntityFilter(entity),
			watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
		),
		watcher.WithAll(
			watcher.WithGithubCredentialsFilter(entity.Credentials),
			watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
		),
	)
}

func poolIDFromLabels(runner params.RunnerReference) string {
	for _, lbl := range runner.Labels {
		if strings.HasPrefix(lbl.Name, poolIDLabelprefix) {
			return lbl.Name[len(poolIDLabelprefix):]
		}
	}
	return ""
}
