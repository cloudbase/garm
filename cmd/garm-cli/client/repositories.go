package client

import (
	"encoding/json"
	"fmt"

	"garm/params"

	"github.com/pkg/errors"
)

func (c *Client) ListRepositories() ([]params.Repository, error) {
	var repos []params.Repository
	url := fmt.Sprintf("%s/api/v1/repositories", c.Config.BaseURL)
	resp, err := c.client.R().
		SetResult(&repos).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return nil, errors.Wrap(decErr, "sending request")
		}
		return nil, fmt.Errorf("error fetching repos: %s", apiErr.Details)
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
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error performing login: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) GetRepository(repoID string) (params.Repository, error) {
	var response params.Repository
	url := fmt.Sprintf("%s/api/v1/repositories/%s", c.Config.BaseURL, repoID)
	resp, err := c.client.R().
		SetResult(&response).
		Get(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error fetching repos: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) DeleteRepository(repoID string) error {
	url := fmt.Sprintf("%s/api/v1/repositories/%s", c.Config.BaseURL, repoID)
	resp, err := c.client.R().
		Delete(url)
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return errors.Wrap(decErr, "sending request")
		}
		return fmt.Errorf("error fetching repos: %s", apiErr.Details)
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
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error performing login: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) ListRepoPools(repoID string) ([]params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/repositories/%s/pools", c.Config.BaseURL, repoID)

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

func (c *Client) GetRepoPool(repoID, poolID string) (params.Pool, error) {
	url := fmt.Sprintf("%s/api/v1/repositories/%s/pools/%s", c.Config.BaseURL, repoID, poolID)

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

func (c *Client) DeleteRepoPool(repoID, poolID string) error {
	url := fmt.Sprintf("%s/api/v1/repositories/%s/pools/%s", c.Config.BaseURL, repoID, poolID)

	resp, err := c.client.R().
		Delete(url)

	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return errors.Wrap(decErr, "sending request")
		}
		return fmt.Errorf("error performing login: %s", apiErr.Details)
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
	if err != nil || resp.IsError() {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return response, errors.Wrap(decErr, "sending request")
		}
		return response, fmt.Errorf("error performing login: %s", apiErr.Details)
	}
	return response, nil
}

func (c *Client) ListRepoInstances(repoID string) ([]params.Instance, error) {
	url := fmt.Sprintf("%s/api/v1/repositories/%s/instances", c.Config.BaseURL, repoID)

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
