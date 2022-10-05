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
	"sort"

	dbCommon "garm/database/common"
	garmTesting "garm/internal/testing"
	"garm/params"
	"testing"

	"github.com/stretchr/testify/suite"
)

type OrgTestFixtures struct {
	Orgs             []params.Organization
	CreateOrgParams  params.CreateOrgParams
	UpdateRepoParams params.UpdateRepositoryParams
}

type OrgTestSuite struct {
	suite.Suite
	Store    dbCommon.Store
	Fixtures *OrgTestFixtures
}

func (s *OrgTestSuite) equalOrgsByName(expected, actual []params.Organization) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].Name > expected[j].Name })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Name > actual[j].Name })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].Name, actual[i].Name)
	}
}

func (s *OrgTestSuite) SetupTest() {
	// create testing sqlite database
	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db

	// create some organization objects in the database, for testing purposes
	orgs := []params.Organization{}
	for i := 1; i <= 3; i++ {
		org, err := db.CreateOrganization(
			context.Background(),
			fmt.Sprintf("test-org-%d", i),
			fmt.Sprintf("test-creds-%d", i),
			fmt.Sprintf("test-webhook-secret-%d", i),
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create database object (test-org-%d)", i))
		}
		orgs = append(orgs, org)
	}

	// setup test fixtures
	fixtures := &OrgTestFixtures{
		Orgs: orgs,
		CreateOrgParams: params.CreateOrgParams{
			Name:            "new-test-org",
			CredentialsName: "new-creds",
			WebhookSecret:   "new-webhook-secret",
		},
		UpdateRepoParams: params.UpdateRepositoryParams{
			CredentialsName: "test-update-creds",
			WebhookSecret:   "test-update-repo-webhook-secret",
		},
	}
	s.Fixtures = fixtures
}

func (s *OrgTestSuite) TestCreateOrganization() {
	// call tested function
	org, err := s.Store.CreateOrganization(
		context.Background(),
		s.Fixtures.CreateOrgParams.Name,
		s.Fixtures.CreateOrgParams.CredentialsName,
		s.Fixtures.CreateOrgParams.WebhookSecret)

	// assertions
	s.Require().Nil(err)
	storeOrg, err := s.Store.GetOrganizationByID(context.Background(), org.ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get organization by id: %v", err))
	}
	s.Require().Equal(storeOrg.Name, org.Name)
	s.Require().Equal(storeOrg.CredentialsName, org.CredentialsName)
	s.Require().Equal(storeOrg.WebhookSecret, org.WebhookSecret)
}

func (s *OrgTestSuite) TestCreateOrganizationInvalidDBPassphrase() {
	cfg := garmTesting.GetTestSqliteDBConfig(s.T())
	conn, err := newDBConn(cfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	// make sure we use a 'sqlDatabase' struct with a wrong 'cfg.Passphrase'
	cfg.Passphrase = "wrong-passphrase" // it must have a size different than 32
	sqlDB := &sqlDatabase{
		conn: conn,
		cfg:  cfg,
	}

	_, err = sqlDB.CreateOrganization(
		context.Background(),
		s.Fixtures.CreateOrgParams.Name,
		s.Fixtures.CreateOrgParams.CredentialsName,
		s.Fixtures.CreateOrgParams.WebhookSecret)

	s.Require().NotNil(err)
	s.Require().Equal("failed to encrypt string", err.Error())
}

func (s *OrgTestSuite) TestGetOrganization() {
	org, err := s.Store.GetOrganization(context.Background(), s.Fixtures.Orgs[0].Name)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Orgs[0].Name, org.Name)
	s.Require().Equal(s.Fixtures.Orgs[0].ID, org.ID)
}

func (s *OrgTestSuite) TestGetOrganizationCaseInsensitive() {
	org, err := s.Store.GetOrganization(context.Background(), "TeSt-oRg-1")

	s.Require().Nil(err)
	s.Require().Equal("test-org-1", org.Name)
}

func (s *OrgTestSuite) TestGetOrganizationNotFound() {
	_, err := s.Store.GetOrganization(context.Background(), "dummy-name")

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: not found", err.Error())
}

func (s *OrgTestSuite) TestListOrganizations() {
	orgs, err := s.Store.ListOrganizations(context.Background())

	s.Require().Nil(err)
	s.equalOrgsByName(s.Fixtures.Orgs, orgs)
}

func (s *OrgTestSuite) TestDeleteOrganization() {
	err := s.Store.DeleteOrganization(context.Background(), s.Fixtures.Orgs[0].ID)

	s.Require().Nil(err)
	_, err = s.Store.GetOrganizationByID(context.Background(), s.Fixtures.Orgs[0].ID)
	s.Require().NotNil(err)
	s.Require().Equal("fetching org: not found", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrganizationInvalidOrgID() {
	err := s.Store.DeleteOrganization(context.Background(), "dummy-org-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestUpdateOrganization() {
	org, err := s.Store.UpdateOrganization(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.UpdateRepoParams)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.CredentialsName, org.CredentialsName)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.WebhookSecret, org.WebhookSecret)
}

func (s *OrgTestSuite) TestUpdateOrganizationInvalidOrgID() {
	_, err := s.Store.UpdateOrganization(context.Background(), "dummy-org-id", s.Fixtures.UpdateRepoParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestGetOrganizationByID() {
	org, err := s.Store.GetOrganizationByID(context.Background(), s.Fixtures.Orgs[0].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Orgs[0].ID, org.ID)
}

func (s *OrgTestSuite) TestGetOrganizationByIDInvalidOrgID() {
	_, err := s.Store.GetOrganizationByID(context.Background(), "dummy-org-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: parsing id: invalid request", err.Error())
}

func TestOrgTestSuite(t *testing.T) {
	suite.Run(t, new(OrgTestSuite))
}
