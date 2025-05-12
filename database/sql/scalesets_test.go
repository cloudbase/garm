package sql

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type ScaleSetsTestSuite struct {
	suite.Suite
	Store    dbCommon.Store
	adminCtx context.Context
	creds    params.GithubCredentials

	org        params.Organization
	repo       params.Repository
	enterprise params.Enterprise

	orgEntity        params.ForgeEntity
	repoEntity       params.ForgeEntity
	enterpriseEntity params.ForgeEntity
}

func (s *ScaleSetsTestSuite) SetupTest() {
	// create testing sqlite database
	ctx := context.Background()
	watcher.InitWatcher(ctx)

	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db

	adminCtx := garmTesting.ImpersonateAdminContext(ctx, db, s.T())
	s.adminCtx = adminCtx

	githubEndpoint := garmTesting.CreateDefaultGithubEndpoint(adminCtx, db, s.T())
	s.creds = garmTesting.CreateTestGithubCredentials(adminCtx, "new-creds", db, s.T(), githubEndpoint)

	// create an organization for testing purposes
	s.org, err = s.Store.CreateOrganization(s.adminCtx, "test-org", s.creds.Name, "test-webhookSecret", params.PoolBalancerTypeRoundRobin)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create org: %s", err))
	}

	s.repo, err = s.Store.CreateRepository(s.adminCtx, "test-org", "test-repo", s.creds.Name, "test-webhookSecret", params.PoolBalancerTypeRoundRobin)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create repo: %s", err))
	}

	s.enterprise, err = s.Store.CreateEnterprise(s.adminCtx, "test-enterprise", s.creds.Name, "test-webhookSecret", params.PoolBalancerTypeRoundRobin)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create enterprise: %s", err))
	}

	s.orgEntity, err = s.org.GetEntity()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get org entity: %s", err))
	}

	s.repoEntity, err = s.repo.GetEntity()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get repo entity: %s", err))
	}

	s.enterpriseEntity, err = s.enterprise.GetEntity()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get enterprise entity: %s", err))
	}

	s.T().Cleanup(func() {
		err := s.Store.DeleteOrganization(s.adminCtx, s.org.ID)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to delete org: %s", err))
		}
		err = s.Store.DeleteRepository(s.adminCtx, s.repo.ID)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to delete repo: %s", err))
		}
		err = s.Store.DeleteEnterprise(s.adminCtx, s.enterprise.ID)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to delete enterprise: %s", err))
		}
	})
}

func (s *ScaleSetsTestSuite) TearDownTest() {
	watcher.CloseWatcher()
}

func (s *ScaleSetsTestSuite) callback(old, newSet params.ScaleSet) error {
	s.Require().Equal(old.Name, "test-scaleset")
	s.Require().Equal(newSet.Name, "test-scaleset-updated")
	s.Require().Equal(old.OSType, commonParams.Linux)
	s.Require().Equal(newSet.OSType, commonParams.Windows)
	s.Require().Equal(old.OSArch, commonParams.Amd64)
	s.Require().Equal(newSet.OSArch, commonParams.Arm64)
	s.Require().Equal(old.ExtraSpecs, json.RawMessage(`{"test": 1}`))
	s.Require().Equal(newSet.ExtraSpecs, json.RawMessage(`{"test": 111}`))
	s.Require().Equal(old.MaxRunners, uint(10))
	s.Require().Equal(newSet.MaxRunners, uint(60))
	s.Require().Equal(old.MinIdleRunners, uint(5))
	s.Require().Equal(newSet.MinIdleRunners, uint(50))
	s.Require().Equal(old.Image, "test-image")
	s.Require().Equal(newSet.Image, "new-test-image")
	s.Require().Equal(old.Flavor, "test-flavor")
	s.Require().Equal(newSet.Flavor, "new-test-flavor")
	s.Require().Equal(old.GitHubRunnerGroup, "test-group")
	s.Require().Equal(newSet.GitHubRunnerGroup, "new-test-group")
	s.Require().Equal(old.RunnerPrefix.Prefix, "garm")
	s.Require().Equal(newSet.RunnerPrefix.Prefix, "test-prefix2")
	s.Require().Equal(old.Enabled, false)
	s.Require().Equal(newSet.Enabled, true)
	return nil
}

