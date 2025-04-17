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

	l.listenerCtx, l.cancelFunc = context.WithCancel(context.Background())
	scaleSet := l.scaleSetHelper.GetScaleSet()
	slog.DebugContext(l.ctx, "creating new message session", "scale_set", scaleSet.ScaleSetID)
	session, err := l.scaleSetHelper.ScaleSetCLI().CreateMessageSession(
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

	if l.messageSession != nil {
		slog.DebugContext(l.ctx, "closing message session", "scale_set", l.scaleSetHelper.GetScaleSet().ScaleSetID)
		if err := l.messageSession.Close(); err != nil {
			slog.ErrorContext(l.ctx, "closing message session", "error", err)
		}
		if err := l.scaleSetHelper.ScaleSetCLI().DeleteMessageSession(context.Background(), l.messageSession); err != nil {
			slog.ErrorContext(l.ctx, "error deleting message session", "error", err)
		}
	}

	l.messageSession.Close()
	l.running = false
	l.listenerCtx = nil
	close(l.quit)
	l.cancelFunc()
	return nil
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
	slog.InfoContext(l.ctx, "handling message", "message", msg, "body", body)
	if msg.MessageID < l.lastMessageID {
		slog.DebugContext(l.ctx, "message is older than last message, ignoring")
		return
	}

	var completedJobs []params.ScaleSetJobMessage
	var availableJobs []params.ScaleSetJobMessage
	var startedJobs []params.ScaleSetJobMessage

	for _, job := range body {
		switch job.MessageType {
		case params.MessageTypeJobAssigned:
			slog.InfoContext(l.ctx, "new job assigned", "job_id", job.RunnerRequestId, "job_name", job.JobDisplayName)
		case params.MessageTypeJobStarted:
			slog.InfoContext(l.ctx, "job started", "job_id", job.RunnerRequestId, "job_name", job.JobDisplayName, "runner_name", job.RunnerName)
			startedJobs = append(startedJobs, job)
		case params.MessageTypeJobCompleted:
			slog.InfoContext(l.ctx, "job completed", "job_id", job.RunnerRequestId, "job_name", job.JobDisplayName, "runner_name", job.RunnerName)
			completedJobs = append(completedJobs, job)
		case params.MessageTypeJobAvailable:
			slog.InfoContext(l.ctx, "job available", "job_id", job.RunnerRequestId, "job_name", job.JobDisplayName)
			availableJobs = append(availableJobs, job)
		default:
			slog.DebugContext(l.ctx, "unknown message type", "message_type", job.MessageType)
		}
	}

	if len(availableJobs) > 0 {
		jobIds := make([]int64, len(availableJobs))
		for idx, job := range availableJobs {
			jobIds[idx] = job.RunnerRequestId
		}
		idsAcquired, err := l.scaleSetHelper.ScaleSetCLI().AcquireJobs(
			l.listenerCtx, l.scaleSetHelper.GetScaleSet().ScaleSetID,
			l.messageSession.MessageQueueAccessToken(), jobIds)
		if err != nil {
			// don't mark message as processed. It will be requeued.
			slog.ErrorContext(l.ctx, "acquiring jobs", "error", err)
			return
		}
		slog.DebugContext(l.ctx, "acquired jobs", "job_ids", idsAcquired)
	}

	if len(completedJobs) > 0 {
		if err := l.scaleSetHelper.HandleJobsCompleted(completedJobs); err != nil {
			slog.ErrorContext(l.ctx, "error handling completed jobs", "error", err)
			return
		}
	}

	if len(startedJobs) > 0 {
		if err := l.scaleSetHelper.HandleJobsStarted(startedJobs); err != nil {
			slog.ErrorContext(l.ctx, "error handling started jobs", "error", err)
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
			// TODO: consume initial message on startup and consolidate.
			// The scale set may have undergone several messages while GARM was
			// down.
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
				l.handleSessionMessage(msg)
			}
		}
	}
}

func (l *scaleSetListener) Wait() <-chan struct{} {
	l.mux.Lock()
	if !l.running {
		slog.DebugContext(l.ctx, "scale set listener is not running")
		l.mux.Unlock()
		return nil
	}
	l.mux.Unlock()
	return l.loopExited
}
