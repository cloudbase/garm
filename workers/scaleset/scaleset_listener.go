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
package scaleset

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/util/github/scalesets"
)

const sessionCleanupTimeout = 10 * time.Second

var closed = make(chan struct{})

func init() { close(closed) }

func newListener(ctx context.Context, scaleSetHelper scaleSetHelper) *scaleSetListener {
	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("sub_worker", "scaleset-listener"),
	)
	return &scaleSetListener{
		ctx:            ctx,
		scaleSetHelper: scaleSetHelper,
		lastMessageID:  scaleSetHelper.GetScaleSet().LastMessageID,
	}
}

type scaleSetListener struct {
	// ctx is the global context for the worker
	ctx context.Context
	// listenerCtx is the context for the listener. We pass this
	// context to GetMessages() which blocks until a message is
	// available. We need to be able to cancel that longpoll request
	// independent of the worker context, in case we need to restart
	// the listener without restarting the worker.
	listenerCtx   context.Context
	cancelFunc    context.CancelFunc
	lastMessageID int64

	scaleSetHelper      scaleSetHelper
	messageSession      *scalesets.MessageSession
	jobAdmission        bool
	jobSessionValidated bool
	pendingJobPollEpoch int64
	pendingJobMessageID int64

	mux        sync.Mutex
	running    atomic.Bool
	quit       chan struct{}
	loopExited chan struct{}
}

func (l *scaleSetListener) Start() error {
	slog.DebugContext(l.ctx, "starting scale set listener", "id", l.scaleSetHelper.GetScaleSet().ID, "scale_set_id", l.scaleSetHelper.GetScaleSet().ScaleSetID)
	l.mux.Lock()
	defer l.mux.Unlock()

	if l.running.Load() {
		return nil
	}

	// Deriving from the worker context keeps the slog values and means the
	// long poll dies with the worker; the cancel func still allows stopping
	// the listener independently, for restarts without a worker restart.
	listenCtx, listenCancelFunc := context.WithCancel(l.ctx)
	listenCtx = garmUtil.WithSlogContext(
		listenCtx,
		slog.Any("session_listener", l.scaleSetHelper.GetScaleSet().ID),
	)
	l.listenerCtx = listenCtx
	l.cancelFunc = listenCancelFunc

	if admission, ok := l.scaleSetHelper.(jobAdmissionHelper); ok {
		jobMode, err := admission.prepareJobSession()
		if err != nil {
			listenCancelFunc()
			return fmt.Errorf("preparing job admission: %w", err)
		}
		l.jobAdmission = jobMode
	}
	scaleSet := l.scaleSetHelper.GetScaleSet()
	scaleSetClient, err := l.scaleSetHelper.GetScaleSetClient()
	if err != nil {
		return fmt.Errorf("getting scale set client: %w", err)
	}
	slog.DebugContext(l.ctx, "creating new message session", "scale_set", scaleSet.ScaleSetID)
	session, err := scaleSetClient.CreateMessageSession(
		l.listenerCtx, scaleSet.ScaleSetID,
		l.scaleSetHelper.Owner(),
	)
	if err != nil {
		return fmt.Errorf("creating message session: %w", err)
	}
	l.messageSession = session
	l.pendingJobPollEpoch, l.pendingJobMessageID = 0, 0
	l.jobSessionValidated = false
	if l.jobAdmission {
		stats, present := session.InitialStatistics()
		if err := l.scaleSetHelper.(jobAdmissionHelper).validateJobSession(stats, present); err != nil {
			slog.ErrorContext(l.ctx, "initial job session requires reconciliation", "error", err)
		} else {
			l.jobSessionValidated = true
		}
	}
	l.quit = make(chan struct{})
	l.running.Store(true)
	l.loopExited = make(chan struct{})
	go l.loop()

	return nil
}

