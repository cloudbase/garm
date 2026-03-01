package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/locking"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/workers/websocket/agent/messaging"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 16384 // 16 KB
)

func NewAgent(ctx context.Context, conn *websocket.Conn, instance params.Instance, store runner.AgentStoreOps) (*Agent, error) {
	if conn == nil {
		return nil, fmt.Errorf("missing connection for agent")
	}
	consumerID := fmt.Sprintf("agent-worker-%s", instance.Name)
	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", "agent"),
		slog.Any("agent_name", instance.Name),
	)

	return &Agent{
		ctx:           ctx,
		conn:          conn,
		instance:      instance,
		agentStore:    store,
		done:          closed,
		consumerID:    consumerID,
		shellSessions: make(map[string]*ClientSession),
	}, nil
}

type Agent struct {
	ctx        context.Context
	instance   params.Instance
	mux        sync.Mutex
	writeMux   sync.Mutex
	conn       *websocket.Conn
	agentStore runner.AgentStoreOps

	consumerID string
	consumer   common.Consumer

	running bool
	done    chan struct{}

	shellSessions map[string]*ClientSession
}

func (a *Agent) CreateShellSession(ctx context.Context, sessionID uuid.UUID, clientConn *websocket.Conn) (*ClientSession, error) {
	a.mux.Lock()
	defer a.mux.Unlock()

	_, ok := a.shellSessions[sessionID.String()]
	if ok {
		return nil, runnerErrors.NewConflictError("session ID %q already in use", sessionID)
	}
	sess, err := NewClientSession(ctx, clientConn, a.writeMessage, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create new client session: %w", err)
	}

	if err := sess.Start(); err != nil {
		return nil, fmt.Errorf("failed to start client session: %w", err)
	}

	if !a.instance.Capabilities.Shell {
		shellDisabled := messaging.ShellReadyMessage{
			SessionID: sessionID,
			IsError:   1,
			Message:   []byte("agent shell is disabled"),
		}
		sess.safeWrite(websocket.BinaryMessage, shellDisabled.Marshal())
		sess.Stop()
		return nil, fmt.Errorf("agent shell is disabled")
	}
	a.shellSessions[sessionID.String()] = sess
	return sess, nil
}

func (a *Agent) RemoveClientSession(sessionID uuid.UUID, safe bool) error {
	if !safe {
		a.mux.Lock()
		defer a.mux.Unlock()
	}
	sess, ok := a.shellSessions[sessionID.String()]
	if !ok {
		return nil
	}

	if err := sess.Stop(); err != nil {
		return fmt.Errorf("failed to stop session")
	}

	delete(a.shellSessions, sessionID.String())
	return nil
}

func (a *Agent) Done() <-chan struct{} {
	return a.done
}

func (a *Agent) IsRunning() bool {
	return a.running
}

func (a *Agent) Start() error {
	a.mux.Lock()
	defer a.mux.Unlock()

	if a.running {
		return nil
	}

	consumer, err := watcher.RegisterConsumer(
		a.ctx, a.consumerID,
		watcher.WithAll(
			// Filter for update and delete ops for the instance the agent belongs to.
			watcher.WithInstanceFilter(a.instance),
			watcher.WithAny(
				watcher.WithOperationTypeFilter(common.DeleteOperation),
				watcher.WithOperationTypeFilter(common.UpdateOperation),
			),
		))
	if err != nil {
		return fmt.Errorf("registering consumer: %w", err)
	}
	a.consumer = consumer

	a.done = make(chan struct{})
	a.running = true
	go a.agentReader()
	go a.loop()
	return nil
}

func (a *Agent) Stop() error {
	a.mux.Lock()
	defer a.mux.Unlock()

	if !a.running {
		return nil
	}
	slog.InfoContext(a.ctx, "removing sessions")
	for _, val := range a.shellSessions {
		slog.InfoContext(a.ctx, "removing session", "session_id", val.sessionID)
		a.RemoveClientSession(val.sessionID, true)
	}

	a.running = false
	slog.InfoContext(a.ctx, "sending websocket close message")
	a.writeMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	slog.InfoContext(a.ctx, "closing connection")
	a.conn.Close()
	slog.InfoContext(a.ctx, "closing done channel")
	close(a.done)
	return nil
}

