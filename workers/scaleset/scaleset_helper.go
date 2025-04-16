package scaleset

import (
	"fmt"

	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/github/scalesets"
)

func (w *Worker) ScaleSetCLI() *scalesets.ScaleSetClient {
	return w.scaleSetCli
}

func (w *Worker) GetScaleSet() params.ScaleSet {
	return w.Entity
}

func (w *Worker) Owner() string {
	return fmt.Sprintf("garm-%s", w.controllerInfo.ControllerID)
}

func (w *Worker) SetLastMessageID(id int64) error {
	if err := w.store.SetScaleSetLastMessageID(w.ctx, w.Entity.ID, id); err != nil {
		return fmt.Errorf("setting last message ID: %w", err)
	}
	return nil
}