func (l *scaleSetListener) Stop() error {
	l.mux.Lock()
	defer l.mux.Unlock()

	if !l.running.Load() {
		return nil
	}
	// Stop the long poll and token refresh before attempting remote cleanup.
	// In particular, Close can otherwise wait for an in-flight refresh.
	l.cancelFunc()
	if l.messageSession != nil {
		slog.DebugContext(l.ctx, "closing message session", "scale_set", l.scaleSetHelper.GetScaleSet().ScaleSetID)
		if err := l.messageSession.Close(); err != nil {
			slog.ErrorContext(l.ctx, "closing message session", "error", err)
		}
		scaleSetClient, err := l.scaleSetHelper.GetScaleSetClient()
		if err != nil {
			slog.ErrorContext(l.ctx, "error getting scale set client", "error", err)
		} else {
			// The worker context is cancelled during shutdown, but deleting the
			// remote session still needs a bounded opportunity to finish.
			cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(l.ctx), sessionCleanupTimeout)
			defer cleanupCancel()
			if err := scaleSetClient.DeleteMessageSession(cleanupCtx, l.messageSession); err != nil {
				slog.ErrorContext(l.ctx, "error deleting message session", "error", err)
			}
		}
	}

	l.running.Store(false)
	close(l.quit)
	return nil
}

func (l *scaleSetListener) IsRunning() bool {
	l.mux.Lock()
	defer l.mux.Unlock()
	return l.running.Load()
}

// scaleSetIDLabel returns the database ID of the scale set as a string, for
// use as the "id" metric label. It matches the "id" label on the other
// garm_scaleset_* metrics.
func (l *scaleSetListener) scaleSetIDLabel() string {
	return strconv.FormatUint(uint64(l.scaleSetHelper.GetScaleSet().ID), 10)
}

