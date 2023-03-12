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

	apiParams "github.com/cloudbase/garm/apiserver/params"
	"github.com/cloudbase/garm/cmd/garm-cli/config"
	"github.com/cloudbase/garm/params"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
)

func NewClient(name string, cfg config.Manager, debug bool) *Client {
	cli := resty.New()
	if cfg.Token != "" {
		cli = cli.SetAuthToken(cfg.Token)
	}
	cli = cli.
		SetHeader("Accept", "application/json").
		SetDebug(debug)
	return &Client{
		ManagerName: name,
		Config:      cfg,
		client:      cli,
	}
}

type Client struct {
	ManagerName string
	Config      config.Manager
	client      *resty.Client
}

func (c *Client) handleError(err error, resp *resty.Response) error {
	var ret error
	if err != nil {
		ret = fmt.Errorf("request returned error: %s", err)
	}

	if resp != nil && resp.IsError() {
		body := resp.Body()
		if len(body) > 0 {
			apiErr, decErr := c.decodeAPIError(resp.Body())
			if decErr == nil {
				ret = fmt.Errorf("API returned error: %s", apiErr.Details)
			}
		}
	}
	return ret
}

func (c *Client) decodeAPIError(body []byte) (apiParams.APIErrorResponse, error) {
	var errDetails apiParams.APIErrorResponse
	if err := json.Unmarshal(body, &errDetails); err != nil {
		return apiParams.APIErrorResponse{}, fmt.Errorf("invalid response from server, use --debug for more info")
	}

	return errDetails, fmt.Errorf("error in API call: %s", errDetails.Details)
}

func (c *Client) InitManager(url string, param params.NewUserParams) (params.User, error) {
	body, err := json.Marshal(param)
	if err != nil {
		return params.User{}, errors.Wrap(err, "marshaling body")
	}
	url = fmt.Sprintf("%s/api/v1/first-run/", url)

	var response params.User
	resp, err := c.client.R().
		SetBody(body).
		SetResult(&response).
		Post(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return params.User{}, errors.Wrap(decErr, "sending request")
		}
		return params.User{}, fmt.Errorf("error running init: %s", apiErr.Details)
	}

	return response, nil
}

func (c *Client) Login(url string, param params.PasswordLoginParams) (string, error) {
	body, err := json.Marshal(param)
	if err != nil {
		return "", errors.Wrap(err, "marshaling body")
	}
	url = fmt.Sprintf("%s/api/v1/auth/login", url)

	var response params.JWTResponse
	resp, err := c.client.R().
		SetBody(body).
		SetResult(&response).
		Post(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return "", errors.Wrap(decErr, "sending request")
		}
		return "", fmt.Errorf("error performing login: %s", apiErr.Details)
	}

	return response.Token, nil
}

func (c *Client) ListCredentials() ([]params.GithubCredentials, error) {
	var ghCreds []params.GithubCredentials
	url := fmt.Sprintf("%s/api/v1/credentials", c.Config.BaseURL)
	resp, err := c.client.R().
		SetResult(&ghCreds).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return nil, errors.Wrap(decErr, "sending request")
		}
		return nil, fmt.Errorf("error fetching credentials: %s", apiErr.Details)
	}
	return ghCreds, nil
}

func (c *Client) ListProviders() ([]params.Provider, error) {
	var providers []params.Provider
	url := fmt.Sprintf("%s/api/v1/providers", c.Config.BaseURL)
	resp, err := c.client.R().
		SetResult(&providers).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return nil, errors.Wrap(decErr, "sending request")
		}
		return nil, fmt.Errorf("error fetching providers: %s", apiErr.Details)
	}
	return providers, nil
}

func (c *Client) GetInstanceByName(instanceName string) (params.Instance, error) {
	url := fmt.Sprintf("%s/api/v1/instances/%s", c.Config.BaseURL, instanceName)

	var response params.Instance
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error performing login: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) DeleteRunner(instanceName string) error {
	url := fmt.Sprintf("%s/api/v1/instances/%s", c.Config.BaseURL, instanceName)
	resp, err := c.client.R().
		Delete(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return errors.Wrap(decErr, "sending request")
		}
		return fmt.Errorf("error deleting runner: %s", apiErr.Details)
	}
	return nil
}

func (c *Client) ListPoolInstances(poolID string) ([]params.Instance, error) {
	url := fmt.Sprintf("%s/api/v1/pools/%s/instances", c.Config.BaseURL, poolID)

	var response []params.Instance
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error performing login: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) ListAllInstances() ([]params.Instance, error) {
	url := fmt.Sprintf("%s/api/v1/instances", c.Config.BaseURL)

	var response []params.Instance
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error performing login: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) GetPoolByID(poolID string) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/pools/%s", c.Config.BaseURL, poolID)

	var response params.Pool
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error performing login: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) ListAllPools() ([]params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/pools", c.Config.BaseURL)

	var response []params.Pool
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error performing login: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) DeletePoolByID(poolID string) error {
	url := fmt.Sprintf("%s/api/v1/pools/%s", c.Config.BaseURL, poolID)
	resp, err := c.client.R().
		Delete(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return errors.Wrap(decErr, "sending request")
		}
		return fmt.Errorf("error deleting pool by ID: %s", apiErr.Details)
	}
	return nil
}

func (c *Client) UpdatePoolByID(poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/pools/%s", c.Config.BaseURL, poolID)

	var response params.Pool
	body, err := json.Marshal(param)
	if err != nil {
		return response, err
	}
	resp, err := c.client.R().
		SetBody(body).
		SetResult(&response).
		Put(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error performing login: %s", apiErr.Details)
	}
	return response, nil
}