func (a *Agent) writeMessage(messageType int, message []byte) error {
	a.writeMux.Lock()
	defer a.writeMux.Unlock()
	if err := a.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}
	if err := a.conn.WriteMessage(messageType, message); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	return nil
}

// agentReader listens for messages sent by the garm-agent. It unmarshals the message and
// routes it to appropriate functions.
func (a *Agent) agentReader() {
	defer func() {
		slog.InfoContext(a.ctx, "stopping agent reader")
		a.Stop()
	}()
	a.conn.SetReadLimit(maxMessageSize)
	a.conn.SetPongHandler(func(string) error {
		if err := a.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			return err
		}
		return nil
	})
	for {
		mt, data, err := a.conn.ReadMessage()
		if err != nil {
			slog.ErrorContext(a.ctx, "error reading websocket message", slog.Any("error", err))
			return
		}

		if mt == websocket.CloseMessage {
			return
		}

		if err := a.messageHandler(data); err != nil {
			if errors.Is(err, ErrShuttingDown) {
				slog.InfoContext(a.ctx, "runner was terminated")
				return
			}
			slog.ErrorContext(a.ctx, "error handling message", slog.Any("error", err))
		}
	}
}

func (a *Agent) handleHeartbeat(agentMsg messaging.AgentMessage) error {
	slog.DebugContext(a.ctx, "received heartbeat message from agent")
	heartbeatMsg, err := messaging.Unmarshal[messaging.RunnerHeartbetMessage](agentMsg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal shell disabled message: %w", err)
	}
	if err := a.agentStore.RecordAgentHeartbeat(a.ctx); err != nil {
		return fmt.Errorf("failed to record heartbeat: %w", err)
	}
	if a.instance.AgentID != int64(heartbeatMsg.AgentID) {
		slog.WarnContext(a.ctx, "missmatching agent ID", "instance_agent_id", a.instance.AgentID, "status_update_agent_id", heartbeatMsg.AgentID)
	}
	slog.DebugContext(a.ctx, "message heartbeat received", "payload", heartbeatMsg.Payload)
	if len(heartbeatMsg.Payload) > 0 {
		var caps params.AgentCapabilities
		if err := json.Unmarshal(heartbeatMsg.Payload, &caps); err != nil {
			return fmt.Errorf("failed to unmarshal capabilities: %w", err)
		}
		if caps.Shell != a.instance.Capabilities.Shell {
			if err := a.agentStore.SetInstanceCapabilities(a.ctx, caps); err != nil {
				return fmt.Errorf("failed to set agent capabilities: %w", err)
			}
		}
	}
	return nil
}

func (a *Agent) handleShellReady(agentMsg messaging.AgentMessage, raw []byte) error {
	shellReady, err := messaging.Unmarshal[messaging.ShellReadyMessage](agentMsg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal shell ready message: %w", err)
	}
	session, ok := a.shellSessions[shellReady.ID()]
	if !ok {
		return nil
	}
	if err := session.Write(raw); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	return nil
}

func (a *Agent) handleShellExit(agentMsg messaging.AgentMessage) error {
	shellExit, err := messaging.Unmarshal[messaging.ShellDataMessage](agentMsg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal shell exit message: %w", err)
	}
	session, ok := a.shellSessions[shellExit.ID()]
	if !ok {
		return nil
	}
	if err := a.RemoveClientSession(session.sessionID, false); err != nil {
		return fmt.Errorf("failed to remove session: %w", err)
	}
	return nil
}

func (a *Agent) handleShellData(agentMsg messaging.AgentMessage, raw []byte) error {
	shellData, err := messaging.Unmarshal[messaging.ShellDataMessage](agentMsg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal shell data message: %w", err)
	}
	session, ok := a.shellSessions[shellData.ID()]
	if !ok {
		return nil
	}
	if err := session.Write(raw); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	return nil
}

