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

//go:generate go run github.com/go-swagger/go-swagger/cmd/swagger@v0.31.0 generate spec --input=../swagger-models.yaml --output=../swagger.yaml --include="routers|controllers"
//go:generate go run github.com/go-swagger/go-swagger/cmd/swagger@v0.31.0 validate ../swagger.yaml
//go:generate rm -rf ../../client
//go:generate go run github.com/go-swagger/go-swagger/cmd/swagger@v0.31.0 generate client --target=../../ --spec=../swagger.yaml

import (
	_ "expvar" // Register the expvar handlers
	"log/slog"
	"net/http"
	_ "net/http/pprof" //nolint:golangci-lint,gosec // Register the pprof handlers

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

func NewAPIRouter(han *controllers.APIController, authMiddleware, initMiddleware, urlsRequiredMiddleware, instanceMiddleware auth.Middleware, manageWebhooks bool) *mux.Router {
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

	//////////////////////////
	// Controller endpoints //
	//////////////////////////
	controllerRouter := apiSubRouter.PathPrefix("/controller").Subrouter()
	// The controller endpoints allow us to get information about the controller and update the URL endpoints.
	// This endpoint must not be guarded by the urlsRequiredMiddleware as that would prevent the user from
	// updating the URLs.
	controllerRouter.Use(initMiddleware.Middleware)
	controllerRouter.Use(authMiddleware.Middleware)
	controllerRouter.Use(auth.AdminRequiredMiddleware)
	// Get controller info
	controllerRouter.Handle("/", http.HandlerFunc(han.ControllerInfoHandler)).Methods("GET", "OPTIONS")
	controllerRouter.Handle("", http.HandlerFunc(han.ControllerInfoHandler)).Methods("GET", "OPTIONS")
	// Update controller
	controllerRouter.Handle("/", http.HandlerFunc(han.UpdateControllerHandler)).Methods("PUT", "OPTIONS")
	controllerRouter.Handle("", http.HandlerFunc(han.UpdateControllerHandler)).Methods("PUT", "OPTIONS")

	////////////////////////////////////
	// API router for everything else //
	////////////////////////////////////
	apiRouter := apiSubRouter.PathPrefix("").Subrouter()
	apiRouter.Use(initMiddleware.Middleware)
	// all endpoints except the controller endpoint should return an error
	// if the required metadata, callback and webhook URLs are not set.
	apiRouter.Use(urlsRequiredMiddleware.Middleware)
	apiRouter.Use(authMiddleware.Middleware)
	apiRouter.Use(auth.AdminRequiredMiddleware)

	// Legacy controller path
	apiRouter.Handle("/controller-info/", http.HandlerFunc(han.ControllerInfoHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/controller-info", http.HandlerFunc(han.ControllerInfoHandler)).Methods("GET", "OPTIONS")

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

	////////////////
	// Scale sets //
	////////////////
	// List all pools
	apiRouter.Handle("/scalesets/", http.HandlerFunc(han.ListAllScaleSetsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/scalesets", http.HandlerFunc(han.ListAllScaleSetsHandler)).Methods("GET", "OPTIONS")
	// Get one pool
	apiRouter.Handle("/scalesets/{scalesetID}/", http.HandlerFunc(han.GetScaleSetByIDHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/scalesets/{scalesetID}", http.HandlerFunc(han.GetScaleSetByIDHandler)).Methods("GET", "OPTIONS")
	// Delete one pool
	apiRouter.Handle("/scalesets/{scalesetID}/", http.HandlerFunc(han.DeleteScaleSetByIDHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/scalesets/{scalesetID}", http.HandlerFunc(han.DeleteScaleSetByIDHandler)).Methods("DELETE", "OPTIONS")
	// Update one pool
	apiRouter.Handle("/scalesets/{scalesetID}/", http.HandlerFunc(han.UpdateScaleSetByIDHandler)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/scalesets/{scalesetID}", http.HandlerFunc(han.UpdateScaleSetByIDHandler)).Methods("PUT", "OPTIONS")
	// List pool instances
	apiRouter.Handle("/scalesets/{scalesetID}/instances/", http.HandlerFunc(han.ListScaleSetInstancesHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/scalesets/{scalesetID}/instances", http.HandlerFunc(han.ListScaleSetInstancesHandler)).Methods("GET", "OPTIONS")

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
	apiRouter.Handle("/repositories/{owner}/{repo}/pools/{poolID}/", http.HandlerFunc(han.GetRepoByNamePoolHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", http.HandlerFunc(han.GetRepoPoolHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{owner}/{repo}/pools/{poolID}", http.HandlerFunc(han.GetRepoByNamePoolHandler)).Methods("GET", "OPTIONS")
	// Delete pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}/", http.HandlerFunc(han.DeleteRepoPoolHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/repositories/{owner}/{repo}/pools/{poolID}/", http.HandlerFunc(han.DeleteRepoByNamePoolHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", http.HandlerFunc(han.DeleteRepoPoolHandler)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/repositories/{owner}/{repo}/pools/{poolID}", http.HandlerFunc(han.DeleteRepoByNamePoolHandler)).Methods("DELETE", "OPTIONS")
	// Update pool
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}/", http.HandlerFunc(han.UpdateRepoPoolHandler)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/repositories/{owner}/{repo}/pools/{poolID}/", http.HandlerFunc(han.UpdateRepoByNamePoolHandler)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools/{poolID}", http.HandlerFunc(han.UpdateRepoPoolHandler)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/repositories/{owner}/{repo}/pools/{poolID}", http.HandlerFunc(han.UpdateRepoByNamePoolHandler)).Methods("PUT", "OPTIONS")
	// List pools
	apiRouter.Handle("/repositories/{repoID}/pools/", http.HandlerFunc(han.ListRepoPoolsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{owner}/{repo}/pools/", http.HandlerFunc(han.ListRepoByNamePoolsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools", http.HandlerFunc(han.ListRepoPoolsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{owner}/{repo}/pools", http.HandlerFunc(han.ListRepoByNamePoolsHandler)).Methods("GET", "OPTIONS")
	// Create pool
	apiRouter.Handle("/repositories/{repoID}/pools/", http.HandlerFunc(han.CreateRepoPoolHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories/{owner}/{repo}/pools/", http.HandlerFunc(han.CreateRepoByNamePoolHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/pools", http.HandlerFunc(han.CreateRepoPoolHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories/{owner}/{repo}/pools", http.HandlerFunc(han.CreateRepoByNamePoolHandler)).Methods("POST", "OPTIONS")

	// Create scale set
	apiRouter.Handle("/repositories/{repoID}/scalesets/", http.HandlerFunc(han.CreateRepoScaleSetHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/scalesets", http.HandlerFunc(han.CreateRepoScaleSetHandler)).Methods("POST", "OPTIONS")

	// List scale sets
	apiRouter.Handle("/repositories/{repoID}/scalesets/", http.HandlerFunc(han.ListRepoScaleSetsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/repositories/{repoID}/scalesets", http.HandlerFunc(han.ListRepoScaleSetsHandler)).Methods("GET", "OPTIONS")

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
		apiRouter.Handle("/repositories/{owner}/{repo}/webhook/", http.HandlerFunc(han.InstallRepoByNameWebhookHandler)).Methods("POST", "OPTIONS")
		apiRouter.Handle("/repositories/{repoID}/webhook", http.HandlerFunc(han.InstallRepoWebhookHandler)).Methods("POST", "OPTIONS")
		apiRouter.Handle("/repositories/{owner}/{repo}/webhook", http.HandlerFunc(han.InstallRepoByNameWebhookHandler)).Methods("POST", "OPTIONS")
		// Uninstall Webhook
		apiRouter.Handle("/repositories/{repoID}/webhook/", http.HandlerFunc(han.UninstallRepoWebhookHandler)).Methods("DELETE", "OPTIONS")
		apiRouter.Handle("/repositories/{owner}/{repo}/webhook/", http.HandlerFunc(han.UninstallRepoByNameWebhookHandler)).Methods("DELETE", "OPTIONS")
		apiRouter.Handle("/repositories/{repoID}/webhook", http.HandlerFunc(han.UninstallRepoWebhookHandler)).Methods("DELETE", "OPTIONS")
		apiRouter.Handle("/repositories/{owner}/{repo}/webhook", http.HandlerFunc(han.UninstallRepoByNameWebhookHandler)).Methods("DELETE", "OPTIONS")
		// Get webhook info
		apiRouter.Handle("/repositories/{repoID}/webhook/", http.HandlerFunc(han.GetRepoWebhookInfoHandler)).Methods("GET", "OPTIONS")
		apiRouter.Handle("/repositories/{owner}/{repo}/webhook/", http.HandlerFunc(han.GetRepoByNameWebhookInfoHandler)).Methods("GET", "OPTIONS")
		apiRouter.Handle("/repositories/{repoID}/webhook", http.HandlerFunc(han.GetRepoWebhookInfoHandler)).Methods("GET", "OPTIONS")
		apiRouter.Handle("/repositories/{owner}/{repo}/webhook", http.HandlerFunc(han.GetRepoByNameWebhookInfoHandler)).Methods("GET", "OPTIONS")
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

	// Create org scale set
	apiRouter.Handle("/organizations/{orgID}/scalesets/", http.HandlerFunc(han.CreateOrgScaleSetHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/scalesets", http.HandlerFunc(han.CreateOrgScaleSetHandler)).Methods("POST", "OPTIONS")

	// List org scale sets
	apiRouter.Handle("/organizations/{orgID}/scalesets/", http.HandlerFunc(han.ListOrgScaleSetsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/organizations/{orgID}/scalesets", http.HandlerFunc(han.ListOrgScaleSetsHandler)).Methods("GET", "OPTIONS")

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

	// Create enterprise scale sets
	apiRouter.Handle("/enterprises/{enterpriseID}/scalesets/", http.HandlerFunc(han.CreateEnterpriseScaleSetHandler)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/scalesets", http.HandlerFunc(han.CreateEnterpriseScaleSetHandler)).Methods("POST", "OPTIONS")

	// List enterprise scale sets
	apiRouter.Handle("/enterprises/{enterpriseID}/scalesets/", http.HandlerFunc(han.ListEnterpriseScaleSetsHandler)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/enterprises/{enterpriseID}/scalesets", http.HandlerFunc(han.ListEnterpriseScaleSetsHandler)).Methods("GET", "OPTIONS")

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

	// Providers
	apiRouter.Handle("/providers/", http.HandlerFunc(han.ListProviders)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/providers", http.HandlerFunc(han.ListProviders)).Methods("GET", "OPTIONS")

	//////////////////////
	// Github Endpoints //
	//////////////////////
	// Create Github Endpoint
	apiRouter.Handle("/github/endpoints/", http.HandlerFunc(han.CreateGithubEndpoint)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/github/endpoints", http.HandlerFunc(han.CreateGithubEndpoint)).Methods("POST", "OPTIONS")
	// List Github Endpoints
	apiRouter.Handle("/github/endpoints/", http.HandlerFunc(han.ListGithubEndpoints)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/github/endpoints", http.HandlerFunc(han.ListGithubEndpoints)).Methods("GET", "OPTIONS")
	// Get Github Endpoint
	apiRouter.Handle("/github/endpoints/{name}/", http.HandlerFunc(han.GetGithubEndpoint)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/github/endpoints/{name}", http.HandlerFunc(han.GetGithubEndpoint)).Methods("GET", "OPTIONS")
	// Delete Github Endpoint
	apiRouter.Handle("/github/endpoints/{name}/", http.HandlerFunc(han.DeleteGithubEndpoint)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/github/endpoints/{name}", http.HandlerFunc(han.DeleteGithubEndpoint)).Methods("DELETE", "OPTIONS")
	// Update Github Endpoint
	apiRouter.Handle("/github/endpoints/{name}/", http.HandlerFunc(han.UpdateGithubEndpoint)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/github/endpoints/{name}", http.HandlerFunc(han.UpdateGithubEndpoint)).Methods("PUT", "OPTIONS")

	////////////////////////
	// Github credentials //
	////////////////////////
	// Legacy credentials path
	apiRouter.Handle("/credentials/", http.HandlerFunc(han.ListCredentials)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/credentials", http.HandlerFunc(han.ListCredentials)).Methods("GET", "OPTIONS")
	// List Github Credentials
	apiRouter.Handle("/github/credentials/", http.HandlerFunc(han.ListCredentials)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/github/credentials", http.HandlerFunc(han.ListCredentials)).Methods("GET", "OPTIONS")
	// Create Github Credentials
	apiRouter.Handle("/github/credentials/", http.HandlerFunc(han.CreateGithubCredential)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/github/credentials", http.HandlerFunc(han.CreateGithubCredential)).Methods("POST", "OPTIONS")
	// Get Github Credential
	apiRouter.Handle("/github/credentials/{id}/", http.HandlerFunc(han.GetGithubCredential)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/github/credentials/{id}", http.HandlerFunc(han.GetGithubCredential)).Methods("GET", "OPTIONS")
	// Delete Github Credential
	apiRouter.Handle("/github/credentials/{id}/", http.HandlerFunc(han.DeleteGithubCredential)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/github/credentials/{id}", http.HandlerFunc(han.DeleteGithubCredential)).Methods("DELETE", "OPTIONS")
	// Update Github Credential
	apiRouter.Handle("/github/credentials/{id}/", http.HandlerFunc(han.UpdateGithubCredential)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/github/credentials/{id}", http.HandlerFunc(han.UpdateGithubCredential)).Methods("PUT", "OPTIONS")

	//////////////////////
	// Gitea Endpoints  //
	//////////////////////
	// Create Gitea Endpoint
	apiRouter.Handle("/gitea/endpoints/", http.HandlerFunc(han.CreateGiteaEndpoint)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/gitea/endpoints", http.HandlerFunc(han.CreateGiteaEndpoint)).Methods("POST", "OPTIONS")
	// List Gitea Endpoints
	apiRouter.Handle("/gitea/endpoints/", http.HandlerFunc(han.ListGiteaEndpoints)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/gitea/endpoints", http.HandlerFunc(han.ListGiteaEndpoints)).Methods("GET", "OPTIONS")
	// Get Gitea Endpoint
	apiRouter.Handle("/gitea/endpoints/{name}/", http.HandlerFunc(han.GetGiteaEndpoint)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/gitea/endpoints/{name}", http.HandlerFunc(han.GetGiteaEndpoint)).Methods("GET", "OPTIONS")
	// Delete Gitea Endpoint
	apiRouter.Handle("/gitea/endpoints/{name}/", http.HandlerFunc(han.DeleteGiteaEndpoint)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/gitea/endpoints/{name}", http.HandlerFunc(han.DeleteGiteaEndpoint)).Methods("DELETE", "OPTIONS")
	// Update Gitea Endpoint
	apiRouter.Handle("/gitea/endpoints/{name}/", http.HandlerFunc(han.UpdateGiteaEndpoint)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/gitea/endpoints/{name}", http.HandlerFunc(han.UpdateGiteaEndpoint)).Methods("PUT", "OPTIONS")

	////////////////////////
	// Gitea credentials  //
	////////////////////////
	// List Gitea Credentials
	apiRouter.Handle("/gitea/credentials/", http.HandlerFunc(han.ListGiteaCredentials)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/gitea/credentials", http.HandlerFunc(han.ListGiteaCredentials)).Methods("GET", "OPTIONS")
	// Create Gitea Credentials
	apiRouter.Handle("/gitea/credentials/", http.HandlerFunc(han.CreateGiteaCredential)).Methods("POST", "OPTIONS")
	apiRouter.Handle("/gitea/credentials", http.HandlerFunc(han.CreateGiteaCredential)).Methods("POST", "OPTIONS")
	// Get Gitea Credential
	apiRouter.Handle("/gitea/credentials/{id}/", http.HandlerFunc(han.GetGiteaCredential)).Methods("GET", "OPTIONS")
	apiRouter.Handle("/gitea/credentials/{id}", http.HandlerFunc(han.GetGiteaCredential)).Methods("GET", "OPTIONS")
	// Delete Gitea Credential
	apiRouter.Handle("/gitea/credentials/{id}/", http.HandlerFunc(han.DeleteGiteaCredential)).Methods("DELETE", "OPTIONS")
	apiRouter.Handle("/gitea/credentials/{id}", http.HandlerFunc(han.DeleteGiteaCredential)).Methods("DELETE", "OPTIONS")
	// Update Gitea Credential
	apiRouter.Handle("/gitea/credentials/{id}/", http.HandlerFunc(han.UpdateGiteaCredential)).Methods("PUT", "OPTIONS")
	apiRouter.Handle("/gitea/credentials/{id}", http.HandlerFunc(han.UpdateGiteaCredential)).Methods("PUT", "OPTIONS")

	/////////////////////////
	// Websocket endpoints //
	/////////////////////////
	// Legacy log websocket path
	apiRouter.Handle("/ws/", http.HandlerFunc(han.WSHandler)).Methods("GET")
	apiRouter.Handle("/ws", http.HandlerFunc(han.WSHandler)).Methods("GET")
	// Log websocket endpoint
	apiRouter.Handle("/ws/logs/", http.HandlerFunc(han.WSHandler)).Methods("GET")
	apiRouter.Handle("/ws/logs", http.HandlerFunc(han.WSHandler)).Methods("GET")
	// DB watcher websocket endpoint
	apiRouter.Handle("/ws/events/", http.HandlerFunc(han.EventsHandler)).Methods("GET")
	apiRouter.Handle("/ws/events", http.HandlerFunc(han.EventsHandler)).Methods("GET")

	// NotFound handler
	apiRouter.PathPrefix("/").HandlerFunc(han.NotFoundHandler).Methods("GET", "POST", "PUT", "DELETE", "OPTIONS")
	return router
}
