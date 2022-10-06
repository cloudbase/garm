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
	Orgs                 []params.Organization
	CreateOrgParams      params.CreateOrgParams
	CreatePoolParams     params.CreatePoolParams
	CreateInstanceParams params.CreateInstanceParams
	UpdateRepoParams     params.UpdateRepositoryParams
	UpdatePoolParams     params.UpdatePoolParams
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

func (s *OrgTestSuite) equalPoolsByID(expected, actual []params.Pool) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].ID > expected[j].ID })
	sort.Slice(actual, func(i, j int) bool { return actual[i].ID > actual[j].ID })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].ID, actual[i].ID)
	}
}

func (s *OrgTestSuite) equalInstancesByName(expected, actual []params.Instance) {
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
	var maxRunners uint = 30
	var minIdleRunners uint = 10
	fixtures := &OrgTestFixtures{
		Orgs: orgs,
		CreateOrgParams: params.CreateOrgParams{
			Name:            "new-test-org",
			CredentialsName: "new-creds",
			WebhookSecret:   "new-webhook-secret",
		},
		CreatePoolParams: params.CreatePoolParams{
			ProviderName:   "test-provider",
			MaxRunners:     3,
			MinIdleRunners: 1,
			Image:          "test-image",
			Flavor:         "test-flavor",
			OSType:         "linux",
			OSArch:         "amd64",
			Tags:           []string{"self-hosted", "arm64", "linux"},
		},
		CreateInstanceParams: params.CreateInstanceParams{
			Name:   "test-instance-name",
			OSType: "linux",
		},
		UpdateRepoParams: params.UpdateRepositoryParams{
			CredentialsName: "test-update-creds",
			WebhookSecret:   "test-update-repo-webhook-secret",
		},
		UpdatePoolParams: params.UpdatePoolParams{
			MaxRunners:     &maxRunners,
			MinIdleRunners: &minIdleRunners,
			Image:          "test-update-image",
			Flavor:         "test-update-flavor",
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

func (s *OrgTestSuite) TestCreateOrganizationPool() {
	pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)

	s.Require().Nil(err)

	org, err := s.Store.GetOrganizationByID(context.Background(), s.Fixtures.Orgs[0].ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot get org by ID: %v", err))
	}
	s.Require().Equal(1, len(org.Pools))
	s.Require().Equal(pool.ID, org.Pools[0].ID)
	s.Require().Equal(s.Fixtures.CreatePoolParams.ProviderName, org.Pools[0].ProviderName)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MaxRunners, org.Pools[0].MaxRunners)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MinIdleRunners, org.Pools[0].MinIdleRunners)
}

func (s *OrgTestSuite) TestCreateOrganizationPoolMissingTags() {
	s.Fixtures.CreatePoolParams.Tags = []string{}

	_, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("no tags specified", err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationPoolInvalidOrgID() {
	_, err := s.Store.CreateOrganizationPool(context.Background(), "dummy-org-id", s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestListOrgPools() {
	orgPools := []params.Pool{}
	for i := 1; i <= 2; i++ {
		s.Fixtures.CreatePoolParams.Flavor = fmt.Sprintf("test-flavor-%v", i)
		pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
		}
		orgPools = append(orgPools, pool)
	}

	pools, err := s.Store.ListOrgPools(context.Background(), s.Fixtures.Orgs[0].ID)

	s.Require().Nil(err)
	s.equalPoolsByID(orgPools, pools)
}

func (s *OrgTestSuite) TestListOrgPoolsInvalidOrgID() {
	_, err := s.Store.ListOrgPools(context.Background(), "dummy-org-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching pools: fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestGetOrganizationPool() {
	pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}

	orgPool, err := s.Store.GetOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, pool.ID)

	s.Require().Nil(err)
	s.Require().Equal(orgPool.ID, pool.ID)
}

func (s *OrgTestSuite) TestGetOrganizationPoolInvalidOrgID() {
	_, err := s.Store.GetOrganizationPool(context.Background(), "dummy-org-id", "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrganizationPool() {
	pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}

	err = s.Store.DeleteOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, pool.ID)

	s.Require().Nil(err)
	_, err = s.Store.GetOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, pool.ID)
	s.Require().Equal("fetching pool: not found", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrganizationPoolInvalidOrgID() {
	err := s.Store.DeleteOrganizationPool(context.Background(), "dummy-org-id", "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("looking up org pool: fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestFindOrganizationPoolByTagsMissingTags() {
	tags := []string{}

	_, err := s.Store.FindOrganizationPoolByTags(context.Background(), s.Fixtures.Orgs[0].ID, tags)

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: missing tags", err.Error())
}

func (s *OrgTestSuite) TestListOrgInstances() {
	pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}
	poolInstances := []params.Instance{}
	for i := 1; i <= 3; i++ {
		s.Fixtures.CreateInstanceParams.Name = fmt.Sprintf("test-org-%v", i)
		instance, err := s.Store.CreateInstance(context.Background(), pool.ID, s.Fixtures.CreateInstanceParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
		}
		poolInstances = append(poolInstances, instance)
	}

	instances, err := s.Store.ListOrgInstances(context.Background(), s.Fixtures.Orgs[0].ID)

	s.Require().Nil(err)
	s.equalInstancesByName(poolInstances, instances)
}

func (s *OrgTestSuite) TestListOrgInstancesInvalidOrgID() {
	_, err := s.Store.ListOrgInstances(context.Background(), "dummy-org-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestUpdateOrganizationPool() {
	pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}

	pool, err = s.Store.UpdateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, pool.ID, s.Fixtures.UpdatePoolParams)

	s.Require().Nil(err)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MaxRunners, pool.MaxRunners)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MinIdleRunners, pool.MinIdleRunners)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Image, pool.Image)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Flavor, pool.Flavor)
}

func (s *OrgTestSuite) TestUpdateOrganizationPoolInvalidOrgID() {
	_, err := s.Store.UpdateOrganizationPool(context.Background(), "dummy-org-id", "dummy-pool-id", s.Fixtures.UpdatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: fetching org: parsing id: invalid request", err.Error())
}

func TestOrgTestSuite(t *testing.T) {
	suite.Run(t, new(OrgTestSuite))
}
