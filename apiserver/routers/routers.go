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

package routers

import (
	"io"
	"net/http"

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
	firstRunRouter.Handle("/", log(logWriter, http.HandlerFunc(han.FirstRunHandler))).Methods("POST", "OPTIONS")

	// Instance callback
	callbackRouter := apiSubRouter.PathPrefix("/callbacks").Subrouter()
	callbackRouter.Handle("/status/", log(logWriter, http.HandlerFunc(han.InstanceStatusMessageHandler))).Methods("POST", "OPTIONS")
	callbackRouter.Handle("/status", log(logWriter, http.HandlerFunc(han.InstanceStatusMessageHandler))).Methods("POST", "OPTIONS")
	callbackRouter.Use(instanceMiddleware.Middleware)
	// Login
	authRouter := apiSubRouter.PathPrefix("/auth").Subrouter()
	authRouter.Handle("/{login:login\\/?}", log(logWriter, http.HandlerFunc(han.LoginHandler))).Methods("POST", "OPTIONS")
	authRouter.Use(initMiddleware.Middleware)

	apiRouter := apiSubRouter.PathPrefix("").Subrouter()
	apiRouter.Use(initMiddleware.Middleware)
	apiRouter.Use(authMiddleware.Middleware)

	///////////
	// Pools //
	///////////
	// List all pools
	apiRouter.Handle("/pools/", log(logWriter, http.HandlerFunc(han.ListAllPoolsHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/pools", log(logWriter, http.HandlerFunc(han.ListAllPoolsHandler))).Methods("GET", "OPTIONS")
	// Get one pool
	apiRouter.Handle("/pools/{poolID}/", log(logWriter, http.HandlerFunc(han.GetPoolByIDHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/pools/{poolID}", log(logWriter, http.HandlerFunc(han.GetPoolByIDHandler))).Methods("GET", "OPTIONS")
	// Delete one pool
	apiRouter.Handle("/pools/{poolID}/", log(logWriter, http.HandlerFunc(han.DeletePoolByIDHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/pools/{poolID}", log(logWriter, http.HandlerFunc(han.DeletePoolByIDHandler))).Methods("DELETE", "OPTIONS")
	// Update one pool
	apiRouter.Handle("/pools/{poolID}/", log(logWriter, http.HandlerFunc(han.UpdatePoolByIDHandler))).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/pools/{poolID}", log(logWriter, http.HandlerFunc(han.UpdatePoolByIDHandler))).Methods("PUT", "OPTIONS")
	// List pool instances
	apiRouter.Handle("/pools/{poolID}/instances/", log(logWriter, http.HandlerFunc(han.ListPoolInstancesHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/pools/{poolID}/instances", log(logWriter, http.HandlerFunc(han.ListPoolInstancesHandler))).Methods("GET", "OPTIONS")

	/////////////
	// Runners //
	/////////////
	// Get instance
	apiRouter.Handle("/instances/{instanceName}/", log(logWriter, http.HandlerFunc(han.GetInstanceHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/instances/{instanceName}", log(logWriter, http.HandlerFunc(han.GetInstanceHandler))).Methods("GET", "OPTIONS")
	// Delete runner
	apiRouter.Handle("/instances/{instanceName}/", log(logWriter, http.HandlerFunc(han.DeleteInstanceHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/instances/{instanceName}", log(logWriter, http.HandlerFunc(han.DeleteInstanceHandler))).Methods("DELETE", "OPTIONS")
	// List runners
	apiRouter.Handle("/instances/", log(logWriter, http.HandlerFunc(han.ListAllInstancesHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/instances", log(logWriter, http.HandlerFunc(han.ListAllInstancesHandler))).Methods("GET", "OPTIONS")

	/////////////////////
	// Repos and pools //
	/////////////////////
	// Get pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}/", log(logWriter, http.HandlerFunc(han.GetRepoPoolHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", log(logWriter, http.HandlerFunc(han.GetRepoPoolHandler))).Methods("GET", "OPTIONS")
	// Delete pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}/", log(logWriter, http.HandlerFunc(han.DeleteRepoPoolHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", log(logWriter, http.HandlerFunc(han.DeleteRepoPoolHandler))).Methods("DELETE", "OPTIONS")
	// Update pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}/", log(logWriter, http.HandlerFunc(han.UpdateRepoPoolHandler))).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", log(logWriter, http.HandlerFunc(han.UpdateRepoPoolHandler))).Methods("PUT", "OPTIONS")
	// List pools
	apiRouter.Handle("/repositories/{repoID}/pools/", log(logWriter, http.HandlerFunc(han.ListRepoPoolsHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools", log(logWriter, http.HandlerFunc(han.ListRepoPoolsHandler))).Methods("GET", "OPTIONS")
	// Create pool
	apiRouter.Handle("/repositories/{repoID}/pools/", log(logWriter, http.HandlerFunc(han.CreateRepoPoolHandler))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools", log(logWriter, http.HandlerFunc(han.CreateRepoPoolHandler))).Methods("POST", "OPTIONS")

	// Repo instances list
	apiRouter.Handle("/repositories/{repoID}/instances/", log(logWriter, http.HandlerFunc(han.ListRepoInstancesHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/instances", log(logWriter, http.HandlerFunc(han.ListRepoInstancesHandler))).Methods("GET", "OPTIONS")

	// Get repo
	apiRouter.Handle("/repositories/{repoID}/", log(logWriter, http.HandlerFunc(han.GetRepoByIDHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}", log(logWriter, http.HandlerFunc(han.GetRepoByIDHandler))).Methods("GET", "OPTIONS")
	// Update repo
	apiRouter.Handle("/repositories/{repoID}/", log(logWriter, http.HandlerFunc(han.UpdateRepoHandler))).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}", log(logWriter, http.HandlerFunc(han.UpdateRepoHandler))).Methods("PUT", "OPTIONS")
	// Delete repo
	apiRouter.Handle("/repositories/{repoID}/", log(logWriter, http.HandlerFunc(han.DeleteRepoHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}", log(logWriter, http.HandlerFunc(han.DeleteRepoHandler))).Methods("DELETE", "OPTIONS")
	// List repos
	apiRouter.Handle("/repositories/", log(logWriter, http.HandlerFunc(han.ListReposHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories", log(logWriter, http.HandlerFunc(han.ListReposHandler))).Methods("GET", "OPTIONS")
	// Create repo
	apiRouter.Handle("/repositories/", log(logWriter, http.HandlerFunc(han.CreateRepoHandler))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories", log(logWriter, http.HandlerFunc(han.CreateRepoHandler))).Methods("POST", "OPTIONS")

	/////////////////////////////
	// Organizations and pools //
	/////////////////////////////
	// Get pool
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}/", log(logWriter, http.HandlerFunc(han.GetOrgPoolHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}", log(logWriter, http.HandlerFunc(han.GetOrgPoolHandler))).Methods("GET", "OPTIONS")
	// Delete pool
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}/", log(logWriter, http.HandlerFunc(han.DeleteOrgPoolHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}", log(logWriter, http.HandlerFunc(han.DeleteOrgPoolHandler))).Methods("DELETE", "OPTIONS")
	// Update pool
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}/", log(logWriter, http.HandlerFunc(han.UpdateOrgPoolHandler))).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}", log(logWriter, http.HandlerFunc(han.UpdateOrgPoolHandler))).Methods("PUT", "OPTIONS")
	// List pools
	apiRouter.Handle("/organizations/{orgID}/pools/", log(logWriter, http.HandlerFunc(han.ListOrgPoolsHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools", log(logWriter, http.HandlerFunc(han.ListOrgPoolsHandler))).Methods("GET", "OPTIONS")
	// Create pool
	apiRouter.Handle("/organizations/{orgID}/pools/", log(logWriter, http.HandlerFunc(han.CreateOrgPoolHandler))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools", log(logWriter, http.HandlerFunc(han.CreateOrgPoolHandler))).Methods("POST", "OPTIONS")

	// Repo instances list
	apiRouter.Handle("/organizations/{orgID}/instances/", log(logWriter, http.HandlerFunc(han.ListOrgInstancesHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/instances", log(logWriter, http.HandlerFunc(han.ListOrgInstancesHandler))).Methods("GET", "OPTIONS")

	// Get org
	apiRouter.Handle("/organizations/{orgID}/", log(logWriter, http.HandlerFunc(han.GetOrgByIDHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}", log(logWriter, http.HandlerFunc(han.GetOrgByIDHandler))).Methods("GET", "OPTIONS")
	// Update org
	apiRouter.Handle("/organizations/{orgID}/", log(logWriter, http.HandlerFunc(han.UpdateOrgHandler))).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}", log(logWriter, http.HandlerFunc(han.UpdateOrgHandler))).Methods("PUT", "OPTIONS")
	// Delete org
	apiRouter.Handle("/organizations/{orgID}/", log(logWriter, http.HandlerFunc(han.DeleteOrgHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}", log(logWriter, http.HandlerFunc(han.DeleteOrgHandler))).Methods("DELETE", "OPTIONS")
	// List orgs
	apiRouter.Handle("/organizations/", log(logWriter, http.HandlerFunc(han.ListOrgsHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations", log(logWriter, http.HandlerFunc(han.ListOrgsHandler))).Methods("GET", "OPTIONS")
	// Create org
	apiRouter.Handle("/organizations/", log(logWriter, http.HandlerFunc(han.CreateOrgHandler))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/organizations", log(logWriter, http.HandlerFunc(han.CreateOrgHandler))).Methods("POST", "OPTIONS")

	/////////////////////////////
	//  Enterprises and pools  //
	/////////////////////////////
	// Get pool
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}/", log(os.Stdout, http.HandlerFunc(han.GetEnterprisePoolHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}", log(os.Stdout, http.HandlerFunc(han.GetEnterprisePoolHandler))).Methods("GET", "OPTIONS")
	// Delete pool
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}/", log(os.Stdout, http.HandlerFunc(han.DeleteEnterprisePoolHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}", log(os.Stdout, http.HandlerFunc(han.DeleteEnterprisePoolHandler))).Methods("DELETE", "OPTIONS")
	// Update pool
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}/", log(os.Stdout, http.HandlerFunc(han.UpdateEnterprisePoolHandler))).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}", log(os.Stdout, http.HandlerFunc(han.UpdateEnterprisePoolHandler))).Methods("PUT", "OPTIONS")
	// List pools
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/", log(os.Stdout, http.HandlerFunc(han.ListEnterprisePoolsHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/pools", log(os.Stdout, http.HandlerFunc(han.ListEnterprisePoolsHandler))).Methods("GET", "OPTIONS")
	// Create pool
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/", log(os.Stdout, http.HandlerFunc(han.CreateEnterprisePoolHandler))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/pools", log(os.Stdout, http.HandlerFunc(han.CreateEnterprisePoolHandler))).Methods("POST", "OPTIONS")

	// Repo instances list
	apiRouter.Handle("/enterprises/{enterpriseID}/instances/", log(os.Stdout, http.HandlerFunc(han.ListEnterpriseInstancesHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/instances", log(os.Stdout, http.HandlerFunc(han.ListEnterpriseInstancesHandler))).Methods("GET", "OPTIONS")

	// Get org
	apiRouter.Handle("/enterprises/{enterpriseID}/", log(os.Stdout, http.HandlerFunc(han.GetEnterpriseByIDHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}", log(os.Stdout, http.HandlerFunc(han.GetEnterpriseByIDHandler))).Methods("GET", "OPTIONS")
	// Update org
	apiRouter.Handle("/enterprises/{enterpriseID}/", log(os.Stdout, http.HandlerFunc(han.UpdateEnterpriseHandler))).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}", log(os.Stdout, http.HandlerFunc(han.UpdateEnterpriseHandler))).Methods("PUT", "OPTIONS")
	// Delete org
	apiRouter.Handle("/enterprises/{enterpriseID}/", log(os.Stdout, http.HandlerFunc(han.DeleteEnterpriseHandler))).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}", log(os.Stdout, http.HandlerFunc(han.DeleteEnterpriseHandler))).Methods("DELETE", "OPTIONS")
	// List orgs
	apiRouter.Handle("/enterprises/", log(os.Stdout, http.HandlerFunc(han.ListEnterprisesHandler))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/enterprises", log(os.Stdout, http.HandlerFunc(han.ListEnterprisesHandler))).Methods("GET", "OPTIONS")
	// Create org
	apiRouter.Handle("/enterprises/", log(os.Stdout, http.HandlerFunc(han.CreateEnterpriseHandler))).Methods("POST", "OPTIONS")
	apiRouter.Handle("/enterprises", log(os.Stdout, http.HandlerFunc(han.CreateEnterpriseHandler))).Methods("POST", "OPTIONS")

	// Credentials and providers
	apiRouter.Handle("/credentials/", log(logWriter, http.HandlerFunc(han.ListCredentials))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/credentials", log(logWriter, http.HandlerFunc(han.ListCredentials))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/providers/", log(logWriter, http.HandlerFunc(han.ListProviders))).Methods("GET", "OPTIONS")
	apiRouter.Handle("/providers", log(logWriter, http.HandlerFunc(han.ListProviders))).Methods("GET", "OPTIONS")

	// Websocket log writer
	apiRouter.Handle("/{ws:ws\\/?}", log(logWriter, http.HandlerFunc(han.WSHandler))).Methods("GET")
	return router
}
