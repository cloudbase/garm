package main

// import (
// 	"context"
// 	"flag"
// 	"fmt"
// 	"log"
// 	"net"
// 	"net/http"
// 	"os/signal"

// 	"runner-manager/apiserver/controllers"
// 	"runner-manager/apiserver/routers"
// 	"runner-manager/config"
// 	"runner-manager/util"
// 	// "github.com/google/go-github/v43/github"
// 	// "golang.org/x/oauth2"
// 	// "gopkg.in/yaml.v3"
// )

// var (
// 	conf    = flag.String("config", config.DefaultConfigFilePath, "runner-manager config file")
// 	version = flag.Bool("version", false, "prints version")
// )

// var Version string

// // var token = "super secret token"

// func main() {
// 	flag.Parse()
// 	if *version {
// 		fmt.Println(Version)
// 		return
// 	}
// 	ctx, stop := signal.NotifyContext(context.Background(), signals...)
// 	defer stop()
// 	fmt.Println(ctx)

// 	cfg, err := config.NewConfig(*conf)
// 	if err != nil {
// 		log.Fatalf("Fetching config: %+v", err)
// 	}

// 	// ts := oauth2.StaticTokenSource(
// 	// 	&oauth2.Token{AccessToken: cfg.Github.OAuth2Token},
// 	// )

// 	// tc := oauth2.NewClient(ctx, ts)

// 	// ghClient := github.NewClient(tc)

// 	// // list all repositories for the authenticated user
// 	// repos, _, err := client.Repositories.List(ctx, "", nil)

// 	// fmt.Println(repos, err)

// 	logWriter, err := util.GetLoggingWriter(cfg)
// 	if err != nil {
// 		log.Fatalf("fetching log writer: %+v", err)
// 	}
// 	log.SetOutput(logWriter)

// 	controller, err := controllers.NewAPIController()
// 	if err != nil {
// 		log.Fatalf("failed to create controller: %+v", err)
// 	}

// 	router := routers.NewAPIRouter(controller, logWriter)

// 	tlsCfg, err := cfg.APIServer.APITLSConfig()
// 	if err != nil {
// 		log.Fatalf("failed to get TLS config: %q", err)
// 	}

// 	srv := &http.Server{
// 		Addr:      cfg.APIServer.BindAddress(),
// 		TLSConfig: tlsCfg,
// 		// Pass our instance of gorilla/mux in.
// 		Handler: router,
// 	}

// 	listener, err := net.Listen("tcp", srv.Addr)
// 	if err != nil {
// 		log.Fatalf("creating listener: %q", err)
// 	}

// 	go func() {
// 		if err := srv.Serve(listener); err != nil {
// 			log.Fatalf("Listening: %+v", err)
// 		}
// 	}()

// 	<-ctx.Done()

// 	// runner, err := runner.NewRunner(ctx, *cfg)
// 	// if err != nil {
// 	// 	log.Fatal(err)
// 	// }

// 	// fmt.Println(runner)
// 	// controllerID := "026d374d-6a8a-4241-8ed9-a246fff6762f"
// 	// provider, err := lxd.NewProvider(ctx, &cfg.Providers[0], controllerID)
// 	// if err != nil {
// 	// 	log.Fatal(err)
// 	// }

// 	// if err := provider.RemoveAllInstances(ctx); err != nil {
// 	// 	log.Fatal(err)
// 	// }

// 	// fmt.Println(provider)

// 	// if err := provider.DeleteInstance(ctx, "runner-manager-2fbe5354-be28-4e00-95a8-11479912368d"); err != nil {
// 	// 	log.Fatal(err)
// 	// }

// 	// instances, err := provider.ListInstances(ctx)

// 	// asJs, err := json.MarshalIndent(instances, "", "  ")
// 	// fmt.Println(string(asJs), err)

// 	// log.Print("Fetching tools")
// 	// tools, _, err := ghClient.Actions.ListRunnerApplicationDownloads(ctx, cfg.Repositories[0].Owner, cfg.Repositories[0].Name)
// 	// if err != nil {
// 	// 	log.Fatal(err)
// 	// }

// 	// toolsAsYaml, err := yaml.Marshal(tools)
// 	// if err != nil {
// 	// 	log.Fatal(err)
// 	// }
// 	// log.Printf("got tools:\n%s\n", string(toolsAsYaml))

// 	// log.Print("fetching runner token")
// 	// ghRunnerToken, _, err := ghClient.Actions.CreateRegistrationToken(ctx, cfg.Repositories[0].Owner, cfg.Repositories[0].Name)
// 	// if err != nil {
// 	// 	log.Fatal(err)
// 	// }
// 	// log.Printf("got token %v", ghRunnerToken)

// 	// bootstrapArgs := params.BootstrapInstance{
// 	// 	Tools:                   tools,
// 	// 	RepoURL:                 cfg.Repositories[0].String(),
// 	// 	GithubRunnerAccessToken: *ghRunnerToken.Token,
// 	// 	RunnerType:              cfg.Repositories[0].Pool.Runners[0].Name,
// 	// 	CallbackURL:             "",
// 	// 	InstanceToken:           "",
// 	// 	SSHKeys: []string{
// 	// 		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQC2oT7j/+elHY9U2ibgk2RYJgCvqIwewYKJTtHslTQFDWlHLeDam93BBOFlQJm9/wKX/qjC8d26qyzjeeeVf2EEAztp+jQfEq9OU+EtgQUi589jxtVmaWuYED8KVNbzLuP79SrBtEZD4xqgmnNotPhRshh3L6eYj4XzLWDUuOD6kzNdsJA2QOKeMOIFpBN6urKJHRHYD+oUPUX1w5QMv1W1Srlffl4m5uE+0eJYAMr02980PG4+jS4bzM170wYdWwUI0pSZsEDC8Fn7jef6QARU2CgHJYlaTem+KWSXislOUTaCpR0uhakP1ezebW20yuuc3bdRNgSlZi9B7zAPALGZpOshVqwF+KmLDi6XiFwG+NnwAFa6zaQfhOxhw/rF5Jk/wVjHIHkNNvYewycZPbKui0E3QrdVtR908N3VsPtLhMQ59BEMl3xlURSi0fiOU3UjnwmOkOoFDy/WT8qk//gFD93tUxlf4eKXDgNfME3zNz8nVi2uCPvG5NT/P/VWR8NMqW6tZcmWyswM/GgL6Y84JQ3ESZq/7WvAetdc1gVIDQJ2ejYbSHBcQpWvkocsiuMTCwiEvQ0sr+UE5jmecQvLPUyXOhuMhw43CwxnLk1ZSeYeCorxbskyqIXH71o8zhbPoPiEbwgB+i9WEoq02u7c8CmCmO8Y9aOnh8MzTKxIgQ==",
// 	// 	},
// 	// }

// 	// instance, err := provider.CreateInstance(ctx, bootstrapArgs)
// 	// if err != nil {
// 	// 	log.Fatal(err)
// 	// }

// 	// fmt.Println(instance)
// }
