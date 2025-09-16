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

package main

import (
	"context"
	"flag"
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
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/locking"
	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner" //nolint:typecheck
	runnerMetrics "github.com/cloudbase/garm/runner/metrics"
	"github.com/cloudbase/garm/runner/providers"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/util/appdefaults"
	"github.com/cloudbase/garm/websocket"
	"github.com/cloudbase/garm/workers/cache"
	"github.com/cloudbase/garm/workers/entity"
	"github.com/cloudbase/garm/workers/provider"
	"github.com/cloudbase/garm/workers/websocket/agent"
)

var (
	conf    = flag.String("config", appdefaults.DefaultConfigFilePath, "garm config file")
	version = flag.Bool("version", false, "prints version")
)

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
}

func maybeInitController(db common.Store) (params.ControllerInfo, error) {
	if info, err := db.ControllerInfo(); err == nil {
		return info, nil
	}

	info, err := db.InitController()
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error initializing controller: %w", err)
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

	handlers := []slog.Handler{
		fileHan,
	}

	if hub != nil {
		wsHan := slog.NewJSONHandler(hub, &opts)
		handlers = append(handlers, wsHan)
	}

	wrapped := &garmUtil.SlogMultiHandler{
		Handlers: handlers,
	}
	slog.SetDefault(slog.New(wrapped))
}

func maybeUpdateURLsFromConfig(cfg config.Config, store common.Store) error {
	info, err := store.ControllerInfo()
	if err != nil {
		return fmt.Errorf("error fetching controller info: %w", err)
	}

	var updateParams params.UpdateControllerParams

	if info.MetadataURL == "" && cfg.Default.MetadataURL != "" {
		updateParams.MetadataURL = &cfg.Default.MetadataURL
	}

	if info.CallbackURL == "" && cfg.Default.CallbackURL != "" {
		updateParams.CallbackURL = &cfg.Default.CallbackURL
	}

	if info.WebhookURL == "" && cfg.Default.WebhookURL != "" {
		updateParams.WebhookURL = &cfg.Default.WebhookURL
	}

	if updateParams.MetadataURL == nil && updateParams.CallbackURL == nil && updateParams.WebhookURL == nil {
		// nothing to update
		return nil
	}

	_, err = store.UpdateController(updateParams)
	if err != nil {
		return fmt.Errorf("error updating controller info: %w", err)
	}
	return nil
}