func (a *Agent) handleStatusMessage(agentMsg messaging.AgentMessage) error {
	statusUpdate, err := messaging.Unmarshal[messaging.RunnerUpdateMessage](agentMsg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal runner status message: %w", err)
	}
	slog.InfoContext(a.ctx, "got runner status update", "status", string(statusUpdate.Payload))
	if a.instance.AgentID != int64(statusUpdate.AgentID) {
		slog.WarnContext(a.ctx, "missmatching agent ID", "instance_agent_id", a.instance.AgentID, "status_update_agent_id", statusUpdate.AgentID)
	}
	var status params.InstanceUpdateMessage
	if err := json.Unmarshal(statusUpdate.Payload, &status); err != nil {
		return fmt.Errorf("failed to unmarshal instance update: %w", err)
	}
	if err := a.agentStore.AddInstanceStatusMessage(a.ctx, status); err != nil {
		return fmt.Errorf("failed to add status message: %w", err)
	}

	if status.Status == params.RunnerTerminated {
		// try to grab a lock to the instance. We block here.
		if err := locking.LockWithContext(a.ctx, a.instance.Name, a.consumerID); err != nil {
			return fmt.Errorf("failed to acquire lock: %w", err)
		}

		// mark the instance as pending_delete
		if err := a.agentStore.SetInstanceToPendingDelete(a.ctx); err != nil {
			locking.Unlock(a.instance.Name, false)
			return fmt.Errorf("failed to mark instance as pending_delete: %w", err)
		}
		locking.Unlock(a.instance.Name, false)
		return ErrShuttingDown
	}
	return nil
}

func (a *Agent) messageHandler(msg []byte) (err error) {
	if len(msg) < 1 {
		return fmt.Errorf("mesage is too short")
	}
	agentMsg, err := messaging.UnmarshalAgentMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal agetne message")
	}

	switch agentMsg.Type {
	case messaging.MessageTypeHeartbeat:
		return a.handleHeartbeat(agentMsg)
	case messaging.MessageTypeShellReady:
		return a.handleShellReady(agentMsg, msg)
	case messaging.MessageTypeShellExit:
		return a.handleShellExit(agentMsg)
	case messaging.MessageTypeShellData:
		return a.handleShellData(agentMsg, msg)
	case messaging.MessageTypeStatusMessage:
		return a.handleStatusMessage(agentMsg)
	}
	return nil
}

func (a *Agent) loop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		a.Stop()
		ticker.Stop()
	}()
	for {
		select {
		case <-ticker.C:
			if err := a.writeMessage(websocket.PingMessage, nil); err != nil {
				if IsErrorOfInterest(err) {
					slog.With(slog.Any("error", err)).Error("failed to write ping message")
				}
				return
			}
		case <-a.ctx.Done():
			return
		case <-a.done:
			return
		case payload := <-a.consumer.Watch():
			instance, ok := payload.Payload.(params.Instance)
			if !ok {
				continue
			}
			if instance.Name != a.instance.Name {
				slog.WarnContext(a.ctx, "invalid instance object received", "agent_instance", a.instance.Name, "payload_instance", instance.Name)
				continue
			}
			// We only really care about update and delete operations.
			switch payload.Operation {
			case common.UpdateOperation:
				a.mux.Lock()
				a.instance = instance
				a.mux.Unlock()
			case common.DeleteOperation:
				// This instance was deleted. The agent connection needs to be dropped and this worker closed.
				return
			}
		}
	}
}

func IsErrorOfInterest(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, websocket.ErrCloseSent) {
		return false
	}

	if errors.Is(err, websocket.ErrBadHandshake) {
		return false
	}

	if errors.Is(err, net.ErrClosed) {
		return false
	}

	asCloseErr, ok := err.(*websocket.CloseError)
	if ok {
		switch asCloseErr.Code {
		case websocket.CloseNormalClosure, websocket.CloseGoingAway,
			websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure:
			return false
		}
	}

	return true
}
