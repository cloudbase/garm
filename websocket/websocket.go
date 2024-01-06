package websocket

import (
	"context"
	"fmt"
	"sync"
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

	mux sync.Mutex
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
				h.mux.Lock()
				h.clients[client.id] = client
				h.mux.Unlock()
			}
		case client := <-h.unregister:
			if client != nil {
				h.mux.Lock()
				if _, ok := h.clients[client.id]; ok {
					client.conn.Close()
					close(client.send)
					delete(h.clients, client.id)
				}
				h.mux.Unlock()
			}
		case message := <-h.broadcast:
			staleClients := []string{}
			for id, client := range h.clients {
				if client == nil {
					staleClients = append(staleClients, id)
					continue
				}

				select {
				case client.send <- message:
				case <-time.After(5 * time.Second):
					staleClients = append(staleClients, id)
				}
			}
			if len(staleClients) > 0 {
				h.mux.Lock()
				for _, id := range staleClients {
					if client, ok := h.clients[id]; ok {
						if client != nil {
							client.conn.Close()
							close(client.send)
						}
						delete(h.clients, id)
					}
				}
				h.mux.Unlock()
			}
		}
	}
}

func (h *Hub) Register(client *Client) error {
	h.register <- client
	return nil
}

func (h *Hub) Write(msg []byte) (int, error) {
	tmp := make([]byte, len(msg))
	copy(tmp, msg)

	select {
	case <-time.After(5 * time.Second):
		return 0, fmt.Errorf("timed out sending message to client")
	case h.broadcast <- tmp:
	}
	return len(tmp), nil
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
