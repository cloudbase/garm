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

	"garm/params"

	"github.com/pkg/errors"
)

func (c *Client) ListEnterprises() ([]params.Enterprise, error) {
	var enterprises []params.Enterprise
	url := fmt.Sprintf("%s/api/v1/enterprises", c.Config.BaseURL)
	resp, err := c.client.R().
		SetResult(&enterprises).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return nil, errors.Wrap(decErr, "sending request")
		}
		return nil, fmt.Errorf("error fetching enterprises: %s", apiErr.Details)
	}
	return enterprises, nil
}

func (c *Client) CreateEnterprise(param params.CreateEnterpriseParams) (params.Enterprise, error) {
	var response params.Enterprise
	url := fmt.Sprintf("%s/api/v1/enterprises", c.Config.BaseURL)

	body, err := json.Marshal(param)
	if err != nil {
		return params.Enterprise{}, err
	}
	resp, err := c.client.R().
		SetBody(body).
		SetResult(&response).
		Post(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error creating enterprise: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) GetEnterprise(enterpriseID string) (params.Enterprise, error) {
	var response params.Enterprise
	url := fmt.Sprintf("%s/api/v1/enterprises/%s", c.Config.BaseURL, enterpriseID)
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error fetching enterprise: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) DeleteEnterprise(enterpriseID string) error {
	url := fmt.Sprintf("%s/api/v1/enterprises/%s", c.Config.BaseURL, enterpriseID)
	resp, err := c.client.R().
		Delete(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return errors.Wrap(decErr, "sending request")
		}
		return fmt.Errorf("error fetching removing enterprise: %s", apiErr.Details)
	}
	return nil
}

func (c *Client) CreateEnterprisePool(enterpriseID string, param params.CreatePoolParams) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/enterprises/%s/pools", c.Config.BaseURL, enterpriseID)

	var response params.Pool
	body, err := json.Marshal(param)
	if err != nil {
		return response, err
	}
	resp, err := c.client.R().
		SetBody(body).
		SetResult(&response).
		Post(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error creating enterprise pool: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) ListEnterprisePools(enterpriseID string) ([]params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/enterprises/%s/pools", c.Config.BaseURL, enterpriseID)

	var response []params.Pool
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error listing enterprise pools: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) GetEnterprisePool(enterpriseID, poolID string) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/enterprises/%s/pools/%s", c.Config.BaseURL, enterpriseID, poolID)

	var response params.Pool
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error fetching enterprise pool: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) DeleteEnterprisePool(enterpriseID, poolID string) error {
	url := fmt.Sprintf("%s/api/v1/enterprises/%s/pools/%s", c.Config.BaseURL, enterpriseID, poolID)

	resp, err := c.client.R().
		Delete(url)

	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return errors.Wrap(decErr, "sending request")
		}
		return fmt.Errorf("error deleting enterprise pool: %s", apiErr.Details)
	}
	return nil
}

func (c *Client) UpdateEnterprisePool(enterpriseID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/enterprises/%s/pools/%s", c.Config.BaseURL, enterpriseID, poolID)

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
		return response, fmt.Errorf("error updating enterprise pool: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) ListEnterpriseInstances(enterpriseID string) ([]params.Instance, error) {
	url := fmt.Sprintf("%s/api/v1/enterprises/%s/instances", c.Config.BaseURL, enterpriseID)

	var response []params.Instance
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error listing enterprise instances: %s", apiErr.Details)
	}
	return response, nil
}
