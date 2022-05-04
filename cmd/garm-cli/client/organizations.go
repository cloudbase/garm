package client

import (
	"encoding/json"
	"fmt"

	"garm/params"

	"github.com/pkg/errors"
)

func (c *Client) ListOrganizations() ([]params.Organization, error) {
	var orgs []params.Organization
	url := fmt.Sprintf("%s/api/v1/organizations", c.Config.BaseURL)
	resp, err := c.client.R().
		SetResult(&orgs).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return nil, errors.Wrap(decErr, "sending request")
		}
		return nil, fmt.Errorf("error fetching orgs: %s", apiErr.Details)
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
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error performing login: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) GetOrganization(orgID string) (params.Organization, error) {
	var response params.Organization
	url := fmt.Sprintf("%s/api/v1/organizations/%s", c.Config.BaseURL, orgID)
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error fetching orgs: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) DeleteOrganization(orgID string) error {
	url := fmt.Sprintf("%s/api/v1/organizations/%s", c.Config.BaseURL, orgID)
	resp, err := c.client.R().
		Delete(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return errors.Wrap(decErr, "sending request")
		}
		return fmt.Errorf("error fetching orgs: %s", apiErr.Details)
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
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error creating org pool: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) ListOrgPools(orgID string) ([]params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/pools", c.Config.BaseURL, orgID)

	var response []params.Pool
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error listing org pools: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) GetOrgPool(orgID, poolID string) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/pools/%s", c.Config.BaseURL, orgID, poolID)

	var response params.Pool
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error fetching org pool: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) DeleteOrgPool(orgID, poolID string) error {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/pools/%s", c.Config.BaseURL, orgID, poolID)

	resp, err := c.client.R().
		Delete(url)

	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return errors.Wrap(decErr, "sending request")
		}
		return fmt.Errorf("error deleting org pool: %s", apiErr.Details)
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
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error updating org pool: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) ListOrgInstances(orgID string) ([]params.Instance, error) {
	url := fmt.Sprintf("%s/api/v1/organizations/%s/instances", c.Config.BaseURL, orgID)

	var response []params.Instance
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error listing org instances: %s", apiErr.Details)
	}
	return response, nil
}
