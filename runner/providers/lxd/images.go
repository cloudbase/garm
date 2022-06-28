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
	"fmt"
	"log"
	"strings"

	"garm/config"
	runnerErrors "garm/errors"

	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type image struct {
	remotes map[string]config.LXDImageRemote

	cli lxd.InstanceServer
}

// parseImageName parses the image name that comes in from the config and returns a
// remote. If no remote is configured with the given name, an error is returned.
func (i *image) parseImageName(imageName string) (config.LXDImageRemote, string, error) {
	if !strings.Contains(imageName, ":") {
		return config.LXDImageRemote{}, "", fmt.Errorf("image does not include a remote")
	}

	details := strings.SplitN(imageName, ":", 2)
	for remoteName, val := range i.remotes {
		if remoteName == details[0] {
			return val, details[1], nil
		}
	}
	return config.LXDImageRemote{}, "", runnerErrors.ErrNotFound
}

func (i *image) getLocalImageByAlias(imageName string, imageType config.LXDImageType, arch string) (*api.Image, error) {
	aliases, err := i.cli.GetImageAliasArchitectures(imageType.String(), imageName)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving alias: %s", imageName)
	}

	alias, ok := aliases[arch]
	if !ok {
		return nil, fmt.Errorf("no image found for arch %s and image type %s with name %s", arch, imageType, imageName)
	}

	image, _, err := i.cli.GetImage(alias.Target)
	if err != nil {
		return nil, errors.Wrap(err, "fetching image details")
	}
	return image, nil
}

func (i *image) clientFromRemoteArgs(remote config.LXDImageRemote) (lxd.ImageServer, error) {
	connectArgs := &lxd.ConnectionArgs{
		InsecureSkipVerify: remote.InsecureSkipVerify,
	}
	d, err := lxd.ConnectSimpleStreams(remote.Address, connectArgs)
	if err != nil {
		return nil, errors.Wrapf(err, "connecting to image server %s", remote.Address)
	}
	return d, nil
}

func (i *image) copyImageFromRemote(remote config.LXDImageRemote, imageName string, imageType config.LXDImageType, arch string) (*api.Image, error) {
	imgCli, err := i.clientFromRemoteArgs(remote)
	if err != nil {
		return nil, errors.Wrap(err, "fetching image server client")
	}
	defer imgCli.Disconnect()

	aliases, err := imgCli.GetImageAliasArchitectures(imageType.String(), imageName)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving alias: %s", imageName)
	}

	yml, err := yaml.Marshal(aliases)
	if err != nil {
		return nil, err
	}

	fmt.Println(string(yml))

	alias, ok := aliases[arch]
	if !ok {
		return nil, fmt.Errorf("no image found for arch %s and image type %s with name %s", arch, imageType, imageName)
	}

	image, _, err := imgCli.GetImage(alias.Target)
	if err != nil {
		return nil, errors.Wrap(err, "fetching image details")
	}

	// Ask LXD to copy the image from the remote server
	imgAliases := []api.ImageAlias{}
	found := false
	for _, alias := range image.Aliases {
		if alias.Name == imageName {
			found = true
			break
		}
	}
	if !found {
		imgAliases = append(imgAliases, api.ImageAlias{Name: imageName})
	}

	imgCopyArgs := &lxd.ImageCopyArgs{
		AutoUpdate:  true,
		CopyAliases: true,
		Aliases:     imgAliases,
	}
	op, err := i.cli.CopyImage(imgCli, *image, imgCopyArgs)
	if err != nil {
		return nil, errors.Wrapf(err, "copying image %s from %s", imageName, remote.Address)
	}

	// And wait for it to finish
	err = op.Wait()
	if err != nil {
		return nil, errors.Wrap(err, "waiting for image copy operation")
	}

	// We should now have the image locally. Force another query. This probably makes no sense,
	// but this is done only once.
	return i.getLocalImageByAlias(imageName, imageType, arch)
}

// EnsureImage will look for an image locally, then attempt to download it from a remote
// server, if the name contains a remote. Allowed formats are:
// remote_name:image_name
// image_name
func (i *image) EnsureImage(imageName string, imageType config.LXDImageType, arch string) (*api.Image, error) {
	if !strings.Contains(imageName, ":") {
		// A remote was not specified, try to find an image using the imageName as
		// an alias.
		return i.getLocalImageByAlias(imageName, imageType, arch)
	}

	remote, parsedName, err := i.parseImageName(imageName)
	if err != nil {
		return nil, errors.Wrap(err, "parsing image name")
	}

	if img, err := i.getLocalImageByAlias(parsedName, imageType, arch); err == nil {
		return img, nil
	} else {
		log.Printf("failed to fetch local image of type %v with name %s and arch %s: %s", imageType, parsedName, arch, err)
	}

	img, err := i.copyImageFromRemote(remote, parsedName, imageType, arch)
	if err != nil {
		return nil, errors.Wrap(err, "fetching image")
	}
	return img, nil
}
