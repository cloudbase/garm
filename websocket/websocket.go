package websocket

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

func NewHub(ctx context.Context) *Hub {
	return &Hub{
		clients:   map[string]*Client{},
		broadcast: make(chan []byte, 100),
		ctx:       ctx,
		closed:    make(chan struct{}),
		quit:      make(chan struct{}),
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

	mux  sync.Mutex
	once sync.Once
}

func (h *Hub) run() {
	defer func() {
		close(h.closed)
	}()
	for {
		select {
		case <-h.quit:
			return
		case <-h.ctx.Done():
			return
		case message := <-h.broadcast:
			staleClients := []string{}
			for id, client := range h.clients {
				if client == nil {
					staleClients = append(staleClients, id)
					continue
				}

				if _, err := client.Write(message); err != nil {
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
	if client == nil {
		return nil
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	cli, ok := h.clients[client.ID()]
	if ok {
		if cli != nil {
			return fmt.Errorf("client already registered")
		}
	}
	slog.DebugContext(h.ctx, "registering client", "client_id", client.ID())
	h.clients[client.id] = client
	return nil
}

func (h *Hub) Unregister(client *Client) error {
	if client == nil {
		return nil
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	cli, ok := h.clients[client.ID()]
	if ok {
		cli.Stop()
		slog.DebugContext(h.ctx, "unregistering client", "client_id", cli.ID())
		delete(h.clients, cli.ID())
		slog.DebugContext(h.ctx, "current client count", "count", len(h.clients))
	}
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

func (h *Hub) Close() error {
	h.once.Do(func() {
		close(h.quit)
	})
	return nil
}

func (h *Hub) Stop() error {
	h.Close()
	return h.Wait()
}

func (h *Hub) Wait() error {
	select {
	case <-h.closed:
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timed out waiting for hub stop")
	}
	return nil
}
