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

	mux     sync.Mutex
	running bool
	once    sync.Once
}

func (h *Hub) run() {
	defer close(h.closed)
	defer h.Stop()

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
							client.Stop()
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
	case h.broadcast <- tmp:
		return len(tmp), nil
	case <-h.quit:
		return 0, fmt.Errorf("websocket hub is shutting down")
	default:
		return 0, fmt.Errorf("failed to broadcast over websocket")
	}
}

func (h *Hub) Start() error {
	h.mux.Lock()
	defer h.mux.Unlock()

	if h.running {
		return nil
	}

	h.running = true

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
	h.mux.Lock()
	defer h.mux.Unlock()

	if !h.running {
		return nil
	}

	h.running = false
	h.Close()
	return h.Wait()
}

func (h *Hub) Wait() error {
	if !h.running {
		return nil
	}
	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()
	select {
	case <-h.closed:
	case <-timer.C:
		return fmt.Errorf("timed out waiting for hub stop")
	}
	return nil
}
