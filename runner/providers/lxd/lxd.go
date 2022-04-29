package lxd

import (
	"context"
	"fmt"

	"runner-manager/config"
	runnerErrors "runner-manager/errors"
	"runner-manager/params"
	"runner-manager/runner/common"
	"runner-manager/util"

	"github.com/google/go-github/v43/github"
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var _ common.Provider = &LXD{}

const (
	// We look for this key in the config of the instances to determine if they are
	// created by us or not.
	controllerIDKeyName = "user.runner-controller-id"
	poolIDKey           = "user.runner-pool-id"
)

var (
	// lxdToGithubArchMap translates LXD architectures to Github tools architectures.
	// TODO: move this in a separate package. This will most likely be used
	// by any other provider.
	lxdToGithubArchMap map[string]string = map[string]string{
		"x86_64":  "x64",
		"amd64":   "x64",
		"armv7l":  "arm",
		"aarch64": "arm64",
		"x64":     "x64",
		"arm":     "arm",
		"arm64":   "arm64",
	}

	configToLXDArchMap map[config.OSArch]string = map[config.OSArch]string{
		config.Amd64: "x86_64",
		config.Arm64: "aarch64",
		config.Arm:   "armv7l",
	}

	lxdToConfigArch map[string]config.OSArch = map[string]config.OSArch{
		"x86_64":  config.Amd64,
		"aarch64": config.Arm64,
		"armv7l":  config.Arm,
	}
)

const (
	DefaultProjectDescription = "This project was created automatically by runner-manager to be used for github ephemeral action runners."
	DefaultProjectName        = "runner-manager-project"
)

func NewProvider(ctx context.Context, cfg *config.Provider, controllerID string) (common.Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating provider config")
	}

	if cfg.ProviderType != config.LXDProvider {
		return nil, fmt.Errorf("invalid provider type %s, expected %s", cfg.ProviderType, config.LXDProvider)
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
		ctx:          ctx,
		cfg:          cfg,
		cli:          cli,
		controllerID: controllerID,
		imageManager: &image{
			cli:     cli,
			remotes: cfg.LXD.ImageRemotes,
		},
	}

	return provider, nil
}

type LXD struct {
	// cfg is the provider config for this provider.
	cfg *config.Provider
	// ctx is the context.
	ctx context.Context
	// cli is the LXD client.
	cli lxd.InstanceServer
	// imageManager downloads images from remotes
	imageManager *image
	// controllerID is the ID of this controller
	controllerID string
}

func (l *LXD) getProfiles(flavor string) ([]string, error) {
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

	if _, ok := set[flavor]; !ok {
		return nil, errors.Wrapf(runnerErrors.ErrNotFound, "looking for profile %s", flavor)
	}

	ret = append(ret, flavor)
	return ret, nil
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

	// Validate image OS. Linux only for now.
	switch osType {
	case config.Linux:
	default:
		return github.RunnerApplicationDownload{}, fmt.Errorf("this provider does not support OS type: %s", osType)
	}

	// Find tools for OS/Arch.
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

		arch, ok := lxdToGithubArchMap[image.Architecture]
		if ok && arch == *tool.Architecture && *tool.OS == string(osType) {
			return *tool, nil
		}
	}
	return github.RunnerApplicationDownload{}, fmt.Errorf("failed to find tools for OS %s and arch %s", osType, image.Architecture)
}

// sadly, the security.secureboot flag is a string encoded boolean.
func (l *LXD) secureBootEnabled() string {
	if l.cfg.LXD.SecureBoot {
		return "true"
	}
	return "false"
}

func (l *LXD) getCreateInstanceArgs(bootstrapParams params.BootstrapInstance) (api.InstancesPost, error) {
	if bootstrapParams.Name == "" {
		return api.InstancesPost{}, runnerErrors.NewBadRequestError("missing name")
	}
	profiles, err := l.getProfiles(bootstrapParams.Flavor)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "fetching profiles")
	}

	arch, err := resolveArchitecture(bootstrapParams.OSArch)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "fetching archictecture")
	}

	image, err := l.imageManager.EnsureImage(bootstrapParams.Image, config.LXDImageVirtualMachine, arch)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "getting image details")
	}

	tools, err := l.getTools(image, bootstrapParams.Tools)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "getting tools")
	}

	cloudCfg, err := util.GetCloudConfig(bootstrapParams, tools, bootstrapParams.Name)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "generating cloud-config")
	}

	args := api.InstancesPost{
		InstancePut: api.InstancePut{
			Architecture: image.Architecture,
			Profiles:     profiles,
			Description:  "Github runner provisioned by runner-manager",
			Config: map[string]string{
				"user.user-data":      cloudCfg,
				"security.secureboot": l.secureBootEnabled(),
				controllerIDKeyName:   l.controllerID,
				poolIDKey:             bootstrapParams.PoolID,
			},
		},
		Source: api.InstanceSource{
			Type:        "image",
			Fingerprint: image.Fingerprint,
		},
		Name: bootstrapParams.Name,
		Type: api.InstanceTypeVM,
	}
	return args, nil
}

