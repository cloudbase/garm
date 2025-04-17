package scaleset

import (
	"errors"
	"fmt"
	"log/slog"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"

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
	for _, job := range jobs {
		if job.RunnerName == "" {
			// This job was not assigned to a runner, so we can skip it.
			continue
		}
		// Set the runner to pending_delete.
		runnerUpdateParams := params.UpdateInstanceParams{
			Status:       commonParams.InstancePendingDelete,
			RunnerStatus: params.RunnerTerminated,
		}
		_, err := w.store.UpdateInstance(w.ctx, job.RunnerName, runnerUpdateParams)
		if err != nil {
			if !errors.Is(err, runnerErrors.ErrNotFound) {
				return fmt.Errorf("updating runner %s: %w", job.RunnerName, err)
			}
		}
	}
	return nil
}

// HandleJobStarted updates the runners from idle to active in the DB and
// assigns the job to them.
func (w *Worker) HandleJobsStarted(jobs []params.ScaleSetJobMessage) error {
	for _, job := range jobs {
		if job.RunnerName == "" {
			// This should not happen, but just in case.
			continue
		}

		updateParams := params.UpdateInstanceParams{
			RunnerStatus: params.RunnerActive,
		}

		_, err := w.store.UpdateInstance(w.ctx, job.RunnerName, updateParams)
		if err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				slog.InfoContext(w.ctx, "runner not found; handled by some other controller?", "runner_name", job.RunnerName)
				continue
			}
			return fmt.Errorf("updating runner %s: %w", job.RunnerName, err)
		}
	}
	return nil
}

func (w *Worker) SetDesiredRunnerCount(count int) error {
	if err := w.store.SetScaleSetDesiredRunnerCount(w.ctx, w.scaleSet.ID, count); err != nil {
		return fmt.Errorf("setting desired runner count: %w", err)
	}
	return nil
}