//gocyclo:ignore
func main() {
	flag.Parse()
	if *version {
		fmt.Println(appdefaults.GetVersion())
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()
	watcher.InitWatcher(ctx)

	ctx = auth.GetAdminContext(ctx)

	cfg, err := config.NewConfig(*conf)
	if err != nil {
		log.Fatalf("Fetching config: %+v", err) //nolint:gocritic
	}

	logCfg := cfg.GetLoggingConfig()
	var hub *websocket.Hub
	if logCfg.EnableLogStreamer != nil && *logCfg.EnableLogStreamer {
		hub = websocket.NewHub(ctx)
		if err := hub.Start(); err != nil {
			log.Fatal(err)
		}
		defer hub.Stop() //nolint
	}
	setupLogging(ctx, logCfg, hub)

	// Migrate credentials to the new format. This field will be read
	// by the DB migration logic.
	cfg.Database.MigrateCredentials = cfg.Github
	db, err := database.NewDatabase(ctx, cfg.Database)
	if err != nil {
		log.Fatal(err)
	}

	controllerInfo, err := maybeInitController(db)
	if err != nil {
		log.Fatal(err)
	}

	agentHub, err := agent.NewHub(ctx)
	if err != nil {
		log.Fatalf("failed to create agent hub: %q", err)
	}

	if err := agentHub.Start(); err != nil {
		log.Fatalf("failed to start agent hub: %q", err)
	}

	// Local locker for now. Will be configurable in the future,
	// as we add scale-out capability to GARM.
	lock, err := locking.NewLocalLocker(ctx, db)
	if err != nil {
		log.Fatalf("failed to create locker: %q", err)
	}

	if err := locking.RegisterLocker(lock); err != nil {
		log.Fatalf("failed to register locker: %q", err)
	}

	if err := maybeUpdateURLsFromConfig(*cfg, db); err != nil {
		log.Fatal(err)
	}

	cacheWorker := cache.NewWorker(ctx, db)
	if err := cacheWorker.Start(); err != nil {
		log.Fatalf("failed to start cache worker: %+v", err)
	}

	providers, err := providers.LoadProvidersFromConfig(ctx, *cfg, controllerInfo.ControllerID.String())
	if err != nil {
		log.Fatalf("loading providers: %+v", err)
	}

	entityController, err := entity.NewController(ctx, db, providers)
	if err != nil {
		log.Fatalf("failed to create entity controller: %+v", err)
	}
	if err := entityController.Start(); err != nil {
		log.Fatalf("failed to start entity controller: %+v", err)
	}

	instanceTokenGetter, err := auth.NewInstanceTokenGetter(cfg.JWTAuth.Secret)
	if err != nil {
		log.Fatalf("failed to create instance token getter: %+v", err)
	}

	providerWorker, err := provider.NewWorker(ctx, db, providers, instanceTokenGetter)
	if err != nil {
		log.Fatalf("failed to create provider worker: %+v", err)
	}
	if err := providerWorker.Start(); err != nil {
		log.Fatalf("failed to start provider worker: %+v", err)
	}

	runner, err := runner.NewRunner(ctx, *cfg, db, instanceTokenGetter)
	if err != nil {
		log.Fatalf("failed to create controller: %+v", err)
	}

	// If there are many repos/pools, this may take a long time.
	if err := runner.Start(); err != nil {
		log.Fatal(err)
	}

	authenticator := auth.NewAuthenticator(cfg.JWTAuth, db)
	controller, err := controllers.NewAPIController(runner, authenticator, hub, agentHub, cfg.APIServer)
	if err != nil {
		log.Fatalf("failed to create controller: %+v", err)
	}

	instanceMiddleware, err := auth.NewInstanceMiddleware(db, cfg.JWTAuth)
	if err != nil {
		log.Fatal(err)
	}

	jwtMiddleware, err := auth.NewjwtMiddleware(db, cfg.JWTAuth)
	if err != nil {
		log.Fatal(err)
	}

	initMiddleware, err := auth.NewInitRequiredMiddleware(db)
	if err != nil {
		log.Fatal(err)
	}

	urlsRequiredMiddleware, err := auth.NewUrlsRequiredMiddleware(db)
	if err != nil {
		log.Fatal(err)
	}

	metricsMiddleware, err := auth.NewMetricsMiddleware(cfg.JWTAuth)
	if err != nil {
		log.Fatal(err)
	}
	agentMiddleware, err := auth.AgentMiddleware(db, cfg.JWTAuth)
	if err != nil {
		log.Fatal(err)
	}

	router := routers.NewAPIRouter(controller, jwtMiddleware, initMiddleware, urlsRequiredMiddleware, instanceMiddleware, cfg.Default.EnableWebhookManagement)

	// Add WebUI routes
	router = routers.WithWebUI(router, cfg.APIServer)
	router = routers.WithAgentRouter(router, controller, agentMiddleware)

	// start the metrics collector
	if cfg.Metrics.Enable {
		slog.InfoContext(ctx, "setting up metric routes")
		router = routers.WithMetricsRouter(router, cfg.Metrics.DisableAuth, metricsMiddleware)

		slog.InfoContext(ctx, "register metrics")
		if err := metrics.RegisterMetrics(); err != nil {
			log.Fatal(err)
		}

		slog.InfoContext(ctx, "start metrics collection")
		runnerMetrics.CollectObjectMetric(ctx, runner, cfg.Metrics.Duration())
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
		log.Fatalf("creating listener: %q", err)
	}

	go func() {
		if cfg.APIServer.UseTLS {
			if err := srv.ServeTLS(listener, cfg.APIServer.TLSConfig.CRT, cfg.APIServer.TLSConfig.Key); err != nil {
				slog.With(slog.Any("error", err)).ErrorContext(ctx, "Listening")
			}
		} else {
			if err := srv.Serve(listener); err != http.ErrServerClosed {
				slog.With(slog.Any("error", err)).ErrorContext(ctx, "Listening")
			}
		}
	}()

	<-ctx.Done()

	slog.InfoContext(ctx, "shutting down http server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "graceful api server shutdown failed")
	}

	if err := cacheWorker.Stop(); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to stop credentials worker")
	}

	slog.InfoContext(ctx, "shutting down entity controller")
	if err := entityController.Stop(); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to stop entity controller")
	}

	slog.InfoContext(ctx, "shutting down provider worker")
	if err := providerWorker.Stop(); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to stop provider worker")
	}

	slog.With(slog.Any("error", err)).InfoContext(ctx, "waiting for runner to stop")
	if err := runner.Wait(); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to shutdown workers")
		os.Exit(1)
	}
}
