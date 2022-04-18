package lxd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"runner-manager/cloudconfig"
	"runner-manager/config"
	runnerErrors "runner-manager/errors"
	"runner-manager/params"
	"runner-manager/runner/common"
	"runner-manager/util"

	"github.com/google/go-github/v43/github"
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
)

var _ common.Provider = &LXD{}

var (
	archMap map[string]string = map[string]string{
		"x86_64": "x64",
		"amd64":  "x64",
	}
)

const (
	DefaultProjectDescription = "This project was created automatically by runner-manager to be used for github ephemeral action runners."
	DefaultProjectName        = "runner-manager-project"
)

func getClientFromConfig(ctx context.Context, cfg *config.LXD) (cli lxd.InstanceServer, err error) {
	if cfg.UnixSocket != "" {
		cli, err = lxd.ConnectLXDUnixWithContext(ctx, cfg.UnixSocket, nil)
	} else {
		connectArgs := lxd.ConnectionArgs{
			TLSServerCert: cfg.TLSServerCert,
			TLSCA:         cfg.TLSCA,
			TLSClientCert: cfg.ClientCertificate,
			TLSClientKey:  cfg.ClientKey,
		}
		cli, err = lxd.ConnectLXD(cfg.URL, &connectArgs)
	}

	if err != nil {
		return nil, errors.Wrap(err, "connecting to LXD")
	}

	return cli, nil
}

func projectName(cfg config.LXD) string {
	if cfg.ProjectName != "" {
		return cfg.ProjectName
	}
	return DefaultProjectName
}

func NewProvider(ctx context.Context, cfg *config.Provider, pool *config.Pool) (common.Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating provider config")
	}
	if err := pool.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating pool")
	}

	if cfg.ProviderType != config.LXDProvider {
		return nil, fmt.Errorf("invalid provider type %s, expected %s", cfg.ProviderType, config.LXDProvider)
	}

	if cfg.Name != pool.ProviderName {
		return nil, fmt.Errorf("provider %s is not responsible for pool", cfg.Name)
	}

	cli, err := getClientFromConfig(ctx, &cfg.LXD)
	if err != nil {
		return nil, errors.Wrap(err, "creating LXD client")
	}

	_, _, err = cli.GetProject(projectName(cfg.LXD))
	if err != nil {
		return nil, errors.Wrapf(err, "fetching project name: %s", projectName(cfg.LXD))
	}
	cli = cli.UseProject(projectName(cfg.LXD))

	provider := &LXD{
		ctx:  ctx,
		cfg:  cfg,
		pool: pool,
		cli:  cli,
	}

	return provider, nil
}

type LXD struct {
	// cfg is the provider config for this provider.
	cfg *config.Provider
	// pool holds the config for the pool this provider is
	// responsible for.
	pool *config.Pool
	// ctx is the context.
	ctx context.Context
	// cli is the LXD client.
	cli lxd.InstanceServer
}

func (l *LXD) getProfiles(runner config.Runner) ([]string, error) {
	ret := []string{}
	if l.cfg.LXD.IncludeDefaultProfile {
		ret = append(ret, "default")
	}

	set := map[string]struct{}{}

	profiles, err := l.cli.GetProfileNames()
	if err != nil {
		return nil, errors.Wrap(err, "fetching profile names")
	}
	for _, profile := range profiles {
		set[profile] = struct{}{}
	}

	if _, ok := set[runner.Flavor]; !ok {
		return nil, errors.Wrapf(runnerErrors.ErrNotFound, "looking for profile %s", runner.Flavor)
	}

	ret = append(ret, runner.Flavor)
	return ret, nil
}

// TODO: Add image details cache. Avoid doing a request if not necessary.
func (l *LXD) getImageDetails(runner config.Runner) (*api.Image, error) {
	alias, _, err := l.cli.GetImageAlias(runner.Image)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving alias: %s", runner.Image)
	}

	image, _, err := l.cli.GetImage(alias.Target)
	if err != nil {
		return nil, errors.Wrap(err, "fetching image details")
	}
	return image, nil
}