func (l *scaleSetListener) handleSessionMessage(msg params.RunnerScaleSetMessage) {
	l.mux.Lock()
	defer l.mux.Unlock()

	if params.ScaleSetMessageType(msg.MessageType) != params.MessageTypeRunnerScaleSetJobMessages {
		slog.DebugContext(l.ctx, "message is not a job message, ignoring")
		return
	}

	body, err := msg.GetJobsFromBody()
	if err != nil {
		slog.ErrorContext(l.ctx, "getting jobs from body", "error", err)
	}

	var completedJobs []params.ScaleSetJobMessage
	var availableJobs []params.ScaleSetJobMessage
	var startedJobs []params.ScaleSetJobMessage
	var assignedJobs []params.ScaleSetJobMessage

	scaleSetID := l.scaleSetIDLabel()
	for _, job := range body {
		metrics.ScaleSetMessagesCount.WithLabelValues(
			scaleSetID,      // label: id
			job.MessageType, // label: message_type
		).Inc()
		switch job.MessageType {
		case params.MessageTypeJobAssigned:
			slog.InfoContext(l.ctx, "new job assigned", "job_id", job.JobID, "job_name", job.JobDisplayName)
			assignedJobs = append(assignedJobs, job)
		case params.MessageTypeJobStarted:
			slog.InfoContext(l.ctx, "job started", "job_id", job.JobID, "job_name", job.JobDisplayName, "runner_name", job.RunnerName)
			startedJobs = append(startedJobs, job)
		case params.MessageTypeJobCompleted:
			slog.InfoContext(l.ctx, "job completed", "job_id", job.JobID, "job_name", job.JobDisplayName, "runner_name", job.RunnerName)
			completedJobs = append(completedJobs, job)
		case params.MessageTypeJobAvailable:
			slog.InfoContext(l.ctx, "job available", "job_id", job.JobID, "job_name", job.JobDisplayName)
			availableJobs = append(availableJobs, job)
		default:
			slog.DebugContext(l.ctx, "unknown message type", "message_type", job.MessageType)
		}
	}

	scaleSetClient, err := l.scaleSetHelper.GetScaleSetClient()
	if err != nil {
		slog.ErrorContext(l.ctx, "getting scale set client", "error", err)
		return
	}
	acquisitionFailed := false
	if len(availableJobs) > 0 {
		jobIDs := make([]int64, len(availableJobs))
		for idx, job := range availableJobs {
			jobIDs[idx] = job.RunnerRequestID
		}
		idsAcquired, err := scaleSetClient.AcquireJobs(
			l.listenerCtx, l.scaleSetHelper.GetScaleSet().ScaleSetID,
			l.messageSession.MessageQueueAccessToken(), jobIDs)
		if err != nil {
			slog.ErrorContext(l.ctx, "acquiring jobs", "error", err)
			acquisitionFailed = true
		} else {
			// HandleJobsAvailable only records jobs in the database for now. The jobs are purely
			// informational, so an error here won't break anything.
			if err := l.scaleSetHelper.HandleJobsAvailable(availableJobs); err != nil {
				slog.ErrorContext(l.ctx, "error handling available jobs", "error", err)
			}
			slog.DebugContext(l.ctx, "acquired jobs", "job_ids", idsAcquired)
		}
	}

	if len(assignedJobs) > 0 {
		if err := l.scaleSetHelper.HandleJobsAvailable(assignedJobs); err != nil {
			slog.ErrorContext(l.ctx, "error handling available jobs", "error", err)
		}
	}

	if len(startedJobs) > 0 {
		if err := l.scaleSetHelper.HandleJobsStarted(startedJobs); err != nil {
			slog.ErrorContext(l.ctx, "error handling started jobs", "error", err)
			return
		}
	}

	if len(completedJobs) > 0 {
		if err := l.scaleSetHelper.HandleJobsCompleted(completedJobs); err != nil {
			slog.ErrorContext(l.ctx, "error handling completed jobs", "error", err)
			return
		}
	}

	if acquisitionFailed {
		// Leave the message unacknowledged so acquisition can be retried.
		return
	}

	if err := l.scaleSetHelper.SetLastMessageID(msg.MessageID); err != nil {
		slog.ErrorContext(l.ctx, "setting last message ID", "error", err)
	} else {
		l.lastMessageID = msg.MessageID
	}

	if err := l.scaleSetHelper.SetDesiredRunnerCount(msg.Statistics.TotalAssignedJobs); err != nil {
		slog.ErrorContext(l.ctx, "setting desired runner count", "error", err)
	}

	if err := l.messageSession.DeleteMessage(l.listenerCtx, msg.MessageID); err != nil {
		slog.ErrorContext(l.ctx, "deleting message", "error", err)
	}
}

func (l *scaleSetListener) loop() {
	defer close(l.loopExited)
	defer l.Stop()
	retryAfterUnauthorized := false

	slog.DebugContext(l.ctx, "starting scale set listener loop", "scale_set", l.scaleSetHelper.GetScaleSet().ScaleSetID)
	for {
		select {
		case <-l.quit:
			return
		case <-l.listenerCtx.Done():
			slog.DebugContext(l.ctx, "stopping scale set listener")
			return
		case <-l.ctx.Done():
			slog.DebugContext(l.ctx, "scaleset worker has stopped")
			return
		default:
			slog.DebugContext(l.ctx, "getting message", "last_message_id", l.lastMessageID, "max_runners", l.scaleSetHelper.GetScaleSet().MaxRunners)
			// Each poll attempt is bounded by the long poll client's own
			// timeout, sized for the broker's ~50s hold; the context only
			// needs to carry cancellation.
			capacity := l.scaleSetHelper.GetScaleSet().MaxRunners
			var epoch int64
			if l.jobAdmission {
				capacity = 0
				if l.jobSessionValidated {
					var authErr error
					capacity, epoch, authErr = l.scaleSetHelper.(jobAdmissionHelper).authorizeJobPoll()
					if authErr != nil {
						slog.WarnContext(l.ctx, "job admission closed", "error", authErr)
						capacity = 0
					}
				}
			}
			var msg params.RunnerScaleSetMessage
			var err error
			if l.jobAdmission {
				msg, err = l.messageSession.GetAdmissionMessage(l.listenerCtx, l.lastMessageID, capacity)
			} else {
				msg, err = l.messageSession.GetMessage(l.listenerCtx, l.lastMessageID, capacity)
			}
			if err != nil {
				if errors.Is(err, runnerErrors.ErrUnauthorized) {
					if retryAfterUnauthorized {
						slog.DebugContext(l.ctx, "unauthorized, stopping listener")
						return
					}
					// The session manager refreshes the token automatically, but once we call
					// GetMessage(), it blocks until a new message is sent on the longpoll.
					// If there are no messages for a while, the token used to longpoll expires
					// and we get an unauthorized error. We simply need to retry the request
					// and it should use the refreshed token. If we fail a second time, we can
					// return and the scaleset worker will attempt to restart the listener.
					retryAfterUnauthorized = true
					slog.DebugContext(l.ctx, "got unauthorized error, retrying")
					continue
				}
				if !errors.Is(err, context.Canceled) {
					slog.ErrorContext(l.ctx, "getting message", "error", err)
				}
				slog.DebugContext(l.ctx, "stopping scale set listener")
				return
			}
			retryAfterUnauthorized = false
			l.processPolledMessage(msg, epoch)
		}
	}
}

