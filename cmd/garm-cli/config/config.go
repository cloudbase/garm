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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
)

const (
	DefaultAppFolder      = "garm-cli"
	DefaultConfigFileName = "config.toml"
)

func getConfigFilePath() (string, error) {
	configDir, err := getHomeDir()
	if err != nil {
		return "", fmt.Errorf("error fetching home folder: %w", err)
	}

	if err := ensureHomeDir(configDir); err != nil {
		return "", fmt.Errorf("error ensuring config dir: %w", err)
	}

	cfgFile := filepath.Join(configDir, DefaultConfigFileName)
	return cfgFile, nil
}

func LoadConfig() (*Config, error) {
	cfgFile, err := getConfigFilePath()
	if err != nil {
		return nil, fmt.Errorf("error fetching config: %w", err)
	}

	if _, err := os.Stat(cfgFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// return empty config
			return &Config{}, nil
		}
		return nil, fmt.Errorf("error accessing config file: %w", err)
	}

	var config Config
	if _, err := toml.DecodeFile(cfgFile, &config); err != nil {
		return nil, fmt.Errorf("error decoding toml: %w", err)
	}

	return &config, nil
}

type Config struct {
	mux           sync.Mutex
	Managers      []Manager `toml:"manager"`
	ActiveManager string    `toml:"active_manager"`
}

func (c *Config) HasManager(mgr string) bool {
	c.mux.Lock()
	defer c.mux.Unlock()
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

func (c *Config) SetManagerToken(name, token string) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	found := false
	newManagerList := []Manager{}
	for _, mgr := range c.Managers {
		newMgr := Manager{
			Name:    mgr.Name,
			BaseURL: mgr.BaseURL,
			Token:   mgr.Token,
		}
		if mgr.Name == name {
			found = true
			newMgr.Token = token
		}
		newManagerList = append(newManagerList, newMgr)
	}
	if !found {
		return fmt.Errorf("profile %s not found", name)
	}
	c.Managers = newManagerList
	return nil
}

func (c *Config) DeleteProfile(name string) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	newManagers := []Manager{}
	for _, val := range c.Managers {
		if val.Name == name {
			continue
		}
		newManagers = append(newManagers, Manager{
			Name:    val.Name,
			BaseURL: val.BaseURL,
			Token:   val.Token,
		})
	}
	c.Managers = newManagers
	if c.ActiveManager == name {
		if len(c.Managers) > 0 {
			c.ActiveManager = c.Managers[0].Name
		} else {
			c.ActiveManager = ""
		}
	}
	return nil
}

func (c *Config) GetActiveConfig() (Manager, error) {
	c.mux.Lock()
	defer c.mux.Unlock()
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
	c.mux.Lock()
	defer c.mux.Unlock()
	cfgFile, err := getConfigFilePath()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("error getting config: %w", err)
		}
	}
	cfgHandle, err := os.Create(cfgFile)
	if err != nil {
		return fmt.Errorf("error getting file handle: %w", err)
	}

	encoder := toml.NewEncoder(cfgHandle)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("error saving config: %w", err)
	}

	return nil
}

type Manager struct {
	Name    string `toml:"name"`
	BaseURL string `toml:"base_url"`
	Token   string `toml:"bearer_token"`
}
