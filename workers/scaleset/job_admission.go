// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

package scaleset

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/github/scalesets"
)

const (
	jobAdmissionProtocol = "job-capacity-experimental"
	jobStateAcquired     = "acquired"
)

func (w *Worker) jobAdmissionPolicy() (dbCommon.AdmissionStore, params.AdmissionConfig, bool, error) {
	store, ok := w.store.(dbCommon.AdmissionStore)
	if !ok {
		return nil, params.AdmissionConfig{}, false, nil
	}
	set, err := w.currentJobScaleSet()
	if err != nil {
		return nil, params.AdmissionConfig{}, false, err
	}
	policy, enrolled, err := store.AdmissionForProvider(w.ctx, set.ProviderName)
	if err != nil {
		return nil, policy, false, err
	}
	return store, policy, enrolled && policy.Protocol == jobAdmissionProtocol, nil
}
func (w *Worker) jobOwner() string { return fmt.Sprintf("scaleset:%d", w.jobScaleSetID) }

func (w *Worker) currentJobScaleSet() (params.ScaleSet, error) {
	if w.jobScaleSetID == 0 {
		return w.scaleSet, nil
	}
	return w.store.GetScaleSetByID(w.ctx, w.jobScaleSetID)
}

// prepareJobSession establishes durable capacity before session creation.
func (w *Worker) prepareJobSession() (bool, error) {
	store, policy, enabled, err := w.jobAdmissionPolicy()
	if err != nil || !enabled {
		return enabled, err
	}
	set, err := w.currentJobScaleSet()
	if err != nil {
		return true, err
	}
	if set.MinIdleRunners != 0 {
		return true, fmt.Errorf("job admission requires zero min idle")
	}
	_, _, err = store.GetAdmissionCredit(w.ctx, policy.Domain, w.jobOwner())
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, dbCommon.ErrAdmissionNotFound) {
		return true, err
	}
	if !set.Enabled {
		return true, fmt.Errorf("disabled scale set has no admission credit")
	}
	if err := w.checkAdmissionReadinessFor(store, policy, set); err != nil {
		return true, err
	}
	if policy.Drained && policy.RequestedActivationGeneration == policy.Generation && policy.RequestedActivationGeneration > policy.LastResumeGeneration {
		if err := store.ResumeAdmission(w.ctx, policy.Domain, policy.Generation); err != nil {
			return true, err
		}
	}
	_, err = store.EnsureAdmissionCredit(w.ctx, policy.Domain, w.jobOwner(), policy.Class, policy.Generation)
	return true, err
}

// A new session must not advertise positive capacity if broker statistics
// contain work that cannot be matched to durable local commitments.
func (w *Worker) validateJobSession(stats params.RunnerScaleSetStatistic, present bool) error {
	if !present {
		return fmt.Errorf("initial session statistics are unavailable")
	}
	store, policy, enabled, err := w.jobAdmissionPolicy()
	if err != nil || !enabled {
		return fmt.Errorf("job admission policy unavailable: %w", err)
	}
	_, jobs, err := store.GetAdmissionCredit(w.ctx, policy.Domain, w.jobOwner())
	if err != nil {
		return err
	}
	known, instances := 0, 0
	for _, job := range jobs {
		if !job.Terminal && (job.State == jobStateAcquired || job.State == "acquire_sending") {
			known++
		}
		if job.InstanceName != "" && !job.Absent {
			instances++
		}
	}
	if stats.TotalAcquiredJobs < 0 || stats.TotalAssignedJobs < 0 || stats.TotalRunningJobs < 0 || stats.TotalRegisteredRunners < 0 || stats.TotalBusyRunners < 0 || stats.TotalIdleRunners < 0 || stats.TotalAvailableJobs < 0 {
		return fmt.Errorf("invalid negative session statistics")
	}
	if stats.TotalAcquiredJobs > 1 || stats.TotalAssignedJobs > 1 || stats.TotalRunningJobs > 1 || stats.TotalAcquiredJobs > known || stats.TotalAssignedJobs > known || stats.TotalRunningJobs > known || stats.TotalRegisteredRunners > instances || stats.TotalBusyRunners > instances || stats.TotalIdleRunners > instances {
		return fmt.Errorf("session contains unaccounted jobs or runners")
	}
	return nil
}

