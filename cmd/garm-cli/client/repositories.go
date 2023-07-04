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

package client

import (
	"encoding/json"
	"fmt"

	"github.com/cloudbase/garm/params"
)

func (c *Client) ListRepositories() ([]params.Repository, error) {
	var repos []params.Repository
	url := fmt.Sprintf("%s/api/v1/repositories", c.Config.BaseURL)
	resp, err := c.client.R().
		SetResult(&repos).
		Get(url)
	if err := c.handleError(err, resp); err != nil {
		return nil, err
	}
	return repos, nil
}

func (c *Client) CreateRepository(param params.CreateRepoParams) (params.Repository, error) {
	var response params.Repository
	url := fmt.Sprintf("%s/api/v1/repositories", c.Config.BaseURL)

	body, err := json.Marshal(param)
	if err != nil {
		return params.Repository{}, err
	}
	resp, err := c.client.R().
		SetBody(body).
		SetResult(&response).
		Post(url)
	if err := c.handleError(err, resp); err != nil {
		return params.Repository{}, err
	}
	return response, nil
}

func (c *Client) GetRepository(repoID string) (params.Repository, error) {
	var response params.Repository
	url := fmt.Sprintf("%s/api/v1/repositories/%s", c.Config.BaseURL, repoID)
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err := c.handleError(err, resp); err != nil {
		return params.Repository{}, err
	}
	return response, nil
}

func (c *Client) DeleteRepository(repoID string) error {
	url := fmt.Sprintf("%s/api/v1/repositories/%s", c.Config.BaseURL, repoID)
	resp, err := c.client.R().
		Delete(url)
	if err := c.handleError(err, resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) CreateRepoPool(repoID string, param params.CreatePoolParams) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/repositories/%s/pools", c.Config.BaseURL, repoID)

	var response params.Pool
	body, err := json.Marshal(param)
	if err != nil {
		return response, err
	}
	resp, err := c.client.R().
		SetBody(body).
		SetResult(&response).
		Post(url)
	if err := c.handleError(err, resp); err != nil {
		return params.Pool{}, err
	}
	return response, nil
}

func (c *Client) ListRepoPools(repoID string) ([]params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/repositories/%s/pools", c.Config.BaseURL, repoID)

	var response []params.Pool
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err := c.handleError(err, resp); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) GetRepoPool(repoID, poolID string) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/repositories/%s/pools/%s", c.Config.BaseURL, repoID, poolID)

	var response params.Pool
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err := c.handleError(err, resp); err != nil {
		return params.Pool{}, err
	}
	return response, nil
}

func (c *Client) DeleteRepoPool(repoID, poolID string) error {
	url := fmt.Sprintf("%s/api/v1/repositories/%s/pools/%s", c.Config.BaseURL, repoID, poolID)

	resp, err := c.client.R().
		Delete(url)

	if err := c.handleError(err, resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) UpdateRepoPool(repoID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/repositories/%s/pools/%s", c.Config.BaseURL, repoID, poolID)

	var response params.Pool
	body, err := json.Marshal(param)
	if err != nil {
		return response, err
	}
	resp, err := c.client.R().
		SetBody(body).
		SetResult(&response).
		Put(url)
	if err := c.handleError(err, resp); err != nil {
		return params.Pool{}, err
	}
	return response, nil
}

func (c *Client) UpdateRepo(repoID string, param params.UpdateRepositoryParams) (params.Repository, error) {
	url := fmt.Sprintf("%s/api/v1/repositories/%s", c.Config.BaseURL, repoID)

	var response params.Repository
	body, err := json.Marshal(param)
	if err != nil {
		return response, err
	}
	resp, err := c.client.R().
		SetBody(body).
		SetResult(&response).
		Put(url)
	if err := c.handleError(err, resp); err != nil {
		return params.Repository{}, err
	}
	return response, nil
}

func (c *Client) ListRepoInstances(repoID string) ([]params.Instance, error) {
	url := fmt.Sprintf("%s/api/v1/repositories/%s/instances", c.Config.BaseURL, repoID)

	var response []params.Instance
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err := c.handleError(err, resp); err != nil {
		return nil, err
	}
	return response, nil
}