func (l *scaleSetListener) processPolledMessage(msg params.RunnerScaleSetMessage, epoch int64) {
	if l.jobAdmission && epoch != 0 && !msg.IsNil() {
		l.pendingJobPollEpoch, l.pendingJobMessageID = epoch, msg.MessageID
	}
	// Empty polls also prove the listener is alive; the broker holds
	// the long poll for ~50s before returning a nil message.
	metrics.ScaleSetListenerLastSuccess.WithLabelValues(l.scaleSetIDLabel()).SetToCurrentTime()
	if !msg.IsNil() {
		slog.DebugContext(l.ctx, "handling message", "message_id", msg.MessageID)
		if l.jobAdmission {
			client, err := l.scaleSetHelper.GetScaleSetClient()
			if err != nil {
				slog.ErrorContext(l.ctx, "getting scale set client", "error", err)
				return
			}
			if err := l.scaleSetHelper.(jobAdmissionHelper).handleJobAdmissionMessage(msg, client, l.messageSession.MessageQueueAccessToken()); err != nil {
				slog.ErrorContext(l.ctx, "handling admitted jobs", "error", err)
				return
			}
		} else {
			l.handleSessionMessage(msg)
		}
	}
	if l.jobAdmission && epoch == 0 && !msg.IsNil() && msg.MessageID == l.pendingJobMessageID {
		epoch = l.pendingJobPollEpoch
	}
	if l.jobAdmission && epoch != 0 {
		if err := l.scaleSetHelper.(jobAdmissionHelper).finishJobPoll(epoch); err != nil {
			slog.ErrorContext(l.ctx, "finishing admission poll", "error", err)
			return
		}
		if epoch == l.pendingJobPollEpoch {
			l.pendingJobPollEpoch, l.pendingJobMessageID = 0, 0
		}
	}
	if l.jobAdmission && !msg.IsNil() {
		if !l.jobSessionValidated && msg.Statistics != nil {
			if err := l.scaleSetHelper.(jobAdmissionHelper).validateJobSession(*msg.Statistics, true); err == nil {
				l.jobSessionValidated = true
			}
		}
		if err := l.scaleSetHelper.SetLastMessageID(msg.MessageID); err != nil {
			slog.ErrorContext(l.ctx, "persisting admitted message", "error", err)
			return
		}
		l.lastMessageID = msg.MessageID
		if err := l.messageSession.DeleteMessage(l.listenerCtx, msg.MessageID); err != nil {
			slog.ErrorContext(l.ctx, "deleting admitted message", "error", err)
		}
	}
}

func (l *scaleSetListener) Wait() <-chan struct{} {
	l.mux.Lock()
	defer l.mux.Unlock()

	if !l.running.Load() {
		slog.DebugContext(l.ctx, "scale set listener is not running")
		return closed
	}
	return l.loopExited
}
