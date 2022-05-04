package routers

import (
	"io"
	"net/http"
	"os"

	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"garm/apiserver/controllers"
	"garm/auth"
)

func NewAPIRouter(han *controllers.APIController, logWriter io.Writer, authMiddleware, initMiddleware, instanceMiddleware auth.Middleware) *mux.Router {
	router := mux.NewRouter()
	log := gorillaHandlers.CombinedLoggingHandler

	// Handles github webhooks
	webhookRouter := router.PathPrefix("/webhooks").Subrouter()
	webhookRouter.PathPrefix("/").Handler(log(logWriter, http.HandlerFunc(han.CatchAll)))
	webhookRouter.PathPrefix("").Handler(log(logWriter, http.HandlerFunc(han.CatchAll)))

	// Handles API calls
	apiSubRouter := router.PathPrefix("/api/v1").Subrouter()

	// FirstRunHandler
	firstRunRouter := apiSubRouter.PathPrefix("/first-run").Subrouter()
	firstRunRouter.Handle("/", log(os.Stdout, http.HandlerFunc(han.FirstRunHandler))).Methods("POST", "OPTIONS")

	// Instance callback
	callbackRouter := apiSubRouter.PathPrefix("/callbacks").Subrouter()
	callbackRouter.Handle("/status/", log(os.Stdout, http.HandlerFunc(han.InstanceStatusMessageHandler))).Methods("POST", "OPTIONS")
	callbackRouter.Handle("/status", log(os.Stdout, http.HandlerFunc(han.InstanceStatusMessageHandler))).Methods("POST", "OPTIONS")
	callbackRouter.Use(instanceMiddleware.Middleware)
	// Login
	authRouter := apiSubRouter.PathPrefix("/auth").Subrouter()
	authRouter.Handle("/{login:login\\/?}", log(os.Stdout, http.HandlerFunc(han.LoginHandler))).Methods("POST", "OPTIONS")
	authRouter.Use(initMiddleware.Middleware)

	apiRouter := apiSubRouter.PathPrefix("").Subrouter()
	apiRouter.Use(initMiddleware.Middleware)
	apiRouter.Use(authMiddleware.Middleware)

	// Runners (instances)
	// List pool instances
	apiRouter.Handle("/pools/instances/{poolID}/", log(os.Stdout, http.HandlerFunc(han.ListPoolInstancesHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/pools/instances/{poolID}", log(os.Stdout, http.HandlerFunc(han.ListPoolInstancesHandler))).Methods("GET", "OPTIONS")
	// Get instance
	apiRouter.Handle("/instances/{instanceName}/", log(os.Stdout, http.HandlerFunc(han.GetInstanceHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/instances/{instanceName}", log(os.Stdout, http.HandlerFunc(han.GetInstanceHandler))).Methods("GET", "OPTIONS")
	// Delete instance
	// apiRouter.Handle("/instances/{instanceName}/", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("DELETE", "OPTIONS")
	// apiRouter.Handle("/instances/{instanceName}", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("DELETE", "OPTIONS")

	/////////////////////
	// Repos and pools //
	/////////////////////
	// Get pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}/", log(os.Stdout, http.HandlerFunc(han.GetRepoPoolHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", log(os.Stdout, http.HandlerFunc(han.GetRepoPoolHandler))).Methods("GET", "OPTIONS")
	// Delete pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}/", log(os.Stdout, http.HandlerFunc(han.DeleteRepoPoolHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", log(os.Stdout, http.HandlerFunc(han.DeleteRepoPoolHandler))).Methods("DELETE", "OPTIONS")
	// Update pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}/", log(os.Stdout, http.HandlerFunc(han.UpdateRepoPoolHandler))).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", log(os.Stdout, http.HandlerFunc(han.UpdateRepoPoolHandler))).Methods("PUT", "OPTIONS")
	// List pools
	apiRouter.Handle("/repositories/{repoID}/pools/", log(os.Stdout, http.HandlerFunc(han.ListRepoPoolsHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools", log(os.Stdout, http.HandlerFunc(han.ListRepoPoolsHandler))).Methods("GET", "OPTIONS")
	// Create pool
	apiRouter.Handle("/repositories/{repoID}/pools/", log(os.Stdout, http.HandlerFunc(han.CreateRepoPoolHandler))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools", log(os.Stdout, http.HandlerFunc(han.CreateRepoPoolHandler))).Methods("POST", "OPTIONS")

	// Repo instances list
	apiRouter.Handle("/repositories/{repoID}/instances/", log(os.Stdout, http.HandlerFunc(han.ListRepoInstancesHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/instances", log(os.Stdout, http.HandlerFunc(han.ListRepoInstancesHandler))).Methods("GET", "OPTIONS")

	// Get repo
	apiRouter.Handle("/repositories/{repoID}/", log(os.Stdout, http.HandlerFunc(han.GetRepoByIDHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}", log(os.Stdout, http.HandlerFunc(han.GetRepoByIDHandler))).Methods("GET", "OPTIONS")
	// Update repo
	apiRouter.Handle("/repositories/{repoID}/", log(os.Stdout, http.HandlerFunc(han.UpdateRepoHandler))).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}", log(os.Stdout, http.HandlerFunc(han.UpdateRepoHandler))).Methods("PUT", "OPTIONS")
	// Delete repo
	apiRouter.Handle("/repositories/{repoID}/", log(os.Stdout, http.HandlerFunc(han.DeleteRepoHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}", log(os.Stdout, http.HandlerFunc(han.DeleteRepoHandler))).Methods("DELETE", "OPTIONS")
	// List repos
	apiRouter.Handle("/repositories/", log(os.Stdout, http.HandlerFunc(han.ListReposHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories", log(os.Stdout, http.HandlerFunc(han.ListReposHandler))).Methods("GET", "OPTIONS")
	// Create repo
	apiRouter.Handle("/repositories/", log(os.Stdout, http.HandlerFunc(han.CreateRepoHandler))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories", log(os.Stdout, http.HandlerFunc(han.CreateRepoHandler))).Methods("POST", "OPTIONS")

	/////////////////////////////
	// Organizations and pools //
	/////////////////////////////
	// Get pool
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}/", log(os.Stdout, http.HandlerFunc(han.GetOrgPoolHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}", log(os.Stdout, http.HandlerFunc(han.GetOrgPoolHandler))).Methods("GET", "OPTIONS")
	// Delete pool
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}/", log(os.Stdout, http.HandlerFunc(han.DeleteOrgPoolHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}", log(os.Stdout, http.HandlerFunc(han.DeleteOrgPoolHandler))).Methods("DELETE", "OPTIONS")
	// Update pool
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}/", log(os.Stdout, http.HandlerFunc(han.UpdateOrgPoolHandler))).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}", log(os.Stdout, http.HandlerFunc(han.UpdateOrgPoolHandler))).Methods("PUT", "OPTIONS")
	// List pools
	apiRouter.Handle("/organizations/{orgID}/pools/", log(os.Stdout, http.HandlerFunc(han.ListOrgPoolsHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools", log(os.Stdout, http.HandlerFunc(han.ListOrgPoolsHandler))).Methods("GET", "OPTIONS")
	// Create pool
	apiRouter.Handle("/organizations/{orgID}/pools/", log(os.Stdout, http.HandlerFunc(han.CreateOrgPoolHandler))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools", log(os.Stdout, http.HandlerFunc(han.CreateOrgPoolHandler))).Methods("POST", "OPTIONS")

	// Repo instances list
	apiRouter.Handle("/organizations/{orgID}/instances/", log(os.Stdout, http.HandlerFunc(han.ListOrgInstancesHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/instances", log(os.Stdout, http.HandlerFunc(han.ListOrgInstancesHandler))).Methods("GET", "OPTIONS")

	// Get org
	apiRouter.Handle("/organizations/{orgID}/", log(os.Stdout, http.HandlerFunc(han.GetOrgByIDHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}", log(os.Stdout, http.HandlerFunc(han.GetOrgByIDHandler))).Methods("GET", "OPTIONS")
	// Update org
	apiRouter.Handle("/organizations/{orgID}/", log(os.Stdout, http.HandlerFunc(han.UpdateOrgHandler))).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}", log(os.Stdout, http.HandlerFunc(han.UpdateOrgHandler))).Methods("PUT", "OPTIONS")
	// Delete org
	apiRouter.Handle("/organizations/{orgID}/", log(os.Stdout, http.HandlerFunc(han.DeleteOrgHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}", log(os.Stdout, http.HandlerFunc(han.DeleteOrgHandler))).Methods("DELETE", "OPTIONS")
	// List orgs
	apiRouter.Handle("/organizations/", log(os.Stdout, http.HandlerFunc(han.ListOrgsHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations", log(os.Stdout, http.HandlerFunc(han.ListOrgsHandler))).Methods("GET", "OPTIONS")
	// Create org
	apiRouter.Handle("/organizations/", log(os.Stdout, http.HandlerFunc(han.CreateOrgHandler))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/organizations", log(os.Stdout, http.HandlerFunc(han.CreateOrgHandler))).Methods("POST", "OPTIONS")

	// Credentials and providers
	apiRouter.Handle("/credentials/", log(os.Stdout, http.HandlerFunc(han.ListCredentials))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/credentials", log(os.Stdout, http.HandlerFunc(han.ListCredentials))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/providers/", log(os.Stdout, http.HandlerFunc(han.ListProviders))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/providers", log(os.Stdout, http.HandlerFunc(han.ListProviders))).Methods("GET", "OPTIONS")

	return router
}