func (w *Worker) authorizeJobPoll() (uint, int64, error) {
	store, policy, enabled, err := w.jobAdmissionPolicy()
	if err != nil || !enabled {
		return 0, 0, fmt.Errorf("job admission policy unavailable: %w", err)
	}
	set, err := w.currentJobScaleSet()
	if err != nil {
		return 0, 0, err
	}
	if !set.Enabled || set.MaxRunners == 0 {
		return 0, 0, nil
	}
	if policy.Drained && (policy.RequestedActivationGeneration != policy.Generation || policy.RequestedActivationGeneration <= policy.LastResumeGeneration) {
		return 0, 0, nil
	}
	if err := w.checkAdmissionReadinessFor(store, policy, set); err != nil {
		return 0, 0, err
	}
	if policy.Drained {
		if err := store.ResumeAdmission(w.ctx, policy.Domain, policy.Generation); err != nil {
			return 0, 0, err
		}
	}
	epoch, err := store.AuthorizeAdmissionPoll(w.ctx, policy.Domain, w.jobOwner(), policy.Generation)
	if err != nil {
		return 0, 0, err
	}
	return 1, epoch, nil
}

func (w *Worker) finishJobPoll(epoch int64) error {
	store, policy, enabled, err := w.jobAdmissionPolicy()
	if err != nil || !enabled {
		return fmt.Errorf("job admission policy unavailable: %w", err)
	}
	return store.FinishAdmissionPoll(w.ctx, policy.Domain, w.jobOwner(), epoch)
}

func admissionJobKey(job params.ScaleSetJobMessage) (string, string, error) {
	if job.RunnerRequestID <= 0 || job.JobID == "" {
		return "", "", fmt.Errorf("%w: job event lacks request or job identity", dbCommon.ErrAdmissionConflict)
	}
	return strconv.FormatInt(job.RunnerRequestID, 10), job.JobID, nil
}

func (w *Worker) handleJobAdmissionMessage(msg params.RunnerScaleSetMessage, client *scalesets.ScaleSetClient, token string) error {
	w.mux.Lock()
	defer w.mux.Unlock()
	store, policy, enabled, err := w.jobAdmissionPolicy()
	if err != nil || !enabled {
		return fmt.Errorf("job admission policy unavailable: %w", err)
	}
	set, err := w.currentJobScaleSet()
	if err != nil {
		return err
	}
	jobs, err := msg.GetJobsFromBody()
	if err != nil {
		return fmt.Errorf("decoding job message: %w", err)
	}
	if err := w.processAdmittedLifecycle(store, policy, jobs); err != nil {
		return err
	}
	if err := w.acquireAdmittedOffers(store, policy, set, jobs, client, token); err != nil {
		return err
	}
	return w.finalizeAdmittedMessage(store, policy, msg)
}

func (w *Worker) processAdmittedLifecycle(store dbCommon.AdmissionStore, policy params.AdmissionConfig, jobs []params.ScaleSetJobMessage) error {
	completed, lifecycleErr := w.prepareAdmittedLifecycle(store, policy, jobs)
	for _, job := range jobs {
		if job.MessageType != params.MessageTypeJobStarted && job.MessageType != params.MessageTypeJobCompleted {
			continue
		}
		request, id, err := admissionJobKey(job)
		if err == nil && job.MessageType == params.MessageTypeJobCompleted && completed[request+"/"+id] {
			continue
		}
		if err == nil {
			if job.MessageType == params.MessageTypeJobCompleted {
				err = store.CompleteAdmissionJob(w.ctx, policy.Domain, w.jobOwner(), request, id)
			} else {
				err = store.MarkAdmissionAcquired(w.ctx, policy.Domain, w.jobOwner(), request, id)
			}
		}
		if err != nil {
			lifecycleErr = errors.Join(lifecycleErr, err)
			continue
		}
		if job.MessageType == params.MessageTypeJobCompleted {
			err = w.HandleJobsCompleted([]params.ScaleSetJobMessage{job})
		} else {
			err = w.HandleJobsStarted([]params.ScaleSetJobMessage{job})
		}
		if err != nil {
			lifecycleErr = errors.Join(lifecycleErr, err)
		}
	}
	if lifecycleErr != nil {
		if errors.Is(lifecycleErr, dbCommon.ErrAdmissionConflict) || errors.Is(lifecycleErr, dbCommon.ErrAdmissionNotFound) {
			if err := store.SetAdmissionDrain(w.ctx, policy.Domain, true); err != nil {
				return errors.Join(lifecycleErr, err)
			}
		}
		return lifecycleErr
	}
	return nil
}

