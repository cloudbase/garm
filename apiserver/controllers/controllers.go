package controllers

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"runner-manager/apiserver/params"
	"runner-manager/auth"
	gErrors "runner-manager/errors"
	runnerParams "runner-manager/params"
	"runner-manager/runner"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func NewAPIController(r *runner.Runner, auth *auth.Authenticator) (*APIController, error) {
	return &APIController{
		r:    r,
		auth: auth,
	}, nil
}

type APIController struct {
	r    *runner.Runner
	auth *auth.Authenticator
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
	var workflowJob runnerParams.WorkflowJob
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
	// fmt.Printf(">>> Signature: %s\n", signature)
	// fmt.Printf(">>> HookType: %s\n", hookType)

	if err := a.r.DispatchWorkflowJob(hookType, signature, body); err != nil {
		log.Printf("failed to dispatch work: %s", err)
		handleError(w, err)
		return
	}
}

func (a *APIController) CatchAll(w http.ResponseWriter, r *http.Request) {
	headers := r.Header.Clone()

	event := runnerParams.Event(headers.Get("X-Github-Event"))
	switch event {
	case runnerParams.WorkflowJobEvent:
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiErr)
}

// LoginHandler returns a jwt token
func (a *APIController) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var loginInfo runnerParams.PasswordLoginParams
	if err := json.NewDecoder(r.Body).Decode(&loginInfo); err != nil {
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	if err := loginInfo.Validate(); err != nil {
		handleError(w, err)
		return
	}

	ctx := r.Context()
	ctx, err := a.auth.AuthenticateUser(ctx, loginInfo)
	if err != nil {
		handleError(w, err)
		return
	}

	tokenString, err := a.auth.GetJWTToken(ctx)
	if err != nil {
		handleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runnerParams.JWTResponse{Token: tokenString})
}

func (a *APIController) FirstRunHandler(w http.ResponseWriter, r *http.Request) {
	if a.auth.IsInitialized() {
		err := gErrors.NewConflictError("already initialized")
		handleError(w, err)
		return
	}

	ctx := r.Context()

	var newUserParams runnerParams.NewUserParams
	if err := json.NewDecoder(r.Body).Decode(&newUserParams); err != nil {
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	newUser, err := a.auth.InitController(ctx, newUserParams)
	if err != nil {
		handleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newUser)

}

func (a *APIController) ListCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds, err := a.r.ListCredentials(ctx)
	if err != nil {
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(creds)
}

func (a *APIController) ListProviders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	providers, err := a.r.ListProviders(ctx)
	if err != nil {
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providers)
}

func (a *APIController) CreateRepoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var repoData runnerParams.CreateRepoParams
	if err := json.NewDecoder(r.Body).Decode(&repoData); err != nil {
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	repo, err := a.r.CreateRepository(ctx, repoData)
	if err != nil {
		log.Printf("error creating repository: %+v", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repo)
}

func (a *APIController) ListReposHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	repos, err := a.r.ListRepositories(ctx)
	if err != nil {
		log.Printf("listing repos: %+v", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repos)
}

func (a *APIController) GetRepoByIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, ok := vars["repoID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo ID specified",
		})
		return
	}

	repo, err := a.r.GetRepositoryByID(ctx, repoID)
	if err != nil {
		log.Printf("fetching repo: %+v", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repo)
}

func (a *APIController) DeleteRepoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, ok := vars["repoID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo ID specified",
		})
		return
	}

	if err := a.r.DeleteRepository(ctx, repoID); err != nil {
		log.Printf("fetching repo: %+v", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

}

func (a *APIController) UpdateRepoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, ok := vars["repoID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo ID specified",
		})
		return
	}

	var updatePayload runnerParams.UpdateRepositoryParams
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	repo, err := a.r.UpdateRepository(ctx, repoID, updatePayload)
	if err != nil {
		log.Printf("error updating repository: %+v", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repo)
}

func (a *APIController) CreateRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, ok := vars["repoID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo ID specified",
		})
		return
	}

	var poolData runnerParams.CreatePoolParams
	if err := json.NewDecoder(r.Body).Decode(&poolData); err != nil {
		log.Printf("failed to decode: %+v", err)
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	pool, err := a.r.CreateRepoPool(ctx, repoID, poolData)
	if err != nil {
		log.Printf("error creating repository pool: %+v", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
}

func (a *APIController) ListRepoPoolsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	repoID, ok := vars["repoID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo ID specified",
		})
		return
	}

	pools, err := a.r.ListRepoPools(ctx, repoID)
	if err != nil {
		log.Printf("listing pools: %+v", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pools)
}

func (a *APIController) GetRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	repoID, repoOk := vars["repoID"]
	poolID, poolOk := vars["poolID"]
	if !repoOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo or pool ID specified",
		})
		return
	}

	pool, err := a.r.GetRepoPoolByID(ctx, repoID, poolID)
	if err != nil {
		log.Printf("listing pools: %+v", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
}

func (a *APIController) DeleteRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, repoOk := vars["repoID"]
	poolID, poolOk := vars["poolID"]
	if !repoOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo or pool ID specified",
		})
		return
	}

	if err := a.r.DeleteRepoPool(ctx, repoID, poolID); err != nil {
		log.Printf("removing pool: %+v", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

}

func (a *APIController) UpdateRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, repoOk := vars["repoID"]
	poolID, poolOk := vars["poolID"]
	if !repoOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo or pool ID specified",
		})
		return
	}

	var poolData runnerParams.UpdatePoolParams
	if err := json.NewDecoder(r.Body).Decode(&poolData); err != nil {
		log.Printf("failed to decode: %+v", err)
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	pool, err := a.r.UpdateRepoPool(ctx, repoID, poolID, poolData)
	if err != nil {
		log.Printf("error creating repository pool: %+v", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
}
