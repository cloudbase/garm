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

package lxd

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/cloudbase/garm/config"
	runnerErrors "github.com/cloudbase/garm/errors"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util"

	"github.com/google/go-github/v48/github"
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/pkg/errors"
)

var _ common.Provider = &LXD{}

const (
	// We look for this key in the config of the instances to determine if they are
	// created by us or not.
	controllerIDKeyName = "user.runner-controller-id"
	poolIDKey           = "user.runner-pool-id"

	// osTypeKeyName is the key we use in the instance config to indicate the OS
	// platform a runner is supposed to have. This value is defined in the pool and
	// passed into the provider as bootstrap params.
	osTypeKeyName = "user.os-type"

	// osArchKeyNAme is the key we use in the instance config to indicate the OS
	// architecture a runner is supposed to have. This value is defined in the pool and
	// passed into the provider as bootstrap params.
	osArchKeyNAme = "user.os-arch"
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

	configToLXDArchMap map[params.OSArch]string = map[params.OSArch]string{
		params.Amd64: "x86_64",
		params.Arm64: "aarch64",
		params.Arm:   "armv7l",
	}

	lxdToConfigArch map[string]params.OSArch = map[string]params.OSArch{
		"x86_64":  params.Amd64,
		"aarch64": params.Arm64,
		"armv7l":  params.Arm,
	}
)

const (
	DefaultProjectDescription = "This project was created automatically by garm to be used for github ephemeral action runners."
	DefaultProjectName        = "garm-project"
)

func NewProvider(ctx context.Context, cfg *config.Provider, controllerID string) (common.Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating provider config")
	}

	if cfg.ProviderType != params.LXDProvider {
		return nil, fmt.Errorf("invalid provider type %s, expected %s", cfg.ProviderType, params.LXDProvider)
	}

	provider := &LXD{
		ctx:          ctx,
		cfg:          cfg,
		controllerID: controllerID,
		imageManager: &image{
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

	mux sync.Mutex
}

func (l *LXD) getCLI() (lxd.InstanceServer, error) {
	l.mux.Lock()
	defer l.mux.Unlock()

	if l.cli != nil {
		return l.cli, nil
	}
	cli, err := getClientFromConfig(l.ctx, &l.cfg.LXD)
	if err != nil {
		return nil, errors.Wrap(err, "creating LXD client")
	}

	_, _, err = cli.GetProject(projectName(l.cfg.LXD))
	if err != nil {
		return nil, errors.Wrapf(err, "fetching project name: %s", projectName(l.cfg.LXD))
	}
	cli = cli.UseProject(projectName(l.cfg.LXD))
	l.cli = cli

	return cli, nil
}

func (l *LXD) getProfiles(flavor string) ([]string, error) {
	ret := []string{}
	if l.cfg.LXD.IncludeDefaultProfile {
		ret = append(ret, "default")
	}

	set := map[string]struct{}{}

	cli, err := l.getCLI()
	if err != nil {
		return nil, errors.Wrap(err, "fetching client")
	}

	profiles, err := cli.GetProfileNames()
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

func (l *LXD) getTools(image *api.Image, tools []*github.RunnerApplicationDownload, assumedOSType params.OSType) (github.RunnerApplicationDownload, error) {
	if image == nil {
		return github.RunnerApplicationDownload{}, fmt.Errorf("nil image received")
	}
	osName, ok := image.ImagePut.Properties["os"]
	if !ok {
		return github.RunnerApplicationDownload{}, fmt.Errorf("missing OS info in image properties")
	}

	osType, err := util.OSToOSType(osName)
	if err != nil {
		log.Printf("failed to determine OS type from image, assuming %s", assumedOSType)
		osType = assumedOSType
	}

	// Validate image OS. Linux only for now.
	switch osType {
	case params.Linux:
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

		// fmt.Println(*tool.Architecture, *tool.OS)
		// fmt.Printf("image arch: %s --> osType: %s\n", image.Architecture, string(osType))
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

	instanceType := l.cfg.LXD.GetInstanceType()

	image, err := l.imageManager.EnsureImage(bootstrapParams.Image, instanceType, arch, l.cli)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "getting image details")
	}

	tools, err := l.getTools(image, bootstrapParams.Tools, bootstrapParams.OSType)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "getting tools")
	}

	cloudCfg, err := util.GetCloudConfig(bootstrapParams, tools, bootstrapParams.Name)
	if err != nil {
		return api.InstancesPost{}, errors.Wrap(err, "generating cloud-config")
	}

	configMap := map[string]string{
		"user.user-data":    cloudCfg,
		osTypeKeyName:       string(bootstrapParams.OSType),
		osArchKeyNAme:       string(bootstrapParams.OSArch),
		controllerIDKeyName: l.controllerID,
		poolIDKey:           bootstrapParams.PoolID,
	}

	if instanceType == config.LXDImageVirtualMachine {
		configMap["security.secureboot"] = l.secureBootEnabled()
	}

	args := api.InstancesPost{
		InstancePut: api.InstancePut{
			Architecture: image.Architecture,
			Profiles:     profiles,
			Description:  "Github runner provisioned by garm",
			Config:       configMap,
		},
		Source: api.InstanceSource{
			Type:        "image",
			Fingerprint: image.Fingerprint,
		},
		Name: bootstrapParams.Name,
		Type: api.InstanceType(instanceType),
	}
	return args, nil
}

