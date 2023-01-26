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
)

func (c *Client) ListOrganizations() ([]params.Organization, error) {
	var orgs []params.Organization
	url := fmt.Sprintf("%s/api/v1/organizations", c.Config.BaseURL)
	resp, err := c.client.R().
		SetResult(&orgs).
		Get(url)
	if err := c.handleError(err, resp); err != nil {
		return nil, err
	}
	return orgs, nil
}

func (c *Client) CreateOrganization(param params.CreateOrgParams) (params.Organization, error) {
	var response params.Organization
	url := fmt.Sprintf("%s/api/v1/organizations", c.Config.BaseURL)

	body, err := json.Marshal(param)
	if err != nil {
		return params.Organization{}, err
	}
	resp, err := c.client.R().
		SetBody(body).
		SetResult(&response).
		Post(url)
	if err := c.handleError(err, resp); err != nil {
		return params.Organization{}, err
	}
	return response, nil
}

func (c *Client) GetOrganization(orgID string) (params.Organization, error) {
	var response params.Organization
	url := fmt.Sprintf("%s/api/v1/organizations/%s", c.Config.BaseURL, orgID)
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err := c.handleError(err, resp); err != nil {
		return params.Organization{}, err
	}
	return response, nil
}

func (c *Client) DeleteOrganization(orgID string) error {
	url := fmt.Sprintf("%s/api/v1/organizations/%s", c.Config.BaseURL, orgID)
	resp, err := c.client.R().
		Delete(url)
	if err := c.handleError(err, resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) CreateOrgPool(orgID string, param params.CreatePoolParams) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/pools", c.Config.BaseURL, orgID)

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

func (c *Client) ListOrgPools(orgID string) ([]params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/pools", c.Config.BaseURL, orgID)

	var response []params.Pool
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err := c.handleError(err, resp); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) GetOrgPool(orgID, poolID string) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/pools/%s", c.Config.BaseURL, orgID, poolID)

	var response params.Pool
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err := c.handleError(err, resp); err != nil {
		return params.Pool{}, err
	}
	return response, nil
}

func (c *Client) DeleteOrgPool(orgID, poolID string) error {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/pools/%s", c.Config.BaseURL, orgID, poolID)

	resp, err := c.client.R().
		Delete(url)

	if err := c.handleError(err, resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) UpdateOrgPool(orgID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/pools/%s", c.Config.BaseURL, orgID, poolID)

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

func (c *Client) ListOrgInstances(orgID string) ([]params.Instance, error) {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/instances", c.Config.BaseURL, orgID)

	var response []params.Instance
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err := c.handleError(err, resp); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) CreateMetricsToken() (string, error) {
	url := fmt.Sprintf("%s/api/v1/metrics-token", c.Config.BaseURL)

	type response struct {
		Token string `json:"token"`
	}

	var t response
	resp, err := c.client.R().
		SetResult(&t).
		Get(url)
	if err := c.handleError(err, resp); err != nil {
		return "", err
	}
	return t.Token, nil
}