func (l *LXD) AsParams() params.Provider {
	return params.Provider{
		Name:         l.cfg.Name,
		ProviderType: l.cfg.ProviderType,
	}
}

func (l *LXD) launchInstance(createArgs api.InstancesPost) error {
	// Get LXD to create the instance (background operation)
	op, err := l.cli.CreateInstance(createArgs)
	if err != nil {
		return errors.Wrap(err, "creating instance")
	}

	// Wait for the operation to complete
	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "waiting for instance creation")
	}

	// Get LXD to start the instance (background operation)
	reqState := api.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
	}

	op, err = l.cli.UpdateInstanceState(createArgs.Name, reqState, "")
	if err != nil {
		return errors.Wrap(err, "starting instance")
	}

	// Wait for the operation to complete
	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "waiting for instance to start")
	}
	return nil
}

// CreateInstance creates a new compute instance in the provider.
func (l *LXD) CreateInstance(ctx context.Context, bootstrapParams params.BootstrapInstance) (params.Instance, error) {
	args, err := l.getCreateInstanceArgs(bootstrapParams)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching create args")
	}

	asJs, err := yaml.Marshal(args)
	fmt.Println(string(asJs), err)
	if err := l.launchInstance(args); err != nil {
		return params.Instance{}, errors.Wrap(err, "creating instance")
	}

	return l.GetInstance(ctx, args.Name)
}

// GetInstance will return details about one instance.
func (l *LXD) GetInstance(ctx context.Context, instanceName string) (params.Instance, error) {
	instance, _, err := l.cli.GetInstanceFull(instanceName)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}

	return lxdInstanceToAPIInstance(instance), nil
}

// Delete instance will delete the instance in a provider.
func (l *LXD) DeleteInstance(ctx context.Context, instance string) error {
	if err := l.setState(instance, "start", true); err != nil {
		return errors.Wrap(err, "stopping instance")
	}

	op, err := l.cli.DeleteInstance(instance)
	if err != nil {
		return errors.Wrap(err, "removing instance")
	}

	err = op.Wait()
	if err != nil {
		return errors.Wrap(err, "waiting for instance deletion")
	}
	return nil
}

// ListInstances will list all instances for a provider.
func (l *LXD) ListInstances(ctx context.Context, poolID string) ([]params.Instance, error) {
	instances, err := l.cli.GetInstancesFull(api.InstanceTypeAny)
	if err != nil {
		return []params.Instance{}, errors.Wrap(err, "fetching instances")
	}

	ret := []params.Instance{}

	for _, instance := range instances {
		if id, ok := instance.ExpandedConfig[controllerIDKeyName]; ok && id == l.controllerID {
			if poolID != "" {
				id := instance.ExpandedConfig[poolID]
				if id != poolID {
					// Pool ID was specified. Filter out instances belonging to other pools.
					continue
				}
			}
			ret = append(ret, lxdInstanceToAPIInstance(&instance))
		}
	}

	return ret, nil
}

// RemoveAllInstances will remove all instances created by this provider.
func (l *LXD) RemoveAllInstances(ctx context.Context) error {
	instances, err := l.ListInstances(ctx, "")
	if err != nil {
		return errors.Wrap(err, "fetching instance list")
	}

	for _, instance := range instances {
		// TODO: remove in parallel
		if err := l.DeleteInstance(ctx, instance.Name); err != nil {
			return errors.Wrapf(err, "removing instance %s", instance.Name)
		}
	}

	return nil
}

func (l *LXD) setState(instance, state string, force bool) error {
	reqState := api.InstanceStatePut{
		Action:  state,
		Timeout: -1,
		Force:   force,
	}

	op, err := l.cli.UpdateInstanceState(instance, reqState, "")
	if err != nil {
		return errors.Wrapf(err, "setting state to %s", state)
	}
	err = op.Wait()
	if err != nil {
		return errors.Wrapf(err, "waiting for instance to transition to state %s", state)
	}
	return nil
}

// Stop shuts down the instance.
func (l *LXD) Stop(ctx context.Context, instance string, force bool) error {
	return l.setState(instance, "stop", force)
}

// Start boots up an instance.
func (l *LXD) Start(ctx context.Context, instance string) error {
	return l.setState(instance, "start", false)
}
