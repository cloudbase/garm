package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"

	"runner-manager/apiserver/controllers"
	"runner-manager/apiserver/routers"
	"runner-manager/auth"
	"runner-manager/config"
	"runner-manager/database"
	"runner-manager/database/common"
	"runner-manager/runner"
	"runner-manager/util"

	"github.com/pkg/errors"
	// "github.com/google/go-github/v43/github"
	// "golang.org/x/oauth2"
	// "gopkg.in/yaml.v3"
)

var (
	conf    = flag.String("config", config.DefaultConfigFilePath, "runner-manager config file")
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

	jwtMiddleware, err := auth.NewjwtMiddleware(db, cfg.JWTAuth)
	if err != nil {
		log.Fatal(err)
	}

	initMiddleware, err := auth.NewInitRequiredMiddleware(db)
	if err != nil {
		log.Fatal(err)
	}

	router := routers.NewAPIRouter(controller, logWriter, jwtMiddleware, initMiddleware)

	tlsCfg, err := cfg.APIServer.APITLSConfig()
	if err != nil {
		log.Fatalf("failed to get TLS config: %q", err)
	}

	srv := &http.Server{
		Addr:      cfg.APIServer.BindAddress(),
		TLSConfig: tlsCfg,
		// Pass our instance of gorilla/mux in.
		Handler: router,
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