func (s *ScaleSetsTestSuite) TestScaleSetOperations() {
	// create a scale set for the organization
	createScaleSetPrams := params.CreateScaleSetParams{
		Name:              "test-scaleset",
		ProviderName:      "test-provider",
		MaxRunners:        10,
		MinIdleRunners:    5,
		Image:             "test-image",
		Flavor:            "test-flavor",
		OSType:            commonParams.Linux,
		OSArch:            commonParams.Amd64,
		ExtraSpecs:        json.RawMessage(`{"test": 1}`),
		GitHubRunnerGroup: "test-group",
	}

	var orgScaleSet params.ScaleSet
	var repoScaleSet params.ScaleSet
	var enterpriseScaleSet params.ScaleSet
	var err error

	s.T().Run("create org scaleset", func(_ *testing.T) {
		orgScaleSet, err = s.Store.CreateEntityScaleSet(s.adminCtx, s.orgEntity, createScaleSetPrams)
		s.Require().NoError(err)
		s.Require().NotNil(orgScaleSet)
		s.Require().Equal(orgScaleSet.Name, createScaleSetPrams.Name)
		s.T().Cleanup(func() {
			err := s.Store.DeleteScaleSetByID(s.adminCtx, orgScaleSet.ID)
			if err != nil {
				s.FailNow(fmt.Sprintf("failed to delete scaleset: %s", err))
			}
		})
	})

	s.T().Run("create repo scaleset", func(_ *testing.T) {
		repoScaleSet, err = s.Store.CreateEntityScaleSet(s.adminCtx, s.repoEntity, createScaleSetPrams)
		s.Require().NoError(err)
		s.Require().NotNil(repoScaleSet)
		s.Require().Equal(repoScaleSet.Name, createScaleSetPrams.Name)
		s.T().Cleanup(func() {
			err := s.Store.DeleteScaleSetByID(s.adminCtx, repoScaleSet.ID)
			if err != nil {
				s.FailNow(fmt.Sprintf("failed to delete scaleset: %s", err))
			}
		})
	})

	s.T().Run("create enterprise scaleset", func(_ *testing.T) {
		enterpriseScaleSet, err = s.Store.CreateEntityScaleSet(s.adminCtx, s.enterpriseEntity, createScaleSetPrams)
		s.Require().NoError(err)
		s.Require().NotNil(enterpriseScaleSet)
		s.Require().Equal(enterpriseScaleSet.Name, createScaleSetPrams.Name)

		s.T().Cleanup(func() {
			err := s.Store.DeleteScaleSetByID(s.adminCtx, enterpriseScaleSet.ID)
			if err != nil {
				s.FailNow(fmt.Sprintf("failed to delete scaleset: %s", err))
			}
		})
	})

	s.T().Run("create list all scalesets", func(_ *testing.T) {
		allScaleSets, err := s.Store.ListAllScaleSets(s.adminCtx)
		s.Require().NoError(err)
		s.Require().NotEmpty(allScaleSets)
		s.Require().Len(allScaleSets, 3)
	})

	s.T().Run("list repo scalesets", func(_ *testing.T) {
		repoScaleSets, err := s.Store.ListEntityScaleSets(s.adminCtx, s.repoEntity)
		s.Require().NoError(err)
		s.Require().NotEmpty(repoScaleSets)
		s.Require().Len(repoScaleSets, 1)
	})

	s.T().Run("list org scalesets", func(_ *testing.T) {
		orgScaleSets, err := s.Store.ListEntityScaleSets(s.adminCtx, s.orgEntity)
		s.Require().NoError(err)
		s.Require().NotEmpty(orgScaleSets)
		s.Require().Len(orgScaleSets, 1)
	})

	s.T().Run("list enterprise scalesets", func(_ *testing.T) {
		enterpriseScaleSets, err := s.Store.ListEntityScaleSets(s.adminCtx, s.enterpriseEntity)
		s.Require().NoError(err)
		s.Require().NotEmpty(enterpriseScaleSets)
		s.Require().Len(enterpriseScaleSets, 1)
	})

	s.T().Run("get repo scaleset by ID", func(_ *testing.T) {
		repoScaleSetByID, err := s.Store.GetScaleSetByID(s.adminCtx, repoScaleSet.ID)
		s.Require().NoError(err)
		s.Require().NotNil(repoScaleSetByID)
		s.Require().Equal(repoScaleSetByID.ID, repoScaleSet.ID)
		s.Require().Equal(repoScaleSetByID.Name, repoScaleSet.Name)
	})

	s.T().Run("get org scaleset by ID", func(_ *testing.T) {
		orgScaleSetByID, err := s.Store.GetScaleSetByID(s.adminCtx, orgScaleSet.ID)
		s.Require().NoError(err)
		s.Require().NotNil(orgScaleSetByID)
		s.Require().Equal(orgScaleSetByID.ID, orgScaleSet.ID)
		s.Require().Equal(orgScaleSetByID.Name, orgScaleSet.Name)
	})

	s.T().Run("get enterprise scaleset by ID", func(_ *testing.T) {
		enterpriseScaleSetByID, err := s.Store.GetScaleSetByID(s.adminCtx, enterpriseScaleSet.ID)
		s.Require().NoError(err)
		s.Require().NotNil(enterpriseScaleSetByID)
		s.Require().Equal(enterpriseScaleSetByID.ID, enterpriseScaleSet.ID)
		s.Require().Equal(enterpriseScaleSetByID.Name, enterpriseScaleSet.Name)
	})

	s.T().Run("get scaleset by ID not found", func(_ *testing.T) {
		_, err = s.Store.GetScaleSetByID(s.adminCtx, 999)
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "not found")
	})

	s.T().Run("Set scale set last message ID and desired count", func(_ *testing.T) {
		err = s.Store.SetScaleSetLastMessageID(s.adminCtx, orgScaleSet.ID, 20)
		s.Require().NoError(err)
		err = s.Store.SetScaleSetDesiredRunnerCount(s.adminCtx, orgScaleSet.ID, 5)
		s.Require().NoError(err)
		orgScaleSetByID, err := s.Store.GetScaleSetByID(s.adminCtx, orgScaleSet.ID)
		s.Require().NoError(err)
		s.Require().NotNil(orgScaleSetByID)
		s.Require().Equal(orgScaleSetByID.LastMessageID, int64(20))
		s.Require().Equal(orgScaleSetByID.DesiredRunnerCount, 5)
	})

	updateParams := params.UpdateScaleSetParams{
		Name: "test-scaleset-updated",
		RunnerPrefix: params.RunnerPrefix{
			Prefix: "test-prefix2",
		},
		OSType:            commonParams.Windows,
		OSArch:            commonParams.Arm64,
		ExtraSpecs:        json.RawMessage(`{"test": 111}`),
		Enabled:           garmTesting.Ptr(true),
		MaxRunners:        garmTesting.Ptr(uint(60)),
		MinIdleRunners:    garmTesting.Ptr(uint(50)),
		Image:             "new-test-image",
		Flavor:            "new-test-flavor",
		GitHubRunnerGroup: garmTesting.Ptr("new-test-group"),
	}

	s.T().Run("update repo scaleset", func(_ *testing.T) {
		newRepoScaleSet, err := s.Store.UpdateEntityScaleSet(s.adminCtx, s.repoEntity, repoScaleSet.ID, updateParams, s.callback)
		s.Require().NoError(err)
		s.Require().NotNil(newRepoScaleSet)
		s.Require().NoError(s.callback(repoScaleSet, newRepoScaleSet))
	})

	s.T().Run("update org scaleset", func(_ *testing.T) {
		newOrgScaleSet, err := s.Store.UpdateEntityScaleSet(s.adminCtx, s.orgEntity, orgScaleSet.ID, updateParams, s.callback)
		s.Require().NoError(err)
		s.Require().NotNil(newOrgScaleSet)
		s.Require().NoError(s.callback(orgScaleSet, newOrgScaleSet))
	})

	s.T().Run("update enterprise scaleset", func(_ *testing.T) {
		newEnterpriseScaleSet, err := s.Store.UpdateEntityScaleSet(s.adminCtx, s.enterpriseEntity, enterpriseScaleSet.ID, updateParams, s.callback)
		s.Require().NoError(err)
		s.Require().NotNil(newEnterpriseScaleSet)
		s.Require().NoError(s.callback(enterpriseScaleSet, newEnterpriseScaleSet))
	})

	s.T().Run("update scaleset not found", func(_ *testing.T) {
		_, err = s.Store.UpdateEntityScaleSet(s.adminCtx, s.enterpriseEntity, 99999, updateParams, s.callback)
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "not found")
	})

	s.T().Run("update scaleset with invalid entity", func(_ *testing.T) {
		_, err = s.Store.UpdateEntityScaleSet(s.adminCtx, params.ForgeEntity{}, enterpriseScaleSet.ID, params.UpdateScaleSetParams{}, nil)
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "missing entity id")
	})

	s.T().Run("Create repo scale set instance", func(_ *testing.T) {
		param := params.CreateInstanceParams{
			Name:              "test-instance",
			Status:            commonParams.InstancePendingCreate,
			RunnerStatus:      params.RunnerPending,
			OSType:            commonParams.Linux,
			OSArch:            commonParams.Amd64,
			CallbackURL:       "http://localhost:8080/callback",
			MetadataURL:       "http://localhost:8080/metadata",
			GitHubRunnerGroup: "test-group",
			JitConfiguration: map[string]string{
				"test": "test",
			},
			AgentID: 5,
		}

		instance, err := s.Store.CreateScaleSetInstance(s.adminCtx, repoScaleSet.ID, param)
		s.Require().NoError(err)
		s.Require().NotNil(instance)
		s.Require().Equal(instance.Name, param.Name)
		s.Require().Equal(instance.Status, param.Status)
		s.Require().Equal(instance.RunnerStatus, param.RunnerStatus)
		s.Require().Equal(instance.OSType, param.OSType)
		s.Require().Equal(instance.OSArch, param.OSArch)
		s.Require().Equal(instance.CallbackURL, param.CallbackURL)
		s.Require().Equal(instance.MetadataURL, param.MetadataURL)
		s.Require().Equal(instance.GitHubRunnerGroup, param.GitHubRunnerGroup)
		s.Require().Equal(instance.JitConfiguration, param.JitConfiguration)
		s.Require().Equal(instance.AgentID, param.AgentID)

		s.T().Cleanup(func() {
			err := s.Store.DeleteInstanceByName(s.adminCtx, instance.Name)
			if err != nil {
				s.FailNow(fmt.Sprintf("failed to delete scaleset instance: %s", err))
			}
		})
	})

	s.T().Run("List repo scale set instances", func(_ *testing.T) {
		instances, err := s.Store.ListScaleSetInstances(s.adminCtx, repoScaleSet.ID)
		s.Require().NoError(err)
		s.Require().NotEmpty(instances)
		s.Require().Len(instances, 1)
	})
}

func TestScaleSetsTestSuite(t *testing.T) {
	suite.Run(t, new(ScaleSetsTestSuite))
}
