// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/apiserver/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/metrics"
	runnerParams "github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner" //nolint:typecheck
	garmUtil "github.com/cloudbase/garm/util"
	wsWriter "github.com/cloudbase/garm/websocket"
	"github.com/cloudbase/garm/workers/websocket/events"
)

func NewAPIController(r *runner.Runner, authenticator *auth.Authenticator, hub *wsWriter.Hub, apiCfg config.APIServer) (*APIController, error) {
	controllerInfo, err := r.GetControllerInfo(auth.GetAdminContext(context.Background()))
	if err != nil {
		return nil, fmt.Errorf("failed to get controller info: %w", err)
	}
	var checkOrigin func(r *http.Request) bool
	if len(apiCfg.CORSOrigins) > 0 {
		checkOrigin = func(r *http.Request) bool {
			origin := r.Header["Origin"]
			if len(origin) == 0 {
				return true
			}
			u, err := url.Parse(origin[0])
			if err != nil {
				return false
			}
			for _, val := range apiCfg.CORSOrigins {
				if val == "*" {
					// user has a setting that allows everything
					return true
				}
				corsVal, err := url.Parse(val)
				if err != nil {
					continue
				}
				if garmUtil.ASCIIEqualFold(u.Host, corsVal.Host) {
					return true
				}
			}
			return false
		}
	}
	return &APIController{
		r:    r,
		auth: authenticator,
		hub:  hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 16384,
			CheckOrigin:     checkOrigin,
		},
		controllerID: controllerInfo.ControllerID.String(),
	}, nil
}

type APIController struct {
	r            *runner.Runner
	auth         *auth.Authenticator
	hub          *wsWriter.Hub
	upgrader     websocket.Upgrader
	controllerID string
}

func handleError(ctx context.Context, w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	apiErr := params.APIErrorResponse{
		Details: err.Error(),
	}
	switch {
	case errors.Is(err, gErrors.ErrNotFound):
		w.WriteHeader(http.StatusNotFound)
		apiErr.Error = "Not Found"
	case errors.Is(err, gErrors.ErrUnauthorized):
		w.WriteHeader(http.StatusUnauthorized)
		apiErr.Error = "Not Authorized"
		// Don't include details on 401 errors.
		apiErr.Details = ""
	case errors.Is(err, gErrors.ErrBadRequest):
		w.WriteHeader(http.StatusBadRequest)
		apiErr.Error = "Bad Request"
	case errors.Is(err, gErrors.ErrDuplicateEntity), errors.Is(err, &gErrors.ConflictError{}):
		w.WriteHeader(http.StatusConflict)
		apiErr.Error = "Conflict"
	default:
		w.WriteHeader(http.StatusInternalServerError)
		apiErr.Error = "Server error"
		// Don't include details on server error.
		apiErr.Details = ""
	}

	if err := json.NewEncoder(w).Encode(apiErr); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

func (a *APIController) handleWorkflowJobEvent(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(ctx, w, gErrors.NewBadRequestError("invalid post body: %s", err))
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")
	hookType := r.Header.Get("X-Github-Hook-Installation-Target-Type")
	giteaTargetType := r.Header.Get("X-Gitea-Hook-Installation-Target-Type")

	forgeType := runnerParams.GithubEndpointType
	if giteaTargetType != "" {
		forgeType = runnerParams.GiteaEndpointType
		hookType = giteaTargetType
	}

	if err := a.r.DispatchWorkflowJob(hookType, signature, forgeType, body); err != nil {
		switch {
		case errors.Is(err, gErrors.ErrNotFound):
			metrics.WebhooksReceived.WithLabelValues(
				"false",         // label: valid
				"owner_unknown", // label: reason
			).Inc()
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "got not found error from DispatchWorkflowJob. webhook not meant for us?")
			return
		case strings.Contains(err.Error(), "signature"):
			// nolint:golangci-lint,godox TODO: check error type
			metrics.WebhooksReceived.WithLabelValues(
				"false",             // label: valid
				"signature_invalid", // label: reason
			).Inc()
		default:
			metrics.WebhooksReceived.WithLabelValues(
				"false",   // label: valid
				"unknown", // label: reason
			).Inc()
		}

		handleError(ctx, w, err)
		return
	}
	metrics.WebhooksReceived.WithLabelValues(
		"true", // label: valid
		"",     // label: reason
	).Inc()
}

func (a *APIController) WebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	controllerID, ok := vars["controllerID"]
	// If the webhook URL includes a controller ID, we validate that it's meant for us. We still
	// support bare webhook URLs, which are tipically configured manually by the user.
	// The controllerID suffixed webhook URL is useful when configuring the webhook for an entity
	// via garm. We cannot tag a webhook URL on github, so there is no way to determine ownership.
	// Using a controllerID suffix is a simple way to denote ownership.
	if ok && controllerID != a.controllerID {
		slog.InfoContext(ctx, "ignoring webhook meant for foreign controller", "req_controller_id", controllerID)
		return
	}

	headers := r.Header.Clone()

	event := runnerParams.Event(headers.Get("X-Github-Event"))
	switch event {
	case runnerParams.WorkflowJobEvent:
		a.handleWorkflowJobEvent(ctx, w, r)
	case runnerParams.PingEvent:
		// Ignore ping event. We may want to save the ping in the github entity table in the future.
	default:
		slog.DebugContext(ctx, "ignoring unknown event", "gh_event", util.SanitizeLogEntry(string(event)))
	}
}

