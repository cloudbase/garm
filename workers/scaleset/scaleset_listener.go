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
	"sync"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/github/scalesets"
)

var closed = make(chan struct{})

func init() { close(closed) }

func newListener(ctx context.Context, scaleSetHelper scaleSetHelper) *scaleSetListener {
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

	scaleSetHelper scaleSetHelper
	messageSession *scalesets.MessageSession

	mux        sync.Mutex
	running    bool
	quit       chan struct{}
	loopExited chan struct{}
}

func (l *scaleSetListener) Start() error {
	slog.DebugContext(l.ctx, "starting scale set listener", "scale_set", l.scaleSetHelper.GetScaleSet().ScaleSetID)
	l.mux.Lock()
	defer l.mux.Unlock()

	if l.running {
		return nil
	}

	l.listenerCtx, l.cancelFunc = context.WithCancel(context.Background())
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
	l.quit = make(chan struct{})
	l.running = true
	l.loopExited = make(chan struct{})
	go l.loop()

	return nil
}

func (l *scaleSetListener) Stop() error {
	l.mux.Lock()
	defer l.mux.Unlock()

	if !l.running {
		return nil
	}
	scaleSetClient, err := l.scaleSetHelper.GetScaleSetClient()
	if err != nil {
		return fmt.Errorf("getting scale set client: %w", err)
	}
	if l.messageSession != nil {
		slog.DebugContext(l.ctx, "closing message session", "scale_set", l.scaleSetHelper.GetScaleSet().ScaleSetID)
		if err := l.messageSession.Close(); err != nil {
			slog.ErrorContext(l.ctx, "closing message session", "error", err)
		}
		if err := scaleSetClient.DeleteMessageSession(context.Background(), l.messageSession); err != nil {
			slog.ErrorContext(l.ctx, "error deleting message session", "error", err)
		}
	}

	l.running = false
	close(l.quit)
	l.cancelFunc()
	return nil
}

func (l *scaleSetListener) IsRunning() bool {
	l.mux.Lock()
	defer l.mux.Unlock()
	return l.running
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

	if msg.MessageID < l.lastMessageID {
		slog.InfoContext(l.ctx, "message is older than last message, ignoring", "received_msg_id", fmt.Sprintf("%d", msg.MessageID), "recorded_msg_id", fmt.Sprintf("%d", l.lastMessageID))
		return
	}

	var completedJobs []params.ScaleSetJobMessage
	var availableJobs []params.ScaleSetJobMessage
	var startedJobs []params.ScaleSetJobMessage
	var assignedJobs []params.ScaleSetJobMessage

	for _, job := range body {
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
	if len(availableJobs) > 0 {
		jobIDs := make([]int64, len(availableJobs))
		for idx, job := range availableJobs {
			jobIDs[idx] = job.RunnerRequestID
		}
		idsAcquired, err := scaleSetClient.AcquireJobs(
			l.listenerCtx, l.scaleSetHelper.GetScaleSet().ScaleSetID,
			l.messageSession.MessageQueueAccessToken(), jobIDs)
		if err != nil {
			// don't mark message as processed. It will be requeued.
			slog.ErrorContext(l.ctx, "acquiring jobs", "error", err)
			return
		}
		// HandleJobsAvailable only records jobs in the database for now. The jobs are purely
		// informational, so an error here won't break anything.
		if err := l.scaleSetHelper.HandleJobsAvailable(availableJobs); err != nil {
			slog.ErrorContext(l.ctx, "error handling available jobs", "error", err)
		}
		slog.DebugContext(l.ctx, "acquired jobs", "job_ids", idsAcquired)
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
			msg, err := l.messageSession.GetMessage(
				l.listenerCtx, l.lastMessageID, l.scaleSetHelper.GetScaleSet().MaxRunners)
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
			if !msg.IsNil() {
				// Longpoll returns after 50 seconds. If no message arrives during that interval
				// we get a nil message. We can simply ignore it and continue.
				slog.DebugContext(l.ctx, "handling message", "message_id", msg.MessageID)
				l.handleSessionMessage(msg)
			}
		}
	}
}

func (l *scaleSetListener) Wait() <-chan struct{} {
	l.mux.Lock()
	defer l.mux.Unlock()

	if !l.running {
		slog.DebugContext(l.ctx, "scale set listener is not running")
		return closed
	}
	return l.loopExited
}
