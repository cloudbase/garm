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

	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing" //nolint:typecheck
)

type CtrlTestSuite struct {
	suite.Suite
	Store dbCommon.Store
}

func (s *CtrlTestSuite) SetupTest() {
	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db
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
