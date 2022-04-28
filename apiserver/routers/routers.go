package routers

import (
	"io"
	"net/http"
	"os"

	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"runner-manager/apiserver/controllers"
	"runner-manager/auth"
)

func NewAPIRouter(han *controllers.APIController, logWriter io.Writer, authMiddleware, initMiddleware auth.Middleware) *mux.Router {
	router := mux.NewRouter()
	log := gorillaHandlers.CombinedLoggingHandler

	// Handles github webhooks
	webhookRouter := router.PathPrefix("/webhooks").Subrouter()
	webhookRouter.PathPrefix("/").Handler(log(logWriter, http.HandlerFunc(han.CatchAll)))

	// Handles API calls
	apiSubRouter := router.PathPrefix("/api/v1").Subrouter()

	// FirstRunHandler
	firstRunRouter := apiSubRouter.PathPrefix("/first-run").Subrouter()
	firstRunRouter.Handle("/", log(os.Stdout, http.HandlerFunc(han.FirstRunHandler))).Methods("POST", "OPTIONS")

	// Login
	authRouter := apiSubRouter.PathPrefix("/auth").Subrouter()
	authRouter.Handle("/{login:login\\/?}", log(os.Stdout, http.HandlerFunc(han.LoginHandler))).Methods("POST", "OPTIONS")
	authRouter.Use(initMiddleware.Middleware)

	apiRouter := apiSubRouter.PathPrefix("").Subrouter()
	apiRouter.Use(initMiddleware.Middleware)
	apiRouter.Use(authMiddleware.Middleware)

	/////////////////////
	// Repos and pools //
	/////////////////////
	// Get pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID:poolID\\/?}", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	// Delete pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID:poolID\\/?}", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("DELETE", "OPTIONS")
	// List pools
	apiRouter.Handle("/repositories/{repoID}/pools/", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	// Create pool
	apiRouter.Handle("/repositories/{repoID}/pools/", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("POST", "OPTIONS")

	// Get repo
	apiRouter.Handle("/repositories/{repoID:repoID\\/?}", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	// Delete repo
	apiRouter.Handle("/repositories/{repoID:repoID\\/?}", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("DELETE", "OPTIONS")
	// List repos
	apiRouter.Handle("/repositories/", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	// Create repo
	apiRouter.Handle("/repositories/", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("POST", "OPTIONS")

	/////////////////////////////
	// Organizations and pools //
	/////////////////////////////
	// Get pool
	apiRouter.Handle("/organizations/{repoID}/pools/{poolID:poolID\\/?}", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	// Delete pool
	apiRouter.Handle("/organizations/{repoID}/pools/{poolID:poolID\\/?}", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("DELETE", "OPTIONS")
	// List pools
	apiRouter.Handle("/organizations/{repoID}/pools/", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{repoID}/pools", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	// Create pool
	apiRouter.Handle("/organizations/{repoID}/pools/", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/organizations/{repoID}/pools", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("POST", "OPTIONS")

	// Get repo
	apiRouter.Handle("/organizations/{repoID:repoID\\/?}", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	// Delete repo
	apiRouter.Handle("/organizations/{repoID:repoID\\/?}", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("DELETE", "OPTIONS")
	// List repos
	apiRouter.Handle("/organizations/", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("GET", "OPTIONS")
	// Create repo
	apiRouter.Handle("/organizations/", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/organizations", log(os.Stdout, http.HandlerFunc(han.CatchAll))).Methods("POST", "OPTIONS")

	// Credentials and providers
	apiRouter.Handle("/credentials", log(os.Stdout, http.HandlerFunc(han.ListCredentials))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/providers", log(os.Stdout, http.HandlerFunc(han.ListProviders))).Methods("GET", "OPTIONS")

	return router
}
