//go:build integration
// +build integration

// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package integration

import (
	"fmt"
	"os"

	"github.com/cloudbase/garm/params"
)

func (suite *GarmSuite) TestGetControllerInfo() {
	controllerInfo := suite.GetControllerInfo()
	suite.NotEmpty(controllerInfo.ControllerID, "controller ID is empty")
}

func (suite *GarmSuite) GetMetricsToken() {
	t := suite.T()
	t.Log("Get metrics token")
	metricsToken, err := getMetricsToken(suite.cli, suite.authToken)
	suite.NoError(err, "error getting metrics token")
	suite.NotEmpty(metricsToken, "metrics token is empty")
}

func (suite *GarmSuite) GetControllerInfo() *params.ControllerInfo {
	t := suite.T()
	t.Log("Get controller info")
	controllerInfo, err := getControllerInfo(suite.cli, suite.authToken)
	suite.NoError(err, "error getting controller info")
	err = suite.appendCtrlInfoToGitHubEnv(&controllerInfo)
	suite.NoError(err, "error appending controller info to GitHub env")
	err = printJSONResponse(controllerInfo)
	suite.NoError(err, "error printing controller info")
	return &controllerInfo
}

func (suite *GarmSuite) TestListCredentials() {
	t := suite.T()
	t.Log("List credentials")
	credentials, err := listCredentials(suite.cli, suite.authToken)
	suite.NoError(err, "error listing credentials")
	suite.NotEmpty(credentials, "credentials list is empty")
}

func (suite *GarmSuite) TestListProviders() {
	t := suite.T()
	t.Log("List providers")
	providers, err := listProviders(suite.cli, suite.authToken)
	suite.NoError(err, "error listing providers")
	suite.NotEmpty(providers, "providers list is empty")
}

func (suite *GarmSuite) appendCtrlInfoToGitHubEnv(controllerInfo *params.ControllerInfo) error {
	t := suite.T()
	envFile, found := os.LookupEnv("GITHUB_ENV")
	if !found {
		t.Log("GITHUB_ENV not set, skipping appending controller info")
		return nil
	}
	file, err := os.OpenFile(envFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	t.Cleanup(func() {
		file.Close()
	})
	if _, err := file.WriteString(fmt.Sprintf("export GARM_CONTROLLER_ID=%s\n", controllerInfo.ControllerID)); err != nil {
		return err
	}
	return nil
}
