package client

import (
	"encoding/json"
	"fmt"

	apiParams "garm/apiserver/params"
	"garm/cmd/garm-cli/config"
	"garm/params"

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

func (c *Client) ListPoolInstances(poolID string) ([]params.Instance, error) {
	url := fmt.Sprintf("%s/api/v1/pools/instances/%s", c.Config.BaseURL, poolID)

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
