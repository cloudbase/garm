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

package sql

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
)

type CtrlTestSuite struct {
	suite.Suite
	Store dbCommon.Store
}

func (s *CtrlTestSuite) SetupTest() {
	ctx := context.Background()
	watcher.InitWatcher(ctx)
	s.Store = newTestDB(s.T())
}

func (s *CtrlTestSuite) TearDownTest() {
	watcher.CloseWatcher()
}

func (s *CtrlTestSuite) TestControllerInfo() {
	initCtrlInfo, err := s.Store.InitController()
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot init controller: %v", err))
	}

	ctrlInfo, err := s.Store.ControllerInfo()

	s.Require().Nil(err)
	s.Require().Equal(initCtrlInfo.ControllerID, ctrlInfo.ControllerID)
}

func (s *CtrlTestSuite) TestControllerInfoErrNotFound() {
	_, err := s.Store.ControllerInfo()

	s.Require().Regexp("fetching controller info: not found", err.Error())
}

func (s *CtrlTestSuite) TestInitControllerAlreadyInitialized() {
	_, err := s.Store.InitController()
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot init controller: %v", err))
	}

	_, err = s.Store.InitController()

	s.Require().Regexp(runnerErrors.NewConflictError("controller already initialized"), err)
}

func TestCtrlTestSuite(t *testing.T) {
	suite.Run(t, new(CtrlTestSuite))
}

func (s *CtrlTestSuite) TestUpdateControllerGARMAgentVersion() {
	_, err := s.Store.InitController()
	s.Require().NoError(err)

	// Pin a specific version.
	version := "v1.2.3"
	info, err := s.Store.UpdateController(params.UpdateControllerParams{GARMAgentVersion: &version})
	s.Require().NoError(err)
	s.Require().Equal("v1.2.3", info.GARMAgentVersion)

	// The value round-trips through ControllerInfo.
	info, err = s.Store.ControllerInfo()
	s.Require().NoError(err)
	s.Require().Equal("v1.2.3", info.GARMAgentVersion)

	// "latest" is canonicalized to the empty string (track latest).
	version = params.GARMAgentLatestVersion
	info, err = s.Store.UpdateController(params.UpdateControllerParams{GARMAgentVersion: &version})
	s.Require().NoError(err)
	s.Require().Equal("", info.GARMAgentVersion)

	// Invalid versions are rejected by validation inside the transaction.
	version = "not-semver"
	_, err = s.Store.UpdateController(params.UpdateControllerParams{GARMAgentVersion: &version})
	s.Require().Error(err)
	s.Require().Regexp("invalid garm_agent_version", err.Error())

	// A nil pointer leaves the stored value untouched.
	version = "v2.0.0"
	_, err = s.Store.UpdateController(params.UpdateControllerParams{GARMAgentVersion: &version})
	s.Require().NoError(err)
	info, err = s.Store.UpdateController(params.UpdateControllerParams{})
	s.Require().NoError(err)
	s.Require().Equal("v2.0.0", info.GARMAgentVersion)
}

func (s *CtrlTestSuite) TestUpdateControllerAllowInsecureGARMAgent() {
	_, err := s.Store.InitController()
	s.Require().NoError(err)

	// The flag defaults to off.
	info, err := s.Store.ControllerInfo()
	s.Require().NoError(err)
	s.Require().False(info.AllowInsecureGARMAgent)

	// Enabling it persists and round-trips through ControllerInfo.
	allow := true
	info, err = s.Store.UpdateController(params.UpdateControllerParams{AllowInsecureGARMAgent: &allow})
	s.Require().NoError(err)
	s.Require().True(info.AllowInsecureGARMAgent)
	info, err = s.Store.ControllerInfo()
	s.Require().NoError(err)
	s.Require().True(info.AllowInsecureGARMAgent)

	// A nil pointer leaves the stored value untouched.
	info, err = s.Store.UpdateController(params.UpdateControllerParams{})
	s.Require().NoError(err)
	s.Require().True(info.AllowInsecureGARMAgent)

	// It can be turned off again.
	allow = false
	info, err = s.Store.UpdateController(params.UpdateControllerParams{AllowInsecureGARMAgent: &allow})
	s.Require().NoError(err)
	s.Require().False(info.AllowInsecureGARMAgent)
}

func (s *CtrlTestSuite) TestControllerInfoResolvesCachedTools() {
	_, err := s.Store.InitController()
	s.Require().NoError(err)

	index := garmUtil.AgentReleaseIndex{
		SourceURL: "https://example.com/releases",
		Releases: garmUtil.GitHubReleases{
			{
				TagName: "v0.2.0",
				Assets: []garmUtil.GitHubReleaseAsset{
					{Name: "garm-agent-linux-amd64", Size: 2, DownloadURL: "https://example.com/v0.2.0/linux-amd64"},
				},
			},
			{
				TagName: "v0.1.0",
				Assets: []garmUtil.GitHubReleaseAsset{
					{Name: "garm-agent-linux-amd64", Size: 1, DownloadURL: "https://example.com/v0.1.0/linux-amd64"},
				},
			},
		},
	}
	data, err := garmUtil.MarshalReleaseIndex(index)
	s.Require().NoError(err)
	s.Require().NoError(s.Store.UpdateCachedGARMAgentReleases(data, time.Now()))

	// Without a pin, the cached tools come from the latest release.
	info, err := s.Store.ControllerInfo()
	s.Require().NoError(err)
	s.Require().Contains(info.CachedGARMAgentTools, "linux/amd64")
	s.Require().Equal("v0.2.0", info.CachedGARMAgentTools["linux/amd64"].Version)

	// Pinning resolves the cached tools to the pinned release.
	version := "v0.1.0"
	_, err = s.Store.UpdateController(params.UpdateControllerParams{GARMAgentVersion: &version})
	s.Require().NoError(err)
	info, err = s.Store.ControllerInfo()
	s.Require().NoError(err)
	s.Require().Equal("v0.1.0", info.CachedGARMAgentTools["linux/amd64"].Version)
}
