package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"

	"runner-manager/cloudconfig"
	"runner-manager/config"
	"runner-manager/params"
	"runner-manager/runner"
	"runner-manager/runner/providers/lxd"
	"runner-manager/util"

	"github.com/google/go-github/v43/github"
	"golang.org/x/oauth2"
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

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.Github.OAuth2Token},
	)

	tc := oauth2.NewClient(ctx, ts)

	ghClient := github.NewClient(tc)

	// // list all repositories for the authenticated user
	// repos, _, err := client.Repositories.List(ctx, "", nil)

	// fmt.Println(repos, err)

	logWriter, err := util.GetLoggingWriter(cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(logWriter)

	runnerWorker, err := runner.NewRunner(ctx, cfg)

	fmt.Println(runnerWorker)

	cloudCfg := cloudconfig.NewDefaultCloudInitConfig()
	cloudCfg.AddPackage("wget", "bmon", "wget")
	cloudCfg.AddFile(nil, "/home/runner/hi.txt", "runner:runner", "0755")
	asStr, err := cloudCfg.Serialize()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(asStr)

	runner, err := runner.NewRunner(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(runner)

	provider, err := lxd.NewProvider(ctx, &cfg.Providers[0], &cfg.Repositories[0].Pool)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(provider)

	log.Print("Fetching tools")
	tools, _, err := ghClient.Actions.ListRunnerApplicationDownloads(ctx, cfg.Repositories[0].Owner, cfg.Repositories[0].Name)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("got tools: %v", tools)

	log.Print("fetching runner token")
	ghRunnerToken, _, err := ghClient.Actions.CreateRegistrationToken(ctx, cfg.Repositories[0].Owner, cfg.Repositories[0].Name)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("got token %v", ghRunnerToken)

	bootstrapArgs := params.BootstrapInstance{
		Tools:                   tools,
		RepoURL:                 cfg.Repositories[0].String(),
		GithubRunnerAccessToken: *ghRunnerToken.Token,
		RunnerType:              cfg.Repositories[0].Pool.Runners[0].Name,
		CallbackURL:             "",
		InstanceToken:           "",
		SSHKeys: []string{
			"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQC2oT7j/+elHY9U2ibgk2RYJgCvqIwewYKJTtHslTQFDWlHLeDam93BBOFlQJm9/wKX/qjC8d26qyzjeeeVf2EEAztp+jQfEq9OU+EtgQUi589jxtVmaWuYED8KVNbzLuP79SrBtEZD4xqgmnNotPhRshh3L6eYj4XzLWDUuOD6kzNdsJA2QOKeMOIFpBN6urKJHRHYD+oUPUX1w5QMv1W1Srlffl4m5uE+0eJYAMr02980PG4+jS4bzM170wYdWwUI0pSZsEDC8Fn7jef6QARU2CgHJYlaTem+KWSXislOUTaCpR0uhakP1ezebW20yuuc3bdRNgSlZi9B7zAPALGZpOshVqwF+KmLDi6XiFwG+NnwAFa6zaQfhOxhw/rF5Jk/wVjHIHkNNvYewycZPbKui0E3QrdVtR908N3VsPtLhMQ59BEMl3xlURSi0fiOU3UjnwmOkOoFDy/WT8qk//gFD93tUxlf4eKXDgNfME3zNz8nVi2uCPvG5NT/P/VWR8NMqW6tZcmWyswM/GgL6Y84JQ3ESZq/7WvAetdc1gVIDQJ2ejYbSHBcQpWvkocsiuMTCwiEvQ0sr+UE5jmecQvLPUyXOhuMhw43CwxnLk1ZSeYeCorxbskyqIXH71o8zhbPoPiEbwgB+i9WEoq02u7c8CmCmO8Y9aOnh8MzTKxIgQ==",
		},
	}

	if err := provider.CreateInstance(ctx, bootstrapArgs); err != nil {
		log.Fatal(err)
	}
}
