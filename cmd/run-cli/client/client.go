package client

import (
	"encoding/json"
	"fmt"

	apiParams "runner-manager/apiserver/params"
	"runner-manager/cmd/run-cli/config"
	"runner-manager/params"

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

func (c *Client) decodeAPIError(body []byte) (apiParams.APIErrorResponse, error) {
	var errDetails apiParams.APIErrorResponse
	if err := json.Unmarshal(body, &errDetails); err != nil {
		return apiParams.APIErrorResponse{}, errors.Wrap(err, "decoding response")
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
