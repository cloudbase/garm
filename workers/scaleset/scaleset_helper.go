package scaleset

import (
	"errors"
	"fmt"
	"log/slog"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/locking"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/github/scalesets"
)

func (w *Worker) GetScaleSetClient() (*scalesets.ScaleSetClient, error) {
	scaleSetEntity, err := w.scaleSet.GetEntity()
	if err != nil {
		return nil, fmt.Errorf("getting entity: %w", err)
	}

	ghCli, ok := cache.GetGithubClient(scaleSetEntity.ID)
	if !ok {
		return nil, fmt.Errorf("getting github client: %w", err)
	}
	scaleSetClient, err := scalesets.NewClient(ghCli)
	if err != nil {
		return nil, fmt.Errorf("creating scale set client: %w", err)
	}

	return scaleSetClient, nil
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

func (w *Worker) recordOrUpdateJob(job params.ScaleSetJobMessage) error {
	entity, err := w.scaleSet.GetEntity()
	if err != nil {
		return fmt.Errorf("getting entity: %w", err)
	}
	asUUID, err := entity.GetIDAsUUID()
	if err != nil {
		return fmt.Errorf("getting entity ID as UUID: %w", err)
	}

	jobParams := job.ToJob()
	jobParams.RunnerGroupName = w.scaleSet.GitHubRunnerGroup

	switch entity.EntityType {
	case params.GithubEntityTypeEnterprise:
		jobParams.EnterpriseID = &asUUID
	case params.GithubEntityTypeRepository:
		jobParams.RepoID = &asUUID
	case params.GithubEntityTypeOrganization:
		jobParams.OrgID = &asUUID
	default:
		return fmt.Errorf("unknown entity type: %s", entity.EntityType)
	}

	if _, jobErr := w.store.CreateOrUpdateJob(w.ctx, jobParams); jobErr != nil {
		slog.With(slog.Any("error", jobErr)).ErrorContext(
			w.ctx, "failed to update job", "job_id", jobParams.ID)
	}
	return nil
}

// HandleJobCompleted handles a job completed message. If a job had a runner
// assigned and was not canceled before it had a chance to run, then we mark
// that runner as pending_delete.
func (w *Worker) HandleJobsCompleted(jobs []params.ScaleSetJobMessage) (err error) {
	slog.DebugContext(w.ctx, "handling job completed", "jobs", jobs)
	defer slog.DebugContext(w.ctx, "finished handling job completed", "jobs", jobs, "error", err)

	for _, job := range jobs {
		if err := w.recordOrUpdateJob(job); err != nil {
			// recording scale set jobs are purely informational for now.
			slog.ErrorContext(w.ctx, "failed to save job data", "job", job, "error", err)
		}

		if job.RunnerName == "" {
			// This job was not assigned to a runner, so we can skip it.
			continue
		}
		// Set the runner to pending_delete.
		runnerUpdateParams := params.UpdateInstanceParams{
			Status:       commonParams.InstancePendingDelete,
			RunnerStatus: params.RunnerTerminated,
		}

		locking.Lock(job.RunnerName, w.consumerID)
		_, err := w.store.UpdateInstance(w.ctx, job.RunnerName, runnerUpdateParams)
		if err != nil {
			if !errors.Is(err, runnerErrors.ErrNotFound) {
				locking.Unlock(job.RunnerName, false)
				return fmt.Errorf("updating runner %s: %w", job.RunnerName, err)
			}
		}
		locking.Unlock(job.RunnerName, false)
	}
	return nil
}

// HandleJobStarted updates the runners from idle to active in the DB and
// assigns the job to them.
func (w *Worker) HandleJobsStarted(jobs []params.ScaleSetJobMessage) (err error) {
	slog.DebugContext(w.ctx, "handling job started", "jobs", jobs)
	defer slog.DebugContext(w.ctx, "finished handling job started", "jobs", jobs, "error", err)
	for _, job := range jobs {
		if err := w.recordOrUpdateJob(job); err != nil {
			// recording scale set jobs are purely informational for now.
			slog.ErrorContext(w.ctx, "failed to save job data", "job", job, "error", err)
		}

		if job.RunnerName == "" {
			// This should not happen, but just in case.
			continue
		}

		updateParams := params.UpdateInstanceParams{
			RunnerStatus: params.RunnerActive,
		}

		locking.Lock(job.RunnerName, w.consumerID)
		_, err := w.store.UpdateInstance(w.ctx, job.RunnerName, updateParams)
		if err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				slog.InfoContext(w.ctx, "runner not found; handled by some other controller?", "runner_name", job.RunnerName)
				locking.Unlock(job.RunnerName, true)
				continue
			}
			locking.Unlock(job.RunnerName, false)
			return fmt.Errorf("updating runner %s: %w", job.RunnerName, err)
		}
		locking.Unlock(job.RunnerName, false)
	}
	return nil
}

func (w *Worker) HandleJobsAvailable(jobs []params.ScaleSetJobMessage) error {
	for _, job := range jobs {
		if err := w.recordOrUpdateJob(job); err != nil {
			// recording scale set jobs are purely informational for now.
			slog.ErrorContext(w.ctx, "failed to save job data", "job", job, "error", err)
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
