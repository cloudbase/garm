package controllers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/apiserver/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/workers/websocket/agent"
)

func (a *APIController) AgentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		slog.ErrorContext(ctx, "failed to authenticate instance")
		return
	}

	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error upgrading to websockets")
		return
	}

	slog.DebugContext(ctx, "new agent connected", "agent_name", instance.Name)
	agent, err := agent.NewAgent(ctx, conn, instance, a.r)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to create agent")
		return
	}
	defer func() {
		slog.DebugContext(ctx, "stopping agent", "agent_name", instance.Name)
		agent.Stop()
	}()

	if err := agent.Start(); err != nil {
		slog.ErrorContext(ctx, "failed to start agent loop", "error", err, "agent_name", instance.Name)
		handleError(ctx, w, err)
		return
	}

	if err := a.agentHub.RegisterAgent(agent); err != nil {
		handleError(ctx, w, err)
		return
	}
	defer a.agentHub.UnregisterAgent(instance.Name)

	select {
	case <-agent.Done():
	case <-ctx.Done():
	}
	slog.InfoContext(ctx, "connection closed", "agent_name", instance.Name)
}

func (a *APIController) AgentShellHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !auth.IsAdmin(ctx) {
		handleError(ctx, w, gErrors.ErrUnauthorized)
		return
	}

	vars := mux.Vars(r)
	agentName, ok := vars["agentName"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No agent name specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	sessionID, err := uuid.NewRandom()
	if err != nil {
		handleError(ctx, w, fmt.Errorf("failed to generate UUID: %w", err))
		return
	}

	agent, err := a.agentHub.GetAgent(agentName)
	if err != nil {
		slog.InfoContext(ctx, "session for agent not found", "agent_name", agentName)
		handleError(ctx, w, err)
		return
	}

	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error upgrading to websockets")
		return
	}

	sess, err := agent.CreateShellSession(ctx, sessionID, conn)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create client session", "error", err)
		return
	}
	slog.InfoContext(ctx, "shell session created", "session_id", sessionID, "agent_name", agentName)
	defer agent.RemoveClientSession(sessionID, false)

	select {
	case <-sess.Done():
	case <-agent.Done():
	case <-ctx.Done():
	}
	slog.InfoContext(ctx, "connection closed", "session_id", sessionID, "agent_name", agentName)
}
