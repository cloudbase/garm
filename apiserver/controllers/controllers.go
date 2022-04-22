package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"runner-manager/apiserver/params"
	gErrors "runner-manager/errors"
	"runner-manager/github"
	"runner-manager/runner"

	"github.com/pkg/errors"
)

func NewAPIController(r *runner.Runner) (*APIController, error) {
	return &APIController{
		r: r,
	}, nil
}

type APIController struct {
	r *runner.Runner
}

func handleError(w http.ResponseWriter, err error) {
	w.Header().Add("Content-Type", "application/json")
	origErr := errors.Cause(err)
	apiErr := params.APIErrorResponse{
		Details: origErr.Error(),
	}

	switch origErr.(type) {
	case *gErrors.NotFoundError:
		w.WriteHeader(http.StatusNotFound)
		apiErr.Error = "Not Found"
	case *gErrors.UnauthorizedError:
		w.WriteHeader(http.StatusUnauthorized)
		apiErr.Error = "Not Authorized"
	case *gErrors.BadRequestError:
		w.WriteHeader(http.StatusBadRequest)
		apiErr.Error = "Bad Request"
	case *gErrors.DuplicateUserError, *gErrors.ConflictError:
		w.WriteHeader(http.StatusConflict)
		apiErr.Error = "Conflict"
	default:
		w.WriteHeader(http.StatusInternalServerError)
		apiErr.Error = "Server error"
	}

	json.NewEncoder(w).Encode(apiErr)
}

func (a *APIController) authenticateHook(body []byte, headers http.Header) error {
	// signature := headers.Get("X-Hub-Signature-256")
	hookType := headers.Get("X-Github-Hook-Installation-Target-Type")
	var workflowJob github.WorkflowJob
	if err := json.Unmarshal(body, &workflowJob); err != nil {
		return gErrors.NewBadRequestError("invalid post body: %s", err)
	}

	switch hookType {
	case "repository":
	case "organization":
	default:
		return gErrors.NewBadRequestError("invalid hook type: %s", hookType)
	}
	return nil
}

func (a *APIController) handleWorkflowJobEvent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleError(w, gErrors.NewBadRequestError("invalid post body: %s", err))
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")
	hookType := r.Header.Get("X-Github-Hook-Installation-Target-Type")
	fmt.Printf(">>> Signature: %s\n", signature)
	fmt.Printf(">>> HookType: %s\n", hookType)

	var workflowJob github.WorkflowJob
	if err := json.Unmarshal(body, &workflowJob); err != nil {
		handleError(w, gErrors.ErrBadRequest)
		return
	}
	// entity := workflowJob.Repository.Owner.Login

	asJs, err := json.MarshalIndent(workflowJob, "", "  ")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%s\n", string(asJs))
}

func (a *APIController) CatchAll(w http.ResponseWriter, r *http.Request) {
	headers := r.Header.Clone()
	for key, val := range headers {
		fmt.Printf("%s --> %v\n", key, val)
	}
	event := github.Event(headers.Get("X-Github-Event"))
	switch event {
	case github.WorkflowJobEvent:
		a.handleWorkflowJobEvent(w, r)
	default:
		log.Printf("ignoring unknown event %s", event)
		return
	}
}

// NotFoundHandler is returned when an invalid URL is acccessed
func (a *APIController) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	apiErr := params.APIErrorResponse{
		Details: "Resource not found",
		Error:   "Not found",
	}
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(apiErr)
}
