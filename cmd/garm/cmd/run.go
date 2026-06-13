// Copyright 2026 Cloudbase Solutions SRL
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

package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/apiserver/controllers"
	"github.com/cloudbase/garm/apiserver/routers"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/locking"
	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner"
	runnerMetrics "github.com/cloudbase/garm/runner/metrics"
	"github.com/cloudbase/garm/runner/providers"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/websocket"
	"github.com/cloudbase/garm/workers/cache"
	"github.com/cloudbase/garm/workers/entity"
	"github.com/cloudbase/garm/workers/provider"
	"github.com/cloudbase/garm/workers/websocket/agent"
	wsMetrics "github.com/cloudbase/garm/workers/websocket/metrics"
)

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
}

// serverComponents holds initialized server components that require
// lifecycle management during shutdown.
type serverComponents struct {
	db             dbCommon.Store
	hub            *websocket.Hub
	agentHub       *agent.Hub
	metricsHub     *wsMetrics.MetricsHub
	cacheWorker    *cache.Worker
	providerWorker *provider.Provider
	entityCtrl     *entity.Controller
	runner         *runner.Runner
}

func runServer() error {
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()
	watcher.InitWatcher(ctx)

	ctx = auth.GetAdminContext(ctx)

	cfg, err := config.NewConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("fetching config: %w", err)
	}

	logCfg := cfg.GetLoggingConfig()
	var hub *websocket.Hub
	if logCfg.EnableLogStreamer != nil && *logCfg.EnableLogStreamer {
		hub = websocket.NewHub(ctx)
		if err := hub.Start(); err != nil {
			return fmt.Errorf("starting log streamer: %w", err)
		}
		defer hub.Stop() //nolint
	}
	setupLogging(ctx, logCfg, hub)

	comp, err := initInfrastructure(ctx, cfg, hub)
	if err != nil {
		return err
	}

	srv, listener, err := buildHTTPServer(ctx, cfg, comp)
	if err != nil {
		return err
	}

	go serve(ctx, srv, listener, cfg.APIServer)

	<-ctx.Done()

	shutdownServer(srv)
	shutdownComponents(comp)

	return nil
}

func initInfrastructure(ctx context.Context, cfg *config.Config, hub *websocket.Hub) (*serverComponents, error) {
	db, err := database.NewDatabase(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("creating database: %w", err)
	}

	controllerInfo, err := maybeInitController(db)
	if err != nil {
		return nil, err
	}

	agentHub, err := agent.NewHub(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating agent hub: %w", err)
	}
	if err := agentHub.Start(); err != nil {
		return nil, fmt.Errorf("starting agent hub: %w", err)
	}

	// Local locker for now. Will be configurable in the future,
	// as we add scale-out capability to GARM.
	lock, err := locking.NewLocalLocker(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("creating locker: %w", err)
	}
	if err := locking.RegisterLocker(lock); err != nil {
		return nil, fmt.Errorf("registering locker: %w", err)
	}

	instanceTokenGetter, err := auth.NewInstanceTokenGetter(cfg.JWTAuth.Secret)
	if err != nil {
		return nil, fmt.Errorf("creating instance token getter: %w", err)
	}

	rnr, err := runner.NewRunner(ctx, *cfg, db, instanceTokenGetter)
	if err != nil {
		return nil, fmt.Errorf("creating runner: %w", err)
	}

	cacheWorker := cache.NewWorker(ctx, db, rnr)
	if err := cacheWorker.Start(); err != nil {
		return nil, fmt.Errorf("starting cache worker: %w", err)
	}

	metricsHub := wsMetrics.NewMetricsHub(ctx)
	if err := metricsHub.Start(); err != nil {
		return nil, fmt.Errorf("starting metrics hub: %w", err)
	}

	loadedProviders, err := providers.LoadProvidersFromConfig(ctx, *cfg, controllerInfo.ControllerID.String())
	if err != nil {
		return nil, fmt.Errorf("loading providers: %w", err)
	}

	// Provider worker must start first — its watcher consumer must be
	// registered before entity/scaleset workers create instances.
	providerWorker, err := provider.NewWorker(ctx, db, loadedProviders, instanceTokenGetter)
	if err != nil {
		return nil, fmt.Errorf("creating provider worker: %w", err)
	}
	if err := providerWorker.Start(); err != nil {
		return nil, fmt.Errorf("starting provider worker: %w", err)
	}

	entityCtrl, err := entity.NewController(ctx, db, loadedProviders)
	if err != nil {
		return nil, fmt.Errorf("creating entity controller: %w", err)
	}
	if err := entityCtrl.Start(); err != nil {
		return nil, fmt.Errorf("starting entity controller: %w", err)
	}

	// If there are many repos/pools, this may take a long time.
	if err := rnr.Start(); err != nil {
		return nil, fmt.Errorf("starting runner: %w", err)
	}

	return &serverComponents{
		db:             db,
		hub:            hub,
		agentHub:       agentHub,
		metricsHub:     metricsHub,
		cacheWorker:    cacheWorker,
		providerWorker: providerWorker,
		entityCtrl:     entityCtrl,
		runner:         rnr,
	}, nil
}