func (l *LXD) AsParams() params.Provider {
	return params.Provider{
		Name:         l.cfg.Name,
		ProviderType: l.cfg.ProviderType,
		Description:  l.cfg.Description,
	}
}

func (l *LXD) launchInstance(createArgs api.InstancesPost) error {
	cli, err := l.getCLI()
	if err != nil {
		return errors.Wrap(err, "fetching client")
	}
	// Get LXD to create the instance (background operation)
	op, err := cli.CreateInstance(createArgs)
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

	op, err = cli.UpdateInstanceState(createArgs.Name, reqState, "")
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

	if err := l.launchInstance(args); err != nil {
		return params.Instance{}, errors.Wrap(err, "creating instance")
	}

	ret, err := l.waitInstanceHasIP(ctx, args.Name)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}

	return ret, nil
}

// GetInstance will return details about one instance.
func (l *LXD) GetInstance(ctx context.Context, instanceName string) (params.Instance, error) {
	cli, err := l.getCLI()
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching client")
	}
	instance, _, err := cli.GetInstanceFull(instanceName)
	if err != nil {
		if isNotFoundError(err) {
			return params.Instance{}, errors.Wrapf(runnerErrors.ErrNotFound, "fetching instance: %q", err)
		}
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}

	return lxdInstanceToAPIInstance(instance), nil
}

// Delete instance will delete the instance in a provider.
func (l *LXD) DeleteInstance(ctx context.Context, instance string) error {
	cli, err := l.getCLI()
	if err != nil {
		return errors.Wrap(err, "fetching client")
	}

	if err := l.setState(instance, "stop", true); err != nil {
		if isNotFoundError(err) {
			return nil
		}
		// I am not proud of this, but the drivers.ErrInstanceIsStopped from LXD pulls in
		// a ton of CGO, linux specific dependencies, that don't make sense having
		// in garm.
		if !(errors.Cause(err).Error() == errInstanceIsStopped.Error()) {
			return errors.Wrap(err, "stopping instance")
		}
	}

	op, err := cli.DeleteInstance(instance)
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return errors.Wrap(err, "removing instance")
	}

	err = op.Wait()
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return errors.Wrap(err, "waiting for instance deletion")
	}
	return nil
}

// ListInstances will list all instances for a provider.
func (l *LXD) ListInstances(ctx context.Context, poolID string) ([]params.Instance, error) {
	cli, err := l.getCLI()
	if err != nil {
		return []params.Instance{}, errors.Wrap(err, "fetching client")
	}

	instances, err := cli.GetInstancesFull(api.InstanceTypeAny)
	if err != nil {
		return []params.Instance{}, errors.Wrap(err, "fetching instances")
	}

	ret := []params.Instance{}

	for _, instance := range instances {
		if id, ok := instance.ExpandedConfig[controllerIDKeyName]; ok && id == l.controllerID {
			if poolID != "" {
				id := instance.ExpandedConfig[poolIDKey]
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

	cli, err := l.getCLI()
	if err != nil {
		return errors.Wrap(err, "fetching client")
	}

	op, err := cli.UpdateInstanceState(instance, reqState, "")
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
