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

package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"

	runnerErrors "garm/errors"
)

const (
	DefaultAppFolder      = "garm-cli"
	DefaultConfigFileName = "config.toml"
)

func getConfigFilePath() (string, error) {
	configDir, err := getHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "fetching home folder")
	}

	if err := ensureHomeDir(configDir); err != nil {
		return "", errors.Wrap(err, "ensuring config dir")
	}

	cfgFile := filepath.Join(configDir, DefaultConfigFileName)
	return cfgFile, nil
}

func LoadConfig() (*Config, error) {
	cfgFile, err := getConfigFilePath()
	if err != nil {
		return nil, errors.Wrap(err, "fetching config")
	}

	if _, err := os.Stat(cfgFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// return empty config
			return &Config{}, nil
		}
		return nil, errors.Wrap(err, "accessing config file")
	}

	var config Config
	if _, err := toml.DecodeFile(cfgFile, &config); err != nil {
		return nil, errors.Wrap(err, "decoding toml")
	}

	return &config, nil
}

type Config struct {
	Managers      []Manager `toml:"manager"`
	ActiveManager string    `toml:"active_manager"`
}

func (c *Config) HasManager(mgr string) bool {
	if mgr == "" {
		return false
	}
	for _, val := range c.Managers {
		if val.Name == mgr {
			return true
		}
	}
	return false
}

func (c *Config) GetActiveConfig() (Manager, error) {
	if c.ActiveManager == "" {
		return Manager{}, runnerErrors.ErrNotFound
	}

	for _, val := range c.Managers {
		if val.Name == c.ActiveManager {
			return val, nil
		}
	}
	return Manager{}, runnerErrors.ErrNotFound
}

func (c *Config) SaveConfig() error {
	cfgFile, err := getConfigFilePath()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Wrap(err, "getting config")
		}
	}
	cfgHandle, err := os.Create(cfgFile)
	if err != nil {
		errors.Wrap(err, "getting file handle")
	}

	encoder := toml.NewEncoder(cfgHandle)
	if err := encoder.Encode(c); err != nil {
		return errors.Wrap(err, "saving config")
	}

	return nil
}

type Manager struct {
	Name    string `toml:"name"`
	BaseURL string `toml:"base_url"`
	Token   string `toml:"bearer_token"`
}
