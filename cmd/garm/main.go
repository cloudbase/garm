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
	"net"
	"net/http"
	"os/signal"

	"garm/apiserver/controllers"
	"garm/apiserver/routers"
	"garm/auth"
	"garm/config"
	"garm/database"
	"garm/database/common"
	"garm/runner"
	"garm/util"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

var (
	conf    = flag.String("config", config.DefaultConfigFilePath, "garm config file")
	version = flag.Bool("version", false, "prints version")
)

var Version string

func maybeInitController(db common.Store) error {
	if _, err := db.ControllerInfo(); err == nil {
		return nil
	}

	if _, err := db.InitController(); err != nil {
		return errors.Wrap(err, "initializing controller")
	}

	return nil
}

func main() {
	flag.Parse()
	if *version {
		fmt.Println(Version)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()
	fmt.Println(ctx)

	cfg, err := config.NewConfig(*conf)
	if err != nil {
		log.Fatalf("Fetching config: %+v", err)
	}

	logWriter, err := util.GetLoggingWriter(cfg)
	if err != nil {
		log.Fatalf("fetching log writer: %+v", err)
	}
	log.SetOutput(logWriter)

	db, err := database.NewDatabase(ctx, cfg.Database)
	if err != nil {
		log.Fatal(err)
	}

	if err := maybeInitController(db); err != nil {
		log.Fatal(err)
	}

	runner, err := runner.NewRunner(ctx, *cfg)
	if err != nil {
		log.Fatalf("failed to create controller: %+v", err)
	}

	// If there are many repos/pools, this may take a long time.
	// TODO: start pool managers in the background and log errors.
	if err := runner.Start(); err != nil {
		log.Fatal(err)
	}

	authenticator := auth.NewAuthenticator(cfg.JWTAuth, db)
	controller, err := controllers.NewAPIController(runner, authenticator)
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

	router := routers.NewAPIRouter(controller, logWriter, jwtMiddleware, initMiddleware, instanceMiddleware)
	corsMw := mux.CORSMethodMiddleware(router)
	router.Use(corsMw)

	allowedOrigins := handlers.AllowedOrigins(cfg.APIServer.CORSOrigins)
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS", "DELETE"})
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"})

	tlsCfg, err := cfg.APIServer.APITLSConfig()
	if err != nil {
		log.Fatalf("failed to get TLS config: %q", err)
	}

	srv := &http.Server{
		Addr:      cfg.APIServer.BindAddress(),
		TLSConfig: tlsCfg,
		// Pass our instance of gorilla/mux in.
		Handler: handlers.CORS(methodsOk, headersOk, allowedOrigins)(router),
	}

	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		log.Fatalf("creating listener: %q", err)
	}

	go func() {
		if err := srv.Serve(listener); err != nil {
			log.Fatalf("Listening: %+v", err)
		}
	}()

	<-ctx.Done()
}
