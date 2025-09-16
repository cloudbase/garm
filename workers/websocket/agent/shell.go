package agent

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/cloudbase/garm/workers/websocket/agent/messaging"
)

type writeMessage func(int, []byte) error

func NewClientSession(ctx context.Context, clientConn *websocket.Conn, agentWriter writeMessage, sessionID uuid.UUID) (*ClientSession, error) {
	return &ClientSession{
		ctx:         ctx,
		sessionID:   sessionID,
		clientConn:  clientConn,
		agentWriter: agentWriter,
		done:        closed,
	}, nil
}

type ClientSession struct {
	ctx       context.Context
	sessionID uuid.UUID

	agentWriter writeMessage
	clientConn  *websocket.Conn

	writeMux sync.Mutex
	mux      sync.Mutex

	running bool
	done    chan struct{}
}

func (c *ClientSession) Done() chan struct{} {
	return c.done
}

func (c *ClientSession) Start() error {
	c.mux.Lock()
	defer c.mux.Unlock()

	if c.running {
		return nil
	}

	createShellMsg := messaging.CreateShellMessage{
		SessionID: c.sessionID,
		Rows:      80,
		Cols:      120,
	}
	if err := c.agentWriter(websocket.BinaryMessage, createShellMsg.Marshal()); err != nil {
		return fmt.Errorf("failed to send create shell message:%w", err)
	}

	c.done = make(chan struct{})
	c.running = true
	go c.clientReader()
	go c.loop()

	return nil
}

func (c *ClientSession) Stop() error {
	c.mux.Lock()
	defer c.mux.Unlock()

	if !c.running {
		return nil
	}

	closeShellMsg := messaging.ClientShellClosedMessage{
		SessionID: c.sessionID,
	}
	exitShellMsg := messaging.ShellExitMessage{
		SessionID: c.sessionID,
	}
	c.safeWrite(websocket.BinaryMessage, exitShellMsg.Marshal())
	c.safeWrite(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	c.clientConn.Close()
	close(c.done)
	if err := c.agentWriter(websocket.BinaryMessage, closeShellMsg.Marshal()); err != nil {
		slog.ErrorContext(c.ctx, "failed to send shell closed msg", "error", err)
	}
	c.running = false
	return nil
}

func (c *ClientSession) safeWrite(messageType int, data []byte) error {
	c.writeMux.Lock()
	defer c.writeMux.Unlock()

	if err := c.clientConn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}
	if err := c.clientConn.WriteMessage(messageType, data); err != nil {
		return fmt.Errorf("failed to write message to client: %w", err)
	}
	return nil
}

func (c *ClientSession) Write(msg []byte) error {
	if err := c.safeWrite(websocket.BinaryMessage, msg); err != nil {
		return fmt.Errorf("failed to write message on client websocket: %w", err)
	}
	return nil
}

func (c *ClientSession) clientReader() {
	defer func() {
		c.Stop()
	}()
	c.clientConn.SetReadLimit(maxMessageSize)
	c.clientConn.SetPongHandler(func(string) error {
		if err := c.clientConn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			return err
		}
		return nil
	})
	for {
		mt, data, err := c.clientConn.ReadMessage()
		if err != nil {
			if IsErrorOfInterest(err) {
				slog.ErrorContext(c.ctx, "error reading websocket message", slog.Any("error", err))
			}
			return
		}

		if mt == websocket.CloseMessage {
			return
		}

		if mt != websocket.BinaryMessage && mt != websocket.TextMessage {
			slog.ErrorContext(c.ctx, "invalid message type received", "message_type", mt)
			return
		}
		agentMsg, err := messaging.UnmarshalAgentMessage(data)
		if err != nil {
			slog.ErrorContext(c.ctx, "invalid message received from client", "error", err)
			return
		}

		switch agentMsg.Type {
		case messaging.MessageTypeClientShellClosed, messaging.MessageTypeShellData,
			messaging.MessageTypeShellResize:
		default:
			slog.ErrorContext(c.ctx, "invalid message type received from client", "message_type", agentMsg.Type)
			return
		}
		if !bytes.Equal(agentMsg.Data[:16], c.sessionID[:]) {
			slog.ErrorContext(c.ctx, "invalid session ID")
			return
		}

		if err := c.agentWriter(websocket.BinaryMessage, data); err != nil {
			slog.ErrorContext(c.ctx, "error handling message", slog.Any("error", err))
			return
		}
	}
}

func (c *ClientSession) loop() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		c.Stop()
		ticker.Stop()
	}()

	for {
		select {
		case <-c.done:
			return
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.safeWrite(websocket.PingMessage, nil); err != nil {
				if IsErrorOfInterest(err) {
					slog.With(slog.Any("error", err)).Error("failed to write ping message")
				}
				return
			}
		}
	}
}
