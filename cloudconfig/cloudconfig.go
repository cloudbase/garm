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

package cloudconfig

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudbase/garm/util/appdefaults"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func NewDefaultCloudInitConfig() *CloudInit {
	return &CloudInit{
		PackageUpgrade: true,
		Packages: []string{
			"curl",
			"tar",
		},
		SystemInfo: &SystemInfo{
			DefaultUser: DefaultUser{
				Name:   appdefaults.DefaultUser,
				Home:   fmt.Sprintf("/home/%s", appdefaults.DefaultUser),
				Shell:  appdefaults.DefaultUserShell,
				Groups: appdefaults.DefaultUserGroups,
				Sudo:   "ALL=(ALL) NOPASSWD:ALL",
			},
		},
	}
}

type DefaultUser struct {
	Name   string   `yaml:"name"`
	Home   string   `yaml:"home"`
	Shell  string   `yaml:"shell"`
	Groups []string `yaml:"groups,omitempty"`
	Sudo   string   `yaml:"sudo"`
}

type SystemInfo struct {
	DefaultUser DefaultUser `yaml:"default_user"`
}

type File struct {
	Encoding    string `yaml:"encoding"`
	Content     string `yaml:"content"`
	Owner       string `yaml:"owner"`
	Path        string `yaml:"path"`
	Permissions string `yaml:"permissions"`
}

type CloudInit struct {
	mux sync.Mutex

	PackageUpgrade    bool        `yaml:"package_upgrade"`
	Packages          []string    `yaml:"packages,omitempty"`
	SSHAuthorizedKeys []string    `yaml:"ssh_authorized_keys,omitempty"`
	SystemInfo        *SystemInfo `yaml:"system_info,omitempty"`
	RunCmd            []string    `yaml:"runcmd,omitempty"`
	WriteFiles        []File      `yaml:"write_files,omitempty"`
	CACerts           CACerts     `yaml:"ca-certs,omitempty"`
}

type CACerts struct {
	RemoveDefaults bool     `yaml:"remove-defaults"`
	Trusted        []string `yaml:"trusted"`
}

func (c *CloudInit) AddCACert(cert []byte) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	if cert == nil {
		return nil
	}

	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM(cert); !ok {
		return fmt.Errorf("failed to parse CA cert bundle")
	}
	c.CACerts.Trusted = append(c.CACerts.Trusted, string(cert))

	return nil
}

func (c *CloudInit) AddSSHKey(keys ...string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	// TODO(gabriel-samfira): Validate the SSH public key.
	for _, key := range keys {
		found := false
		for _, val := range c.SSHAuthorizedKeys {
			if val == key {
				found = true
				break
			}
		}
		if !found {
			c.SSHAuthorizedKeys = append(c.SSHAuthorizedKeys, key)
		}
	}
}

func (c *CloudInit) AddPackage(pkgs ...string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	for _, pkg := range pkgs {
		found := false
		for _, val := range c.Packages {
			if val == pkg {
				found = true
				break
			}
		}
		if !found {
			c.Packages = append(c.Packages, pkg)
		}
	}
}

func (c *CloudInit) AddRunCmd(cmd string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.RunCmd = append(c.RunCmd, cmd)
}

func (c *CloudInit) AddFile(contents []byte, path, owner, permissions string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	for _, val := range c.WriteFiles {
		if val.Path == path {
			return
		}
	}

	file := File{
		Encoding:    "b64",
		Content:     base64.StdEncoding.EncodeToString(contents),
		Owner:       owner,
		Permissions: permissions,
		Path:        path,
	}
	c.WriteFiles = append(c.WriteFiles, file)
}

func (c *CloudInit) Serialize() (string, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	ret := []string{
		"#cloud-config",
	}

	asYaml, err := yaml.Marshal(c)
	if err != nil {
		return "", errors.Wrap(err, "marshaling to yaml")
	}

	ret = append(ret, string(asYaml))
	return strings.Join(ret, "\n"), nil
}
