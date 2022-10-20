package websocket

import (
	"context"
	"fmt"
	"time"
)

func NewHub(ctx context.Context) *Hub {
	return &Hub{
		clients:    map[string]*Client{},
		broadcast:  make(chan []byte, 100),
		register:   make(chan *Client, 100),
		unregister: make(chan *Client, 100),
		ctx:        ctx,
		closed:     make(chan struct{}),
		quit:       make(chan struct{}),
	}
}

type Hub struct {
	ctx    context.Context
	closed chan struct{}
	quit   chan struct{}
	// Registered clients.
	clients map[string]*Client

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func (h *Hub) run() {
	for {
		select {
		case <-h.quit:
			close(h.closed)
			return
		case <-h.ctx.Done():
			close(h.closed)
			return
		case client := <-h.register:
			if client != nil {
				h.clients[client.id] = client
			}
		case client := <-h.unregister:
			if client != nil {
				if _, ok := h.clients[client.id]; ok {
					delete(h.clients, client.id)
					close(client.send)
				}
			}
		case message := <-h.broadcast:
			for id, client := range h.clients {
				if client == nil {
					continue
				}

				select {
				case client.send <- message:
				case <-time.After(5 * time.Second):
					close(client.send)
					delete(h.clients, id)
				}
			}
		}
	}
}

func (h *Hub) Register(client *Client) error {
	h.register <- client
	return nil
}

func (h *Hub) Write(msg []byte) (int, error) {
	select {
	case <-time.After(5 * time.Second):
		return 0, fmt.Errorf("timed out sending message to client")
	case h.broadcast <- msg:

	}
	return len(msg), nil
}

func (h *Hub) Start() error {
	go h.run()
	return nil
}

func (h *Hub) Stop() error {
	close(h.quit)
	select {
	case <-h.closed:
		return nil
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timed out waiting for hub stop")
	}
}

func (h *Hub) Wait() {
	select {
	case <-h.closed:
	case <-time.After(60 * time.Second):
	}
}
