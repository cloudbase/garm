package lxd

import (
	"context"
	"fmt"

	"runner-manager/config"
	runnerErrors "runner-manager/errors"
	"runner-manager/runner/common"
	"runner-manager/util"

	"github.com/google/go-github/v43/github"
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
)

var _ common.Provider = &LXD{}

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
		return nil, errors.Wrap(err, "fetching project name")
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

func (l *LXD) getProfiles(runnerType string) ([]string, error) {
	ret := []string{}
	if l.cfg.LXD.IncludeDefaultProfile {
		ret = append(ret, "default")
	}

	set := map[string]struct{}{}

	runner, err := util.FindRunner(runnerType, l.pool.Runners)
	if err != nil {
		return nil, errors.Wrapf(err, "finding runner of type %s", runnerType)
	}

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

func (l *LXD) getCloudConfig(runnerType string) (string, error) {
	return "", nil
}

func (l *LXD) getCreateInstanceArgs(runnerType string) (api.InstancesPost, error) {
	name := fmt.Sprintf("runner-manager-%s", uuid.New())
	profiles, err := l.getProfiles(runnerType)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "fetching profiles")
	}

	args := api.InstancesPost{
		InstancePut: api.InstancePut{
			Profiles:    profiles,
			Description: "Github runner provisioned by runner-manager",
		},
		Name: name,
		Type: api.InstanceTypeVM,
	}
	return args, nil
}

// CreateInstance creates a new compute instance in the provider.
func (l *LXD) CreateInstance(ctx context.Context, runnerType string, tools github.RunnerApplicationDownload) error {
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
