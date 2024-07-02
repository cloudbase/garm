package websocket

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 1024
)

type HandleWebsocketMessage func([]byte) error

func NewClient(ctx context.Context, conn *websocket.Conn) (*Client, error) {
	clientID := uuid.New()
	consumerID := fmt.Sprintf("ws-client-watcher-%s", clientID.String())

	user := auth.UserID(ctx)
	if user == "" {
		return nil, fmt.Errorf("user not found in context")
	}
	generation := auth.PasswordGeneration(ctx)

	consumer, err := watcher.RegisterConsumer(
		ctx, consumerID,
		watcher.WithUserIDFilter(user),
	)
	if err != nil {
		return nil, errors.Wrap(err, "registering consumer")
	}
	return &Client{
		id:                 clientID.String(),
		conn:               conn,
		ctx:                ctx,
		userID:             user,
		passwordGeneration: generation,
		consumer:           consumer,
		done:               make(chan struct{}),
		send:               make(chan []byte, 100),
	}, nil
}

type Client struct {
	id   string
	conn *websocket.Conn
	// Buffered channel of outbound messages.
	send chan []byte
	mux  sync.Mutex
	ctx  context.Context

	userID             string
	passwordGeneration uint
	consumer           common.Consumer

	messageHandler HandleWebsocketMessage

	running bool
	done    chan struct{}
}

func (c *Client) ID() string {
	return c.id
}

func (c *Client) Stop() {
	c.mux.Lock()
	defer c.mux.Unlock()

	if !c.running {
		return
	}

	c.running = false
	c.conn.Close()
	close(c.send)
	close(c.done)
}

func (c *Client) Done() <-chan struct{} {
	return c.done
}

func (c *Client) SetMessageHandler(handler HandleWebsocketMessage) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.messageHandler = handler
}

func (c *Client) Start() error {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.running = true

	go c.runWatcher()
	go c.clientReader()
	go c.clientWriter()

	return nil
}

func (c *Client) Write(msg []byte) (int, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if !c.running {
		return 0, fmt.Errorf("client is stopped")
	}

	tmp := make([]byte, len(msg))
	copy(tmp, msg)

	select {
	case <-time.After(5 * time.Second):
		return 0, fmt.Errorf("timed out sending message to client")
	case c.send <- tmp:
	}
	return len(tmp), nil
}

// clientReader waits for options changes from the client. The client can at any time
// change the log level and binary name it watches.
func (c *Client) clientReader() {
	defer func() {
		c.Stop()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		slog.With(slog.Any("error", err)).Error("failed to set read deadline")
	}
	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			return err
		}
		return nil
	})
	for {
		mt, data, err := c.conn.ReadMessage()
		if err != nil {
			if IsErrorOfInterest(err) {
				slog.ErrorContext(c.ctx, "error reading websocket message", slog.Any("error", err))
			}
			break
		}

		if c.messageHandler != nil {
			if err := c.messageHandler(data); err != nil {
				slog.ErrorContext(c.ctx, "error handling message", slog.Any("error", err))
			}
		}
		if mt == websocket.CloseMessage {
			break
		}
	}
}

// clientWriter
func (c *Client) clientWriter() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		c.Stop()
		ticker.Stop()
	}()
	var authExpires time.Time
	expires := auth.Expires(c.ctx)
	if expires != nil {
		authExpires = *expires
	}
	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				slog.With(slog.Any("error", err)).Error("failed to set write deadline")
			}
			if !ok {
				// The hub closed the channel.
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					if IsErrorOfInterest(err) {
						slog.With(slog.Any("error", err)).Error("failed to write message")
					}
				}
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				if IsErrorOfInterest(err) {
					slog.With(slog.Any("error", err)).Error("error sending message")
				}
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				slog.With(slog.Any("error", err)).Error("failed to set write deadline")
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				if IsErrorOfInterest(err) {
					slog.With(slog.Any("error", err)).Error("failed to write ping message")
				}
				return
			}
		case <-c.ctx.Done():
			return
		case <-time.After(time.Until(authExpires)):
			// Auth has expired
			slog.DebugContext(c.ctx, "auth expired, closing connection")
			return
		}
	}
}

func (c *Client) runWatcher() {
	defer func() {
		c.Stop()
	}()
	for {
		select {
		case <-c.Done():
			return
		case <-c.ctx.Done():
			return
		case event, ok := <-c.consumer.Watch():
			if !ok {
				slog.InfoContext(c.ctx, "watcher closed")
				return
			}
			go func(event common.ChangePayload) {
				if event.EntityType != common.UserEntityType {
					return
				}

				user, ok := event.Payload.(params.User)
				if !ok {
					slog.ErrorContext(c.ctx, "failed to cast payload to user")
					return
				}

				if user.ID != c.userID {
					return
				}

				if user.Generation != c.passwordGeneration {
					slog.InfoContext(c.ctx, "password generation mismatch; closing connection")
					c.Stop()
				}
			}(event)
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
