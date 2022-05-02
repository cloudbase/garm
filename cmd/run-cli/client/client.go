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

func NewClient(name string, cfg config.Manager) *Client {
	cli := resty.New()
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
		SetHeader("Content-Type", "application/json").
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
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		SetResult(&response).
		Post(url)
	if err != nil {
		apiErr, decErr := c.decodeAPIError(resp.Body())
		if decErr != nil {
			return "", errors.Wrap(err, "sending request")
		}
		return "", fmt.Errorf("error running init: %s", apiErr.Details)
	}

	return response.Token, nil
}
