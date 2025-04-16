package scaleset

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/github/scalesets"
)

func newListener(ctx context.Context, scaleSetHelper scaleSetHelper) *scaleSetListener {
	return &scaleSetListener{
		ctx:            ctx,
		scaleSetHelper: scaleSetHelper,
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

	mux     sync.Mutex
	running bool
	quit    chan struct{}
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
	l.cancelFunc()
	l.messageSession.Close()
	l.running = false
	close(l.quit)
	return nil
}

func (l *scaleSetListener) handleSessionMessage(msg params.RunnerScaleSetMessage) {
	l.mux.Lock()
	defer l.mux.Unlock()
	body, err := msg.GetJobsFromBody()
	if err != nil {
		slog.ErrorContext(l.ctx, "getting jobs from body", "error", err)
		return
	}
	slog.InfoContext(l.ctx, "handling message", "message", msg, "body", body)
	l.lastMessageID = msg.MessageID
}

func (l *scaleSetListener) loop() {
	defer l.Stop()

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
			slog.DebugContext(l.ctx, "getting message")
			msg, err := l.messageSession.GetMessage(
				l.listenerCtx, l.lastMessageID, l.scaleSetHelper.GetScaleSet().MaxRunners)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					slog.ErrorContext(l.ctx, "getting message", "error", err)
				}
				return
			}
			l.handleSessionMessage(msg)
		}
	}
}

func (l *scaleSetListener) Wait() <-chan struct{} {
	if !l.running {
		return nil
	}
	return l.listenerCtx.Done()
}