func (w *Worker) prepareAdmittedLifecycle(store dbCommon.AdmissionStore, policy params.AdmissionConfig, jobs []params.ScaleSetJobMessage) (map[string]bool, error) {
	var lifecycleErr error
	_, existing, err := store.GetAdmissionCredit(w.ctx, policy.Domain, w.jobOwner())
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool)
	for _, job := range existing {
		known[job.RequestID+"/"+job.JobID] = true
	}
	completed := make(map[string]bool)
	// Finish already-known jobs first so the next assignment can be queued
	// while the previous runner is still being deleted.
	for _, job := range jobs {
		if job.MessageType != params.MessageTypeJobCompleted {
			continue
		}
		request, id, keyErr := admissionJobKey(job)
		if keyErr != nil || !known[request+"/"+id] {
			continue
		}
		if err := store.CompleteAdmissionJob(w.ctx, policy.Domain, w.jobOwner(), request, id); err != nil {
			lifecycleErr = errors.Join(lifecycleErr, err)
			continue
		}
		if err := w.HandleJobsCompleted([]params.ScaleSetJobMessage{job}); err != nil {
			lifecycleErr = errors.Join(lifecycleErr, err)
			continue
		}
		completed[request+"/"+id] = true
	}
	// Persist assigned events before their lifecycle events in this message.
	for _, job := range jobs {
		if job.MessageType != params.MessageTypeJobAssigned {
			continue
		}
		request, id, err := admissionJobKey(job)
		if err == nil {
			_, err = store.RecordAdmissionAssignment(w.ctx, policy.Domain, w.jobOwner(), request, id)
		}
		if err == nil && job.MessageType == params.MessageTypeJobAssigned {
			err = store.MarkAdmissionAcquired(w.ctx, policy.Domain, w.jobOwner(), request, id)
		}
		if err != nil {
			lifecycleErr = errors.Join(lifecycleErr, err)
		}
	}
	return completed, lifecycleErr
}

func (w *Worker) acquireAdmittedOffers(store dbCommon.AdmissionStore, policy params.AdmissionConfig, set params.ScaleSet, jobs []params.ScaleSetJobMessage, client *scalesets.ScaleSetClient, token string) error {
	// Acquire at most one eligible offer. Other available messages are offers,
	// not commitments, and must not be counted as assigned work.
	_, current, err := store.GetAdmissionCredit(w.ctx, policy.Domain, w.jobOwner())
	if err != nil {
		return err
	}
	busy := false
	for _, job := range current {
		if !job.Terminal && (job.State == jobStateAcquired || job.State == "acquire_sending") {
			busy = true
		}
	}
	for _, job := range jobs {
		if job.MessageType != params.MessageTypeJobAvailable {
			continue
		}
		request, id, err := admissionJobKey(job)
		if err != nil {
			return err
		}
		already := false
		for _, existing := range current {
			if existing.RequestID == request && existing.JobID == id && (existing.State == jobStateAcquired || existing.Terminal) {
				already = true
				break
			}
		}
		if already {
			continue
		}
		if busy || !set.Enabled || set.MaxRunners == 0 || policy.Drained || !policy.Ready || !policy.HealthyUntil.After(time.Now()) {
			continue
		}
		recorded, err := store.RecordAdmissionAssignment(w.ctx, policy.Domain, w.jobOwner(), request, id)
		if err != nil {
			return err
		}
		if recorded.Terminal || recorded.State == jobStateAcquired {
			continue
		}
		send, err := store.BeginAdmissionAcquire(w.ctx, policy.Domain, w.jobOwner(), request, id)
		if err != nil {
			return err
		}
		if !send {
			return fmt.Errorf("acquire outcome is unresolved")
		}
		acquired, err := client.AcquireJobs(w.ctx, set.ScaleSetID, token, []int64{job.RunnerRequestID})
		if err != nil {
			return fmt.Errorf("acquire outcome is uncertain: %w", err)
		}
		if len(acquired) != 1 || acquired[0] != job.RunnerRequestID {
			return fmt.Errorf("acquire outcome is incomplete")
		}
		if err := store.MarkAdmissionAcquired(w.ctx, policy.Domain, w.jobOwner(), request, id); err != nil {
			return err
		}
		busy = true
	}
	return nil
}

func (w *Worker) finalizeAdmittedMessage(store dbCommon.AdmissionStore, policy params.AdmissionConfig, msg params.RunnerScaleSetMessage) error {
	if msg.Statistics == nil {
		return fmt.Errorf("job message has no statistics")
	}
	if err := w.validateJobSession(*msg.Statistics, true); err != nil {
		if drainErr := store.SetAdmissionDrain(w.ctx, policy.Domain, true); drainErr != nil {
			return errors.Join(err, drainErr)
		}
		return err
	}
	_, knownJobs, err := store.GetAdmissionCredit(w.ctx, policy.Domain, w.jobOwner())
	if err != nil {
		return err
	}
	desired := 0
	for _, job := range knownJobs {
		if !job.Terminal && job.State == jobStateAcquired {
			desired = 1
			break
		}
	}
	if err := w.SetDesiredRunnerCount(desired); err != nil {
		return err
	}
	return nil
}
