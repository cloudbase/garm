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

// Package routers Garm API.
//
// The Garm API generated using go-swagger.
//
//	BasePath: /api/v1
//	Version: 1.0.0
//	License: Apache 2.0 https://www.apache.org/licenses/LICENSE-2.0
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//
//	Security:
//	- Bearer:
//
//	SecurityDefinitions:
//	  Bearer:
//	    type: apiKey
//	    name: Authorization
//	    in: header
//	    description: >-
//	      The token with the `Bearer: ` prefix, e.g. "Bearer abcde12345".
//
// swagger:meta
package routers

//go:generate go run github.com/go-swagger/go-swagger/cmd/swagger@v0.30.5 generate spec --input=../swagger-models.yaml --output=../swagger.yaml --include="routers|controllers"
//go:generate go run github.com/go-swagger/go-swagger/cmd/swagger@v0.30.5 validate ../swagger.yaml
//go:generate rm -rf ../../client
//go:generate go run github.com/go-swagger/go-swagger/cmd/swagger@v0.30.5 generate client --target=../../ --spec=../swagger.yaml

import (
	_ "expvar" // Register the expvar handlers
	"log/slog"
	"net/http"
	_ "net/http/pprof" // Register the pprof handlers

	"github.com/felixge/httpsnoop"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/cloudbase/garm/apiserver/controllers"
	"github.com/cloudbase/garm/auth"
)

func WithMetricsRouter(parentRouter *mux.Router, disableAuth bool, metricsMiddlerware auth.Middleware) *mux.Router {
	if parentRouter == nil {
		return nil
	}

	metricsRouter := parentRouter.PathPrefix("/metrics").Subrouter()
	if !disableAuth {
		metricsRouter.Use(metricsMiddlerware.Middleware)
	}
	metricsRouter.Handle("/", promhttp.Handler()).Methods("GET", "OPTIONS")
	metricsRouter.Handle("", promhttp.Handler()).Methods("GET", "OPTIONS")
	return parentRouter
}

func WithDebugServer(parentRouter *mux.Router) *mux.Router {
	if parentRouter == nil {
		return nil
	}

	parentRouter.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	return parentRouter
}

func requestLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// gathers metrics from the upstream handlers
		metrics := httpsnoop.CaptureMetrics(h, w, r)

		//prints log and metrics
		slog.Info(
			"access_log",
			slog.String("method", r.Method),
			slog.String("uri", r.URL.RequestURI()),
			slog.String("user_agent", r.Header.Get("User-Agent")),
			slog.String("ip", r.RemoteAddr),
			slog.Int("code", metrics.Code),
			slog.Int64("bytes", metrics.Written),
			slog.Duration("request_time", metrics.Duration),
		)
	})
}