func buildHTTPServer(ctx context.Context, cfg *config.Config, comp *serverComponents) (*http.Server, net.Listener, error) {
	authenticator := auth.NewAuthenticator(cfg.JWTAuth, comp.db)
	controller, err := controllers.NewAPIController(comp.runner, authenticator, comp.hub, comp.agentHub, comp.metricsHub, cfg.APIServer)
	if err != nil {
		return nil, nil, fmt.Errorf("creating API controller: %w", err)
	}

	instanceMiddleware, err := auth.NewInstanceMiddleware(comp.db, cfg.JWTAuth)
	if err != nil {
		return nil, nil, fmt.Errorf("creating instance middleware: %w", err)
	}

	jwtMiddleware, err := auth.NewjwtMiddleware(comp.db, cfg.JWTAuth)
	if err != nil {
		return nil, nil, fmt.Errorf("creating JWT middleware: %w", err)
	}

	initMiddleware, err := auth.NewInitRequiredMiddleware(comp.db)
	if err != nil {
		return nil, nil, fmt.Errorf("creating init-required middleware: %w", err)
	}

	urlsRequiredMiddleware, err := auth.NewUrlsRequiredMiddleware(comp.db)
	if err != nil {
		return nil, nil, fmt.Errorf("creating URLs-required middleware: %w", err)
	}

	metricsMiddleware, err := auth.NewMetricsMiddleware(cfg.JWTAuth)
	if err != nil {
		return nil, nil, fmt.Errorf("creating metrics middleware: %w", err)
	}

	agentMiddleware, err := auth.AgentMiddleware(comp.db, cfg.JWTAuth)
	if err != nil {
		return nil, nil, fmt.Errorf("creating agent middleware: %w", err)
	}

	router := routers.NewAPIRouter(
		controller,
		jwtMiddleware,
		initMiddleware,
		urlsRequiredMiddleware,
		instanceMiddleware,
		cfg.Default.EnableWebhookManagement)

	// Add WebUI routes
	router = routers.WithWebUI(router, cfg.APIServer)
	router = routers.WithAgentRouter(router, controller, agentMiddleware)

	// start the metrics collector
	if cfg.Metrics.Enable {
		slog.InfoContext(ctx, "setting up metric routes")
		router = routers.WithMetricsRouter(router, cfg.Metrics.DisableAuth, metricsMiddleware)

		slog.InfoContext(ctx, "register metrics")
		if err := metrics.RegisterMetrics(); err != nil {
			return nil, nil, fmt.Errorf("registering metrics: %w", err)
		}

		slog.InfoContext(ctx, "start metrics collection")
		runnerMetrics.CollectObjectMetric(ctx, comp.runner, cfg.Metrics.Duration())
	}

	if cfg.Default.DebugServer {
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(1)
		slog.InfoContext(ctx, "setting up debug routes")
		router = routers.WithDebugServer(router)
	}

	corsMw := mux.CORSMethodMiddleware(router)
	router.Use(corsMw)

	allowedOrigins := handlers.AllowedOrigins(cfg.APIServer.CORSOrigins)
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"})
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"})

	srv := &http.Server{
		Addr: cfg.APIServer.BindAddress(),
		// Pass our instance of gorilla/mux in.
		Handler:           handlers.CORS(methodsOk, headersOk, allowedOrigins)(router),
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
		// Increased timeouts to support large file uploads/downloads
		// ReadTimeout covers the entire request read including body
		ReadTimeout: 30 * time.Minute,
		// WriteTimeout covers the entire response write
		WriteTimeout: 30 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return nil, nil, fmt.Errorf("creating listener: %w", err)
	}

	return srv, listener, nil
}

