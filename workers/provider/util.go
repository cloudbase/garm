package provider

import (
	"golang.org/x/sync/errgroup"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
)

func composeProviderWatcher() dbCommon.PayloadFilterFunc {
	return watcher.WithAny(
		watcher.WithEntityTypeFilter(dbCommon.InstanceEntityType),
		watcher.WithEntityTypeFilter(dbCommon.ScaleSetEntityType),
	)
}

func (p *Provider) waitForErrorGroupOrContextCancelled(g *errgroup.Group) error {
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
	case <-p.ctx.Done():
		return p.ctx.Err()
	case <-p.quit:
		return nil
	}
}
