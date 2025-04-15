package entity

import (
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
)

func composeControllerWatcherFilters() dbCommon.PayloadFilterFunc {
	return watcher.WithAll(
		watcher.WithAny(
			watcher.WithEntityTypeFilter(dbCommon.RepositoryEntityType),
			watcher.WithEntityTypeFilter(dbCommon.OrganizationEntityType),
			watcher.WithEntityTypeFilter(dbCommon.EnterpriseEntityType),
		),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(dbCommon.CreateOperation),
			watcher.WithOperationTypeFilter(dbCommon.DeleteOperation),
		),
	)
}

func composeWorkerWatcherFilters(entity params.GithubEntity) dbCommon.PayloadFilterFunc {
	return watcher.WithAny(
		watcher.WithAll(
			watcher.WithEntityFilter(entity),
			watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
		),
		// Watch for credentials updates.
		watcher.WithAll(
			watcher.WithGithubCredentialsFilter(entity.Credentials),
			watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
		),
	)
}