func NewAPIRouter(han *controllers.APIController, authMiddleware, initMiddleware, instanceMiddleware auth.Middleware, manageWebhooks bool) *mux.Router {
	router := mux.NewRouter()
	router.Use(requestLogger)

	// Handles github webhooks
	webhookRouter := router.PathPrefix("/webhooks").Subrouter()
	webhookRouter.Handle("/", http.HandlerFunc(han.WebhookHandler))
	webhookRouter.Handle("", http.HandlerFunc(han.WebhookHandler))
	webhookRouter.Handle("/{controllerID}/", http.HandlerFunc(han.WebhookHandler))
	webhookRouter.Handle("/{controllerID}", http.HandlerFunc(han.WebhookHandler))

	// Handles API calls
	apiSubRouter := router.PathPrefix("/api/v1").Subrouter()

	// FirstRunHandler
	firstRunRouter := apiSubRouter.PathPrefix("/first-run").Subrouter()
	firstRunRouter.Handle("/", http.HandlerFunc(han.FirstRunHandler)).Methods("POST", "OPTIONS")
	firstRunRouter.Handle("", http.HandlerFunc(han.FirstRunHandler)).Methods("POST", "OPTIONS")

	// Instance URLs
	callbackRouter := apiSubRouter.PathPrefix("/callbacks").Subrouter()
	callbackRouter.Handle("/status/", http.HandlerFunc(han.InstanceStatusMessageHandler)).Methods("POST", "OPTIONS")
	callbackRouter.Handle("/status", http.HandlerFunc(han.InstanceStatusMessageHandler)).Methods("POST", "OPTIONS")
	callbackRouter.Handle("/system-info/", http.HandlerFunc(han.InstanceSystemInfoHandler)).Methods("POST", "OPTIONS")
	callbackRouter.Handle("/system-info", http.HandlerFunc(han.InstanceSystemInfoHandler)).Methods("POST", "OPTIONS")
	callbackRouter.Use(instanceMiddleware.Middleware)

	///////////////////
	// Metadata URLs //
	///////////////////
	metadataRouter := apiSubRouter.PathPrefix("/metadata").Subrouter()
	metadataRouter.Use(instanceMiddleware.Middleware)

	// Registration token
	metadataRouter.Handle("/runner-registration-token/", http.HandlerFunc(han.InstanceGithubRegistrationTokenHandler)).Methods("GET", "OPTIONS")
	metadataRouter.Handle("/runner-registration-token", http.HandlerFunc(han.InstanceGithubRegistrationTokenHandler)).Methods("GET", "OPTIONS")
	// JIT credential files
	metadataRouter.Handle("/credentials/{fileName}/", http.HandlerFunc(han.JITCredentialsFileHandler)).Methods("GET", "OPTIONS")
	metadataRouter.Handle("/credentials/{fileName}", http.HandlerFunc(han.JITCredentialsFileHandler)).Methods("GET", "OPTIONS")
	// Systemd files
	metadataRouter.Handle("/system/service-name/", http.HandlerFunc(han.SystemdServiceNameHandler)).Methods("GET", "OPTIONS")
	metadataRouter.Handle("/system/service-name", http.HandlerFunc(han.SystemdServiceNameHandler)).Methods("GET", "OPTIONS")
	metadataRouter.Handle("/systemd/unit-file/", http.HandlerFunc(han.SystemdUnitFileHandler)).Methods("GET", "OPTIONS")
	metadataRouter.Handle("/systemd/unit-file", http.HandlerFunc(han.SystemdUnitFileHandler)).Methods("GET", "OPTIONS")
	metadataRouter.Handle("/system/cert-bundle/", http.HandlerFunc(han.RootCertificateBundleHandler)).Methods("GET", "OPTIONS")
	metadataRouter.Handle("/system/cert-bundle", http.HandlerFunc(han.RootCertificateBundleHandler)).Methods("GET", "OPTIONS")

	// Login
	authRouter := apiSubRouter.PathPrefix("/auth").Subrouter()
	authRouter.Handle("/{login:login\\/?}", http.HandlerFunc(han.LoginHandler)).Methods("POST", "OPTIONS")
	authRouter.Use(initMiddleware.Middleware)

	apiRouter := apiSubRouter.PathPrefix("").Subrouter()
	apiRouter.Use(initMiddleware.Middleware)
	apiRouter.Use(authMiddleware.Middleware)
	apiRouter.Use(auth.AdminRequiredMiddleware)

	// Metrics Token
	apiRouter.Handle("/metrics-token/", http.HandlerFunc(han.MetricsTokenHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/metrics-token", http.HandlerFunc(han.MetricsTokenHandler)).Methods("GET", "OPTIONS")

	//////////
	// Jobs //
	//////////
	// List all jobs
	apiRouter.Handle("/jobs/", http.HandlerFunc(han.ListAllJobs)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/jobs", http.HandlerFunc(han.ListAllJobs)).Methods("GET", "OPTIONS")

	///////////
	// Pools //
	///////////
	// List all pools
	apiRouter.Handle("/pools/", http.HandlerFunc(han.ListAllPoolsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/pools", http.HandlerFunc(han.ListAllPoolsHandler)).Methods("GET", "OPTIONS")
	// Get one pool
	apiRouter.Handle("/pools/{poolID}/", http.HandlerFunc(han.GetPoolByIDHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/pools/{poolID}", http.HandlerFunc(han.GetPoolByIDHandler)).Methods("GET", "OPTIONS")
	// Delete one pool
	apiRouter.Handle("/pools/{poolID}/", http.HandlerFunc(han.DeletePoolByIDHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/pools/{poolID}", http.HandlerFunc(han.DeletePoolByIDHandler)).Methods("DELETE", "OPTIONS")
	// Update one pool
	apiRouter.Handle("/pools/{poolID}/", http.HandlerFunc(han.UpdatePoolByIDHandler)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/pools/{poolID}", http.HandlerFunc(han.UpdatePoolByIDHandler)).Methods("PUT", "OPTIONS")
	// List pool instances
	apiRouter.Handle("/pools/{poolID}/instances/", http.HandlerFunc(han.ListPoolInstancesHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/pools/{poolID}/instances", http.HandlerFunc(han.ListPoolInstancesHandler)).Methods("GET", "OPTIONS")

	/////////////
	// Runners //
	/////////////
	// Get instance
	apiRouter.Handle("/instances/{instanceName}/", http.HandlerFunc(han.GetInstanceHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/instances/{instanceName}", http.HandlerFunc(han.GetInstanceHandler)).Methods("GET", "OPTIONS")
	// Delete runner
	apiRouter.Handle("/instances/{instanceName}/", http.HandlerFunc(han.DeleteInstanceHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/instances/{instanceName}", http.HandlerFunc(han.DeleteInstanceHandler)).Methods("DELETE", "OPTIONS")
	// List runners
	apiRouter.Handle("/instances/", http.HandlerFunc(han.ListAllInstancesHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/instances", http.HandlerFunc(han.ListAllInstancesHandler)).Methods("GET", "OPTIONS")

	/////////////////////
	// Repos and pools //
	/////////////////////
	// Get pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}/", http.HandlerFunc(han.GetRepoPoolHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", http.HandlerFunc(han.GetRepoPoolHandler)).Methods("GET", "OPTIONS")
	// Delete pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}/", http.HandlerFunc(han.DeleteRepoPoolHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", http.HandlerFunc(han.DeleteRepoPoolHandler)).Methods("DELETE", "OPTIONS")
	// Update pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}/", http.HandlerFunc(han.UpdateRepoPoolHandler)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", http.HandlerFunc(han.UpdateRepoPoolHandler)).Methods("PUT", "OPTIONS")
	// List pools
	apiRouter.Handle("/repositories/{repoID}/pools/", http.HandlerFunc(han.ListRepoPoolsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools", http.HandlerFunc(han.ListRepoPoolsHandler)).Methods("GET", "OPTIONS")
	// Create pool
	apiRouter.Handle("/repositories/{repoID}/pools/", http.HandlerFunc(han.CreateRepoPoolHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools", http.HandlerFunc(han.CreateRepoPoolHandler)).Methods("POST", "OPTIONS")

	// Repo instances list
	apiRouter.Handle("/repositories/{repoID}/instances/", http.HandlerFunc(han.ListRepoInstancesHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/instances", http.HandlerFunc(han.ListRepoInstancesHandler)).Methods("GET", "OPTIONS")

	// Get repo
	apiRouter.Handle("/repositories/{repoID}/", http.HandlerFunc(han.GetRepoByIDHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}", http.HandlerFunc(han.GetRepoByIDHandler)).Methods("GET", "OPTIONS")
	// Update repo
	apiRouter.Handle("/repositories/{repoID}/", http.HandlerFunc(han.UpdateRepoHandler)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}", http.HandlerFunc(han.UpdateRepoHandler)).Methods("PUT", "OPTIONS")
	// Delete repo
	apiRouter.Handle("/repositories/{repoID}/", http.HandlerFunc(han.DeleteRepoHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}", http.HandlerFunc(han.DeleteRepoHandler)).Methods("DELETE", "OPTIONS")
	// List repos
	apiRouter.Handle("/repositories/", http.HandlerFunc(han.ListReposHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories", http.HandlerFunc(han.ListReposHandler)).Methods("GET", "OPTIONS")
	// Create repo
	apiRouter.Handle("/repositories/", http.HandlerFunc(han.CreateRepoHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories", http.HandlerFunc(han.CreateRepoHandler)).Methods("POST", "OPTIONS")

	if manageWebhooks {
		// Install Webhook
		apiRouter.Handle("/repositories/{repoID}/webhook/", http.HandlerFunc(han.InstallRepoWebhookHandler)).Methods("POST", "OPTIONS")
		apiRouter.Handle("/repositories/{repoID}/webhook", http.HandlerFunc(han.InstallRepoWebhookHandler)).Methods("POST", "OPTIONS")
		// Uninstall Webhook
		apiRouter.Handle("/repositories/{repoID}/webhook/", http.HandlerFunc(han.UninstallRepoWebhookHandler)).Methods("DELETE", "OPTIONS")
		apiRouter.Handle("/repositories/{repoID}/webhook", http.HandlerFunc(han.UninstallRepoWebhookHandler)).Methods("DELETE", "OPTIONS")
		// Get webhook info
		apiRouter.Handle("/repositories/{repoID}/webhook/", http.HandlerFunc(han.GetRepoWebhookInfoHandler)).Methods("GET", "OPTIONS")
		apiRouter.Handle("/repositories/{repoID}/webhook", http.HandlerFunc(han.GetRepoWebhookInfoHandler)).Methods("GET", "OPTIONS")
	}
	/////////////////////////////
	// Organizations and pools //
	/////////////////////////////
	// Get pool
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}/", http.HandlerFunc(han.GetOrgPoolHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}", http.HandlerFunc(han.GetOrgPoolHandler)).Methods("GET", "OPTIONS")
	// Delete pool
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}/", http.HandlerFunc(han.DeleteOrgPoolHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}", http.HandlerFunc(han.DeleteOrgPoolHandler)).Methods("DELETE", "OPTIONS")
	// Update pool
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}/", http.HandlerFunc(han.UpdateOrgPoolHandler)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools/{poolID}", http.HandlerFunc(han.UpdateOrgPoolHandler)).Methods("PUT", "OPTIONS")
	// List pools
	apiRouter.Handle("/organizations/{orgID}/pools/", http.HandlerFunc(han.ListOrgPoolsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools", http.HandlerFunc(han.ListOrgPoolsHandler)).Methods("GET", "OPTIONS")
	// Create pool
	apiRouter.Handle("/organizations/{orgID}/pools/", http.HandlerFunc(han.CreateOrgPoolHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/pools", http.HandlerFunc(han.CreateOrgPoolHandler)).Methods("POST", "OPTIONS")

	// Org instances list
	apiRouter.Handle("/organizations/{orgID}/instances/", http.HandlerFunc(han.ListOrgInstancesHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/instances", http.HandlerFunc(han.ListOrgInstancesHandler)).Methods("GET", "OPTIONS")

	// Get org
	apiRouter.Handle("/organizations/{orgID}/", http.HandlerFunc(han.GetOrgByIDHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}", http.HandlerFunc(han.GetOrgByIDHandler)).Methods("GET", "OPTIONS")
	// Update org
	apiRouter.Handle("/organizations/{orgID}/", http.HandlerFunc(han.UpdateOrgHandler)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}", http.HandlerFunc(han.UpdateOrgHandler)).Methods("PUT", "OPTIONS")
	// Delete org
	apiRouter.Handle("/organizations/{orgID}/", http.HandlerFunc(han.DeleteOrgHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}", http.HandlerFunc(han.DeleteOrgHandler)).Methods("DELETE", "OPTIONS")
	// List orgs
	apiRouter.Handle("/organizations/", http.HandlerFunc(han.ListOrgsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations", http.HandlerFunc(han.ListOrgsHandler)).Methods("GET", "OPTIONS")
	// Create org
	apiRouter.Handle("/organizations/", http.HandlerFunc(han.CreateOrgHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/organizations", http.HandlerFunc(han.CreateOrgHandler)).Methods("POST", "OPTIONS")

	if manageWebhooks {
		// Install Webhook
		apiRouter.Handle("/organizations/{orgID}/webhook/", http.HandlerFunc(han.InstallOrgWebhookHandler)).Methods("POST", "OPTIONS")
		apiRouter.Handle("/organizations/{orgID}/webhook", http.HandlerFunc(han.InstallOrgWebhookHandler)).Methods("POST", "OPTIONS")
		// Uninstall Webhook
		apiRouter.Handle("/organizations/{orgID}/webhook/", http.HandlerFunc(han.UninstallOrgWebhookHandler)).Methods("DELETE", "OPTIONS")
		apiRouter.Handle("/organizations/{orgID}/webhook", http.HandlerFunc(han.UninstallOrgWebhookHandler)).Methods("DELETE", "OPTIONS")
		// Get webhook info
		apiRouter.Handle("/organizations/{orgID}/webhook/", http.HandlerFunc(han.GetOrgWebhookInfoHandler)).Methods("GET", "OPTIONS")
		apiRouter.Handle("/organizations/{orgID}/webhook", http.HandlerFunc(han.GetOrgWebhookInfoHandler)).Methods("GET", "OPTIONS")
	}
	/////////////////////////////
	//  Enterprises and pools  //
	/////////////////////////////
	// Get pool
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}/", http.HandlerFunc(han.GetEnterprisePoolHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}", http.HandlerFunc(han.GetEnterprisePoolHandler)).Methods("GET", "OPTIONS")
	// Delete pool
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}/", http.HandlerFunc(han.DeleteEnterprisePoolHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}", http.HandlerFunc(han.DeleteEnterprisePoolHandler)).Methods("DELETE", "OPTIONS")
	// Update pool
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}/", http.HandlerFunc(han.UpdateEnterprisePoolHandler)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/{poolID}", http.HandlerFunc(han.UpdateEnterprisePoolHandler)).Methods("PUT", "OPTIONS")
	// List pools
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/", http.HandlerFunc(han.ListEnterprisePoolsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/pools", http.HandlerFunc(han.ListEnterprisePoolsHandler)).Methods("GET", "OPTIONS")
	// Create pool
	apiRouter.Handle("/enterprises/{enterpriseID}/pools/", http.HandlerFunc(han.CreateEnterprisePoolHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/pools", http.HandlerFunc(han.CreateEnterprisePoolHandler)).Methods("POST", "OPTIONS")

	// Enterprise instances list
	apiRouter.Handle("/enterprises/{enterpriseID}/instances/", http.HandlerFunc(han.ListEnterpriseInstancesHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/instances", http.HandlerFunc(han.ListEnterpriseInstancesHandler)).Methods("GET", "OPTIONS")

	// Get enterprise
	apiRouter.Handle("/enterprises/{enterpriseID}/", http.HandlerFunc(han.GetEnterpriseByIDHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}", http.HandlerFunc(han.GetEnterpriseByIDHandler)).Methods("GET", "OPTIONS")
	// Update enterprise
	apiRouter.Handle("/enterprises/{enterpriseID}/", http.HandlerFunc(han.UpdateEnterpriseHandler)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}", http.HandlerFunc(han.UpdateEnterpriseHandler)).Methods("PUT", "OPTIONS")
	// Delete enterprise
	apiRouter.Handle("/enterprises/{enterpriseID}/", http.HandlerFunc(han.DeleteEnterpriseHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}", http.HandlerFunc(han.DeleteEnterpriseHandler)).Methods("DELETE", "OPTIONS")
	// List enterprises
	apiRouter.Handle("/enterprises/", http.HandlerFunc(han.ListEnterprisesHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/enterprises", http.HandlerFunc(han.ListEnterprisesHandler)).Methods("GET", "OPTIONS")
	// Create enterprise
	apiRouter.Handle("/enterprises/", http.HandlerFunc(han.CreateEnterpriseHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/enterprises", http.HandlerFunc(han.CreateEnterpriseHandler)).Methods("POST", "OPTIONS")

	// Credentials and providers
	apiRouter.Handle("/credentials/", http.HandlerFunc(han.ListCredentials)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/credentials", http.HandlerFunc(han.ListCredentials)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/providers/", http.HandlerFunc(han.ListProviders)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/providers", http.HandlerFunc(han.ListProviders)).Methods("GET", "OPTIONS")

	// Controller info
	apiRouter.Handle("/controller-info/", http.HandlerFunc(han.ControllerInfoHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/controller-info", http.HandlerFunc(han.ControllerInfoHandler)).Methods("GET", "OPTIONS")

	// Websocket log writer
	apiRouter.Handle("/{ws:ws\\/?}", http.HandlerFunc(han.WSHandler)).Methods("GET")

	// NotFound handler
	apiRouter.PathPrefix("/").HandlerFunc(han.NotFoundHandler).Methods("GET", "POST", "PUT", "DELETE", "OPTIONS")
	return router
}
