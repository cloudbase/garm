package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"

	"runner-manager/config"
	"runner-manager/runner"
	"runner-manager/util"
)

var (
	conf    = flag.String("config", config.DefaultConfigFilePath, "runner-manager config file")
	version = flag.Bool("version", false, "prints version")
)

var Version string

// var token = "super secret token"

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
		log.Fatal(err)
	}

	logWriter, err := util.GetLoggingWriter(cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(logWriter)

	runnerWorker, err := runner.NewRunner(ctx, cfg)

	fmt.Println(runnerWorker)

	// ctx := context.Background()
	// ts := oauth2.StaticTokenSource(
	// 	&oauth2.Token{AccessToken: token},
	// )

	// tc := oauth2.NewClient(ctx, ts)

	// client := github.NewClient(tc)

	// // list all repositories for the authenticated user
	// repos, _, err := client.Repositories.List(ctx, "", nil)

	// fmt.Println(repos, err)
}
