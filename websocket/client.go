package websocket

import (
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/gorilla/websocket"
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

func NewClient(conn *websocket.Conn, hub *Hub) (*Client, error) {
	clientID := uuid.New()
	return &Client{
		id:   clientID.String(),
		conn: conn,
		hub:  hub,
		send: make(chan []byte, 100),
	}, nil
}

type Client struct {
	id   string
	conn *websocket.Conn
	// Buffered channel of outbound messages.
	send chan []byte

	hub *Hub
}

func (c *Client) Go() {
	go c.clientReader()
	go c.clientWriter()
}

// clientReader waits for options changes from the client. The client can at any time
// change the log level and binary name it watches.
func (c *Client) clientReader() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
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
		mt, _, err := c.conn.ReadMessage()
		if err != nil {
			break
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
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				slog.With(slog.Any("error", err)).Error("failed to set write deadline")
			}
			if !ok {
				// The hub closed the channel.
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					slog.With(slog.Any("error", err)).Error("failed to write message")
				}
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				slog.With(slog.Any("error", err)).Error("error sending message")
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				slog.With(slog.Any("error", err)).Error("failed to set write deadline")
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
