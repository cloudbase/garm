package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	garmUtil "github.com/cloudbase/garm/util"
)

var closed = make(chan struct{})

func init() { close(closed) }

func NewHub(ctx context.Context) (*Hub, error) {
	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", "agent-hub"),
	)

	return &Hub{
		ctx:    ctx,
		agents: make(map[string]*Agent),
		done:   closed,
	}, nil
}

type Hub struct {
	ctx    context.Context
	agents map[string]*Agent
	mux    sync.Mutex

	done    chan struct{}
	running bool
}

func (a *Hub) Start() error {
	a.mux.Lock()
	defer a.mux.Unlock()

	if a.running {
		return nil
	}

	a.done = make(chan struct{})
	a.running = true
	go a.loop()
	return nil
}

func (a *Hub) GetAgent(agentID string) (*Agent, error) {
	a.mux.Lock()
	defer a.mux.Unlock()

	if agent, ok := a.agents[agentID]; ok {
		return agent, nil
	}
	return nil, runnerErrors.NewNotFoundError("no such agent")
}

func (a *Hub) Stop() error {
	a.mux.Lock()
	defer a.mux.Unlock()

	if !a.running {
		return nil
	}

	close(a.done)
	a.running = false
	return nil
}

func (a *Hub) RegisterAgent(agent *Agent) error {
	a.mux.Lock()
	defer a.mux.Unlock()

	if agent == nil {
		return fmt.Errorf("missing agent")
	}

	if !agent.IsRunning() {
		return fmt.Errorf("agent is not running; refusing to register")
	}

	if _, ok := a.agents[agent.instance.Name]; ok {
		return runnerErrors.NewConflictError("agent %s is already registered", agent.instance.Name)
	}
	a.agents[agent.instance.Name] = agent
	go a.reapStoppedAgent(agent)

	return nil
}

func (a *Hub) UnregisterAgent(agentID string) error {
	a.mux.Lock()
	defer a.mux.Unlock()
	if agent, ok := a.agents[agentID]; ok {
		if err := agent.Stop(); err != nil {
			return fmt.Errorf("failed to stop agent loop for runner %s: %w", agent.instance.Name, err)
		}
		delete(a.agents, agentID)
	}
	return nil
}

func (a *Hub) reapStoppedAgent(agent *Agent) {
	if agent == nil {
		return
	}

	select {
	case <-agent.Done():
		slog.InfoContext(a.ctx, "agent is done", "agent_name", agent.instance.Name)
		if err := a.UnregisterAgent(agent.instance.Name); err != nil {
			slog.ErrorContext(a.ctx, "failed to unregister stopped agent", "agent", agent.instance.Name, "error", err)
		}
	case <-a.ctx.Done():
	case <-a.done:
	}
}

func (a *Hub) loop() {
	defer func() {
		a.Stop()
	}()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.done:
			return
		}
	}
}
