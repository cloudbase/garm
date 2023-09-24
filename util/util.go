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

package util

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"

	"github.com/google/go-github/v55/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

type githubClient struct {
	*github.ActionsService
	org  *github.OrganizationsService
	repo *github.RepositoriesService
}

func (g *githubClient) ListOrgHooks(ctx context.Context, org string, opts *github.ListOptions) ([]*github.Hook, *github.Response, error) {
	return g.org.ListHooks(ctx, org, opts)
}

func (g *githubClient) GetOrgHook(ctx context.Context, org string, id int64) (*github.Hook, *github.Response, error) {
	return g.org.GetHook(ctx, org, id)
}

func (g *githubClient) CreateOrgHook(ctx context.Context, org string, hook *github.Hook) (*github.Hook, *github.Response, error) {
	return g.org.CreateHook(ctx, org, hook)
}

func (g *githubClient) DeleteOrgHook(ctx context.Context, org string, id int64) (*github.Response, error) {
	return g.org.DeleteHook(ctx, org, id)
}

func (g *githubClient) PingOrgHook(ctx context.Context, org string, id int64) (*github.Response, error) {
	return g.org.PingHook(ctx, org, id)
}

func (g *githubClient) ListRepoHooks(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.Hook, *github.Response, error) {
	return g.repo.ListHooks(ctx, owner, repo, opts)
}

func (g *githubClient) GetRepoHook(ctx context.Context, owner, repo string, id int64) (*github.Hook, *github.Response, error) {
	return g.repo.GetHook(ctx, owner, repo, id)
}

func (g *githubClient) CreateRepoHook(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, *github.Response, error) {
	return g.repo.CreateHook(ctx, owner, repo, hook)
}

func (g *githubClient) DeleteRepoHook(ctx context.Context, owner, repo string, id int64) (*github.Response, error) {
	return g.repo.DeleteHook(ctx, owner, repo, id)
}

func (g *githubClient) PingRepoHook(ctx context.Context, owner, repo string, id int64) (*github.Response, error) {
	return g.repo.PingHook(ctx, owner, repo, id)
}

func GithubClient(ctx context.Context, token string, credsDetails params.GithubCredentials) (common.GithubClient, common.GithubEnterpriseClient, error) {
	var roots *x509.CertPool
	if credsDetails.CABundle != nil && len(credsDetails.CABundle) > 0 {
		roots = x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(credsDetails.CABundle)
		if !ok {
			return nil, nil, fmt.Errorf("failed to parse CA cert")
		}
	}
	httpTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: roots,
		},
	}
	httpClient := &http.Client{Transport: httpTransport}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	ghClient, err := github.NewEnterpriseClient(credsDetails.APIBaseURL, credsDetails.UploadBaseURL, tc)
	if err != nil {
		return nil, nil, errors.Wrap(err, "fetching github client")
	}

	cli := &githubClient{
		ActionsService: ghClient.Actions,
		org:            ghClient.Organizations,
		repo:           ghClient.Repositories,
	}
	return cli, ghClient.Enterprise, nil
}