func serve(ctx context.Context, srv *http.Server, listener net.Listener, apiCfg config.APIServer) {
	if apiCfg.UseTLS {
		if err := srv.ServeTLS(listener, apiCfg.TLSConfig.CRT, apiCfg.TLSConfig.Key); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "Listening")
		}
	} else {
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "Listening")
		}
	}
}

func shutdownServer(srv *http.Server) {
	slog.Info("shutting down http server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.With(slog.Any("error", err)).Error("graceful api server shutdown failed")
	}
}

// shutdownComponents stops all components in reverse startup order.
func shutdownComponents(comp *serverComponents) {
	slog.Info("waiting for runner to stop")
	if err := comp.runner.Wait(); err != nil {
		slog.With(slog.Any("error", err)).Error("failed to shutdown runner")
		os.Exit(1)
	}

	slog.Info("shutting down entity controller")
	if err := comp.entityCtrl.Stop(); err != nil {
		slog.With(slog.Any("error", err)).Error("failed to stop entity controller")
	}

	slog.Info("shutting down provider worker")
	if err := comp.providerWorker.Stop(); err != nil {
		slog.With(slog.Any("error", err)).Error("failed to stop provider worker")
	}

	comp.metricsHub.Stop() //nolint

	if err := comp.cacheWorker.Stop(); err != nil {
		slog.With(slog.Any("error", err)).Error("failed to stop cache worker")
	}
}

func maybeInitController(db dbCommon.Store) (params.ControllerInfo, error) {
	if info, err := db.ControllerInfo(); err == nil {
		return info, nil
	}

	info, err := db.InitController()
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("initializing controller: %w", err)
	}

	return info, nil
}

func setupLogging(ctx context.Context, logCfg config.Logging, hub *websocket.Hub) {
	logWriter, err := util.GetLoggingWriter(logCfg.LogFile)
	if err != nil {
		log.Fatalf("fetching log writer: %+v", err)
	}

	// rotate log file on SIGHUP
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-ctx.Done():
				// Daemon is exiting.
				return
			case <-ch:
				// we got a SIGHUP. Rotate log file.
				if logger, ok := logWriter.(*lumberjack.Logger); ok {
					if err := logger.Rotate(); err != nil {
						slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to rotate log file")
					}
				}
			}
		}
	}()

	var logLevel slog.Level
	switch logCfg.LogLevel {
	case config.LevelDebug:
		logLevel = slog.LevelDebug
	case config.LevelInfo:
		logLevel = slog.LevelInfo
	case config.LevelWarn:
		logLevel = slog.LevelWarn
	case config.LevelError:
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// logger options
	opts := slog.HandlerOptions{
		AddSource: logCfg.LogSource,
		Level:     logLevel,
	}

	var fileHan slog.Handler
	switch logCfg.LogFormat {
	case config.FormatJSON:
		fileHan = slog.NewJSONHandler(logWriter, &opts)
	default:
		fileHan = slog.NewTextHandler(logWriter, &opts)
	}

	slogHandlers := []slog.Handler{
		fileHan,
	}

	if hub != nil {
		wsHan := slog.NewJSONHandler(hub, &opts)
		slogHandlers = append(slogHandlers, wsHan)
	}

	wrapped := &garmUtil.SlogMultiHandler{
		Handlers: slogHandlers,
	}
	slog.SetDefault(slog.New(wrapped))
}