func (a *APIController) EventsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !auth.IsAdmin(ctx) {
		w.WriteHeader(http.StatusForbidden)
		if _, err := w.Write([]byte("events are available to admin users")); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error upgrading to websockets")
		return
	}
	defer conn.Close()

	wsClient, err := wsWriter.NewClient(ctx, conn)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to create new client")
		return
	}
	defer wsClient.Stop()

	eventHandler, err := events.NewHandler(ctx, wsClient)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to create new event handler")
		return
	}

	if err := eventHandler.Start(); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to start event handler")
		return
	}
	<-eventHandler.Done()
}

func (a *APIController) WSHandler(writer http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if !auth.IsAdmin(ctx) {
		writer.WriteHeader(http.StatusForbidden)
		if _, err := writer.Write([]byte("you need admin level access to view logs")); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	if a.hub == nil {
		handleError(ctx, writer, gErrors.NewBadRequestError("log streamer is disabled"))
		return
	}

	conn, err := a.upgrader.Upgrade(writer, req, nil)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error upgrading to websockets")
		return
	}
	defer conn.Close()

	client, err := wsWriter.NewClient(ctx, conn)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to create new client")
		return
	}
	if err := a.hub.Register(client); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to register new client")
		return
	}
	defer a.hub.Unregister(client)

	if err := client.Start(); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to start client")
		return
	}
	<-client.Done()
	slog.Info("client disconnected", "client_id", client.ID())
}

// NotFoundHandler is returned when an invalid URL is acccessed
func (a *APIController) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	apiErr := params.APIErrorResponse{
		Details: "Resource not found",
		Error:   "Not found",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	if err := json.NewEncoder(w).Encode(apiErr); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failet to write response")
	}
}

// swagger:route GET /metrics-token metrics-token GetMetricsToken
//
// Returns a JWT token that can be used to access the metrics endpoint.
//
//	Responses:
//	  200: JWTResponse
//	  401: APIErrorResponse
func (a *APIController) MetricsTokenHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !auth.IsAdmin(ctx) {
		handleError(ctx, w, gErrors.ErrUnauthorized)
		return
	}

	token, err := a.auth.GetJWTMetricsToken(ctx)
	if err != nil {
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(runnerParams.JWTResponse{Token: token})
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /auth/login login Login
//
// Logs in a user and returns a JWT token.
//
//	Parameters:
//	  + name: Body
//	    description: Login information.
//	    type: PasswordLoginParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: JWTResponse
//	  400: APIErrorResponse
//
// LoginHandler returns a jwt token
func (a *APIController) LoginHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var loginInfo runnerParams.PasswordLoginParams
	if err := json.NewDecoder(r.Body).Decode(&loginInfo); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	if err := loginInfo.Validate(); err != nil {
		handleError(ctx, w, err)
		return
	}

	ctx, err := a.auth.AuthenticateUser(ctx, loginInfo)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	tokenString, err := a.auth.GetJWTToken(ctx)
	if err != nil {
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(runnerParams.JWTResponse{Token: tokenString}); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /first-run first-run FirstRun
//
// Initialize the first run of the controller.
//
//	Parameters:
//	  + name: Body
//	    description: Create a new user.
//	    type: NewUserParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: User
//	  400: APIErrorResponse
func (a *APIController) FirstRunHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if a.auth.IsInitialized() {
		err := gErrors.NewConflictError("already initialized")
		handleError(ctx, w, err)
		return
	}

	var newUserParams runnerParams.NewUserParams
	if err := json.NewDecoder(r.Body).Decode(&newUserParams); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	newUser, err := a.auth.InitController(ctx, newUserParams)
	if err != nil {
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(newUser); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /providers providers ListProviders
//
// List all providers.
//
//	Responses:
//	  200: Providers
//	  400: APIErrorResponse
func (a *APIController) ListProviders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	providers, err := a.r.ListProviders(ctx)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(providers); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /jobs jobs ListJobs
//
// List all jobs.
//
//	Responses:
//	  200: Jobs
//	  400: APIErrorResponse
func (a *APIController) ListAllJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobs, err := a.r.ListAllJobs(ctx)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(jobs); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /controller-info controllerInfo ControllerInfo
//
// Get controller info.
//
//	Responses:
//	  200: ControllerInfo
//	  409: APIErrorResponse
func (a *APIController) ControllerInfoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	info, err := a.r.GetControllerInfo(ctx)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route PUT /controller controller UpdateController
//
// Update controller.
//
//	Parameters:
//	  + name: Body
//	    description: Parameters used when updating the controller.
//	    type: UpdateControllerParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: ControllerInfo
//	  400: APIErrorResponse
func (a *APIController) UpdateControllerHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var updateParams runnerParams.UpdateControllerParams
	if err := json.NewDecoder(r.Body).Decode(&updateParams); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	if err := updateParams.Validate(); err != nil {
		handleError(ctx, w, err)
		return
	}

	info, err := a.r.UpdateController(ctx, updateParams)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}
