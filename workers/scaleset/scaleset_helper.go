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
	return w.scaleSet
}

func (w *Worker) Owner() string {
	return fmt.Sprintf("garm-%s", w.controllerInfo.ControllerID)
}

func (w *Worker) SetLastMessageID(id int64) error {
	if err := w.store.SetScaleSetLastMessageID(w.ctx, w.scaleSet.ID, id); err != nil {
		return fmt.Errorf("setting last message ID: %w", err)
	}
	return nil
}

// HandleJobCompleted handles a job completed message. If a job had a runner
// assigned and was not canceled before it had a chance to run, then we mark
// that runner as pending_delete.
func (w *Worker) HandleJobsCompleted(jobs []params.ScaleSetJobMessage) error {
	return nil
}

// HandleJobStarted updates the runners from idle to active in the DB and
// assigns the job to them.
func (w *Worker) HandleJobsStarted(jobs []params.ScaleSetJobMessage) error {
	return nil
}
