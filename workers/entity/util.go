// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package entity

import (
	"strings"

	"golang.org/x/sync/errgroup"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
)

const (
	// These are duplicated until we decide if we move the pool manager to the new
	// worker flow.
	poolIDLabelprefix     = "runner-pool-id:"
	controllerLabelPrefix = "runner-controller-id:"
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
			watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
		),
	)
}

func composeWorkerWatcherFilters(entity params.ForgeEntity) dbCommon.PayloadFilterFunc {
	return watcher.WithAny(
		watcher.WithAll(
			watcher.WithEntityFilter(entity),
			watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
		),
		// Watch for credentials updates.
		watcher.WithAll(
			watcher.WithForgeCredentialsFilter(entity.Credentials),
			watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
		),
	)
}

func (c *Controller) waitForErrorGroupOrContextCancelled(g *errgroup.Group) error {
	if g == nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		waitErr := g.Wait()
		done <- waitErr
	}()

	select {
	case err := <-done:
		return err
	case <-c.ctx.Done():
		return c.ctx.Err()
	case <-c.quit:
		return nil
	}
}

func poolIDFromLabels(runner params.RunnerReference) string {
	for _, lbl := range runner.Labels {
		if strings.HasPrefix(lbl.Name, poolIDLabelprefix) {
			return lbl.Name[len(poolIDLabelprefix):]
		}
	}
	return ""
}