func (l *LXD) getCloudConfig(runner config.Runner, bootstrapParams params.BootstrapInstance, tools github.RunnerApplicationDownload, runnerName string) (string, error) {
	cloudCfg := cloudconfig.NewDefaultCloudInitConfig()

	installRunnerParams := cloudconfig.InstallRunnerParams{
		FileName:       *tools.Filename,
		DownloadURL:    *tools.DownloadURL,
		GithubToken:    bootstrapParams.GithubRunnerAccessToken,
		RunnerUsername: config.DefaultUser,
		RunnerGroup:    config.DefaultUser,
		RepoURL:        bootstrapParams.RepoURL,
		RunnerName:     runnerName,
		RunnerLabels:   strings.Join(runner.Labels, ","),
	}

	installScript, err := cloudconfig.InstallRunnerScript(installRunnerParams)
	if err != nil {
		return "", errors.Wrap(err, "generating script")
	}

	cloudCfg.AddSSHKey(bootstrapParams.SSHKeys...)
	cloudCfg.AddFile(installScript, "/var/run/install_runner.sh", "root:root", "755")
	cloudCfg.AddRunCmd("/var/run/install_runner.sh")

	asStr, err := cloudCfg.Serialize()
	if err != nil {
		return "", errors.Wrap(err, "creating cloud config")
	}
	return asStr, nil
}

func (l *LXD) getTools(image *api.Image, tools []*github.RunnerApplicationDownload) (github.RunnerApplicationDownload, error) {
	if image == nil {
		return github.RunnerApplicationDownload{}, fmt.Errorf("nil image received")
	}
	osName, ok := image.ImagePut.Properties["os"]
	if !ok {
		return github.RunnerApplicationDownload{}, fmt.Errorf("missing OS info in image properties")
	}

	osType, err := util.OSToOSType(osName)
	if err != nil {
		return github.RunnerApplicationDownload{}, errors.Wrap(err, "fetching OS type")
	}

	switch osType {
	case config.Linux:
	default:
		return github.RunnerApplicationDownload{}, fmt.Errorf("this provider does not support OS type: %s", osType)
	}

	for _, tool := range tools {
		if tool == nil {
			continue
		}
		if tool.OS == nil || tool.Architecture == nil {
			continue
		}

		fmt.Println(*tool.Architecture, *tool.OS)
		fmt.Printf("image arch: %s --> osType: %s\n", image.Architecture, string(osType))
		if *tool.Architecture == image.Architecture && *tool.OS == string(osType) {
			return *tool, nil
		}

		arch, ok := archMap[image.Architecture]
		if ok && arch == *tool.Architecture {
			return *tool, nil
		}
	}
	return github.RunnerApplicationDownload{}, fmt.Errorf("failed to find tools for OS %s and arch %s", osType, image.Architecture)
}

func (l *LXD) getCreateInstanceArgs(bootstrapParams params.BootstrapInstance) (api.InstancesPost, error) {
	name := fmt.Sprintf("runner-manager-%s", uuid.New())
	runner, err := util.FindRunnerType(bootstrapParams.RunnerType, l.pool.Runners)

	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "fetching runner")
	}

	profiles, err := l.getProfiles(runner)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "fetching profiles")
	}

	image, err := l.getImageDetails(runner)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "getting image details")
	}

	tools, err := l.getTools(image, bootstrapParams.Tools)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "getting tools")
	}

	cloudCfg, err := l.getCloudConfig(runner, bootstrapParams, tools, name)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "generating cloud-config")
	}

	args := api.InstancesPost{
		InstancePut: api.InstancePut{
			Architecture: image.Architecture,
			Profiles:     profiles,
			Description:  "Github runner provisioned by runner-manager",
			Config: map[string]string{
				"user.user-data": cloudCfg,
			},
		},
		Source: api.InstanceSource{
			Type:  "image",
			Alias: runner.Image,
		},
		Name: name,
		Type: api.InstanceTypeVM,
	}
	return args, nil
}

// CreateInstance creates a new compute instance in the provider.
func (l *LXD) CreateInstance(ctx context.Context, bootstrapParams params.BootstrapInstance) error {
	args, err := l.getCreateInstanceArgs(bootstrapParams)
	if err != nil {
		return errors.Wrap(err, "fetching create args")
	}

	asJs, err := json.MarshalIndent(args, "", "  ")
	fmt.Println(string(asJs), err)
	return nil
}

// Delete instance will delete the instance in a provider.
func (l *LXD) DeleteInstance(ctx context.Context, instance string) error {
	return nil
}

// ListInstances will list all instances for a provider.
func (l *LXD) ListInstances(ctx context.Context) error {
	return nil
}

// RemoveAllInstances will remove all instances created by this provider.
func (l *LXD) RemoveAllInstances(ctx context.Context) error {
	return nil
}

// Status returns the instance status.
func (l *LXD) Status(ctx context.Context, instance string) error {
	return nil
}

// Stop shuts down the instance.
func (l *LXD) Stop(ctx context.Context, instance string) error {
	return nil
}

// Start boots up an instance.
func (l *LXD) Start(ctx context.Context, instance string) error {
	return nil
}
