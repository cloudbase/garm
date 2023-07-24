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

	"github.com/google/go-github/v53/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

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
			ClientCAs: roots,
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

	return ghClient.Actions, ghClient.Enterprise, nil
}
