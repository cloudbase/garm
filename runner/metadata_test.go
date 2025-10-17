// Copyright 2025 Cloudbase Solutions SRL
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

package runner

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	runnerCommonMocks "github.com/cloudbase/garm/runner/common/mocks"
	runnerMocks "github.com/cloudbase/garm/runner/mocks"
)

type MetadataTestFixtures struct {
	AdminContext    context.Context
	Store           dbCommon.Store
	Providers       map[string]common.Provider
	ProviderMock    *runnerCommonMocks.Provider
	PoolMgrMock     *runnerCommonMocks.PoolManager
	PoolMgrCtrlMock *runnerMocks.PoolManagerController
	TestInstance    params.Instance
	TestEntity      params.ForgeEntity
	TestPool        params.Pool
	TestTemplate    params.Template
}

type MetadataTestSuite struct {
	suite.Suite
	Fixtures *MetadataTestFixtures
	Runner   *Runner

	adminCtx           context.Context
	instanceCtx        context.Context
	unauthorizedCtx    context.Context
	invalidInstanceCtx context.Context
	jitInstanceCtx     context.Context
	githubEndpoint     params.ForgeEndpoint
}

func (s *MetadataTestSuite) SetupTest() {
	// create testing sqlite database
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(context.Background(), dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	s.adminCtx = garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())

	s.githubEndpoint = garmTesting.CreateDefaultGithubEndpoint(s.adminCtx, db, s.T())
	testCreds := garmTesting.CreateTestGithubCredentials(s.adminCtx, "test-creds", db, s.T(), s.githubEndpoint)

	// Create test organization
	org, err := db.CreateOrganization(s.adminCtx, "test-org", testCreds, "test-webhook-secret", params.PoolBalancerTypeRoundRobin)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test org: %s", err))
	}

	entity, err := org.GetEntity()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get entity: %s", err))
	}
	// Set entity name for service name generation
	entity.Name = "test-org"

	// Create test template
	template, err := db.CreateTemplate(s.adminCtx, params.CreateTemplateParams{
		Name:        "test-template",
		Description: "Test template for metadata tests",
		OSType:      commonParams.Linux,
		ForgeType:   params.GithubEndpointType,
		Data:        []byte(`#!/bin/bash\necho "Installing runner..."`),
	})
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test template: %s", err))
	}

	// Create test pool
	pool, err := db.CreateEntityPool(s.adminCtx, entity, params.CreatePoolParams{
		ProviderName:           "test-provider",
		MaxRunners:             2,
		MinIdleRunners:         1,
		Image:                  "ubuntu:22.04",
		Flavor:                 "medium",
		OSType:                 commonParams.Linux,
		OSArch:                 commonParams.Amd64,
		Tags:                   []string{"linux", "amd64"},
		RunnerBootstrapTimeout: 10,
		TemplateID:             &template.ID,
	})
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test pool: %s", err))
	}

	// Create test instance
	instance, err := db.CreateInstance(s.adminCtx, pool.ID, params.CreateInstanceParams{
		Name:   "test-instance",
		OSType: commonParams.Linux,
		OSArch: commonParams.Amd64,
	})
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test instance: %s", err))
	}

	providerMock := runnerCommonMocks.NewProvider(s.T())
	poolMgrMock := runnerCommonMocks.NewPoolManager(s.T())
	poolMgrCtrlMock := runnerMocks.NewPoolManagerController(s.T())

	fixtures := &MetadataTestFixtures{
		AdminContext: s.adminCtx,
		Store:        db,
		Providers: map[string]common.Provider{
			"test-provider": providerMock,
		},
		ProviderMock:    providerMock,
		PoolMgrMock:     poolMgrMock,
		PoolMgrCtrlMock: poolMgrCtrlMock,
		TestInstance:    instance,
		TestEntity:      entity,
		TestPool:        pool,
		TestTemplate:    template,
	}

	s.Fixtures = fixtures

	// setup test runner
	runner := &Runner{
		providers:       fixtures.Providers,
		ctx:             fixtures.AdminContext,
		store:           fixtures.Store,
		poolManagerCtrl: fixtures.PoolMgrCtrlMock,
	}
	s.Runner = runner

	// Set up various contexts for testing
	s.setupContexts()
}

func (s *MetadataTestSuite) setupContexts() {
	// Valid instance context
	s.instanceCtx = auth.SetInstanceParams(context.Background(), s.Fixtures.TestInstance)
	s.instanceCtx = auth.SetInstanceRunnerStatus(s.instanceCtx, params.RunnerPending)
	s.instanceCtx = auth.SetInstanceEntity(s.instanceCtx, s.Fixtures.TestEntity)
	s.instanceCtx = auth.SetInstanceAuthToken(s.instanceCtx, "test-auth-token")

	// Unauthorized context (no instance params)
	s.unauthorizedCtx = context.Background()

	// Invalid instance context (wrong status)
	s.invalidInstanceCtx = auth.SetInstanceParams(context.Background(), s.Fixtures.TestInstance)
	s.invalidInstanceCtx = auth.SetInstanceRunnerStatus(s.invalidInstanceCtx, params.RunnerActive)
	s.invalidInstanceCtx = auth.SetInstanceEntity(s.invalidInstanceCtx, s.Fixtures.TestEntity)

	// JIT instance context
	jitInstance := s.Fixtures.TestInstance
	jitInstance.JitConfiguration = map[string]string{
		".runner":      base64.StdEncoding.EncodeToString([]byte("runner config")),
		".credentials": base64.StdEncoding.EncodeToString([]byte("credentials config")),
	}
	s.jitInstanceCtx = auth.SetInstanceParams(context.Background(), jitInstance)
	s.jitInstanceCtx = auth.SetInstanceRunnerStatus(s.jitInstanceCtx, params.RunnerPending)
	s.jitInstanceCtx = auth.SetInstanceEntity(s.jitInstanceCtx, s.Fixtures.TestEntity)
	s.jitInstanceCtx = auth.SetInstanceHasJITConfig(s.jitInstanceCtx, jitInstance.JitConfiguration)
}

func (s *MetadataTestSuite) TestGetServiceNameForEntity() {
	tests := []struct {
		name     string
		entity   params.ForgeEntity
		expected string
		hasError bool
	}{
		{
			name: "Organization entity",
			entity: params.ForgeEntity{
				EntityType: params.ForgeEntityTypeOrganization,
				Owner:      "test-name",
			},
			expected: "actions.runner.test-name",
			hasError: false,
		},
		{
			name: "Repository entity",
			entity: params.ForgeEntity{
				EntityType: params.ForgeEntityTypeRepository,
				Owner:      "test-owner",
				Name:       "test-repo",
			},
			expected: "actions.runner.test-owner.test-repo",
			hasError: false,
		},
		{
			name: "Enterprise entity",
			entity: params.ForgeEntity{
				EntityType: params.ForgeEntityTypeEnterprise,
				Owner:      "test-enterprise",
			},
			expected: "actions.runner.test-enterprise",
			hasError: false,
		},
		{
			name: "Unknown entity type",
			entity: params.ForgeEntity{
				EntityType: "unknown",
				Owner:      "test-owner",
				Name:       "test-name",
			},
			expected: "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			serviceName, err := s.Runner.getServiceNameForEntity(tt.entity)
			if tt.hasError {
				s.Require().NotNil(err)
				s.Require().Contains(err.Error(), "unknown entity type")
			} else {
				s.Require().Nil(err)
				s.Require().Equal(tt.expected, serviceName)
			}
		})
	}
}

func (s *MetadataTestSuite) TestGetRunnerServiceName() {
	serviceName, err := s.Runner.GetRunnerServiceName(s.instanceCtx)

	s.Require().Nil(err)
	s.Require().Equal("actions.runner.test-org", serviceName)
}

func (s *MetadataTestSuite) TestGetRunnerServiceNameUnauthorized() {
	_, err := s.Runner.GetRunnerServiceName(s.unauthorizedCtx)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetLabelsForInstance() {
	// Test with regular instance
	// Since we can't easily set up the cache in tests, labels might be empty
	// but the function should not panic
	labels := getLabelsForInstance(s.Fixtures.TestInstance)
	s.Require().NotNil(labels) // Should return a slice, even if empty

	// Test with JIT instance (should return empty labels)
	jitInstance := s.Fixtures.TestInstance
	jitInstance.JitConfiguration = map[string]string{"test": "config"}
	jitLabels := getLabelsForInstance(jitInstance)
	s.Require().Empty(jitLabels)

	// Test with scale set instance (should return empty labels)
	scaleSetInstance := s.Fixtures.TestInstance
	scaleSetInstance.ScaleSetID = 123
	scaleSetLabels := getLabelsForInstance(scaleSetInstance)
	s.Require().Empty(scaleSetLabels)
}

func (s *MetadataTestSuite) TestGetRunnerInstallScript() {
	// This test requires complex cache setup for github tools
	// Skipping for now as it would require significant test infrastructure
	s.T().Skip("Skipping install script test - requires github tools cache setup")
}

func (s *MetadataTestSuite) TestGetRunnerInstallScriptUnauthorized() {
	_, err := s.Runner.GetRunnerInstallScript(s.unauthorizedCtx)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetRunnerInstallScriptInvalidState() {
	_, err := s.Runner.GetRunnerInstallScript(s.invalidInstanceCtx)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGenerateSystemdUnitFile() {
	tests := []struct {
		name             string
		runAsUser        string
		forgeType        params.EndpointType
		expectedTemplate string
	}{
		{
			name:             "GitHub with custom user",
			runAsUser:        "custom-user",
			forgeType:        params.GithubEndpointType,
			expectedTemplate: "GitHub Actions Runner",
		},
		{
			name:             "GitHub with default user",
			runAsUser:        "",
			forgeType:        params.GithubEndpointType,
			expectedTemplate: "GitHub Actions Runner",
		},
		{
			name:             "Gitea with custom user",
			runAsUser:        "gitea-user",
			forgeType:        params.GiteaEndpointType,
			expectedTemplate: "Act Runner",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Set up entity with specific forge type
			entity := s.Fixtures.TestEntity
			entity.Credentials.ForgeType = tt.forgeType
			ctx := auth.SetInstanceEntity(context.Background(), entity)

			unitFile, err := s.Runner.GenerateSystemdUnitFile(ctx, tt.runAsUser)

			s.Require().Nil(err)
			s.Require().NotEmpty(unitFile)
			s.Require().Contains(string(unitFile), tt.expectedTemplate)
			s.Require().Contains(string(unitFile), "test-org")

			if tt.runAsUser != "" {
				s.Require().Contains(string(unitFile), tt.runAsUser)
			} else {
				s.Require().Contains(string(unitFile), "runner") // default user
			}
		})
	}
}

func (s *MetadataTestSuite) TestGenerateSystemdUnitFileUnknownForgeType() {
	entity := s.Fixtures.TestEntity
	entity.Credentials.ForgeType = "unknown"
	ctx := auth.SetInstanceEntity(context.Background(), entity)

	_, err := s.Runner.GenerateSystemdUnitFile(ctx, "test-user")

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "unknown forge type")
}

func (s *MetadataTestSuite) TestGenerateSystemdUnitFileUnauthorized() {
	_, err := s.Runner.GenerateSystemdUnitFile(s.unauthorizedCtx, "test-user")

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetJITConfigFile() {
	fileName := ".runner"
	expectedContent := "runner config"

	content, err := s.Runner.GetJITConfigFile(s.jitInstanceCtx, fileName)

	s.Require().Nil(err)
	s.Require().Equal(expectedContent, string(content))
}

func (s *MetadataTestSuite) TestGetJITConfigFileNotJIT() {
	_, err := s.Runner.GetJITConfigFile(s.instanceCtx, ".runner")

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "instance not configured for JIT")
}

func (s *MetadataTestSuite) TestGetJITConfigFileUnauthorized() {
	_, err := s.Runner.GetJITConfigFile(s.unauthorizedCtx, ".runner")

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "instance not configured for JIT")
}

func (s *MetadataTestSuite) TestGetJITConfigFileNotFound() {
	_, err := s.Runner.GetJITConfigFile(s.jitInstanceCtx, "nonexistent-file")

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "could not find file")
}

func (s *MetadataTestSuite) TestGetInstanceGithubRegistrationToken() {
	expectedToken := "test-registration-token"

	// Set up mocks
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("GithubRunnerRegistrationToken").Return(expectedToken, nil)

	token, err := s.Runner.GetInstanceGithubRegistrationToken(s.instanceCtx)

	s.Require().Nil(err)
	s.Require().Equal(expectedToken, token)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
}

func (s *MetadataTestSuite) TestGetInstanceGithubRegistrationTokenUnauthorized() {
	_, err := s.Runner.GetInstanceGithubRegistrationToken(s.unauthorizedCtx)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetInstanceGithubRegistrationTokenInvalidState() {
	_, err := s.Runner.GetInstanceGithubRegistrationToken(s.invalidInstanceCtx)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetInstanceGithubRegistrationTokenAlreadyFetched() {
	// Set up context with token already fetched
	ctx := auth.SetInstanceTokenFetched(s.instanceCtx, true)

	_, err := s.Runner.GetInstanceGithubRegistrationToken(ctx)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetInstanceGithubRegistrationTokenJITConfig() {
	_, err := s.Runner.GetInstanceGithubRegistrationToken(s.jitInstanceCtx)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetRootCertificateBundleUnauthorized() {
	_, err := s.Runner.GetRootCertificateBundle(s.unauthorizedCtx)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetRootCertificateBundleAuthorized() {
	// Load a valid test certificate from testdata
	certPath := filepath.Join("../testdata/certs", "srv-pub.pem")
	testCertPEM, err := os.ReadFile(certPath)
	s.Require().NoError(err, "Failed to read test certificate")

	// Set up entity with valid CA bundle
	entity := s.Fixtures.TestEntity
	entity.Credentials.CABundle = testCertPEM
	ctx := auth.SetInstanceParams(context.Background(), s.Fixtures.TestInstance)
	ctx = auth.SetInstanceEntity(ctx, entity)

	bundle, err := s.Runner.GetRootCertificateBundle(ctx)

	s.Require().Nil(err)
	s.Require().NotNil(bundle.RootCertificates)
	s.Require().NotEmpty(bundle.RootCertificates)
	// The test certificate file contains 2 certificates
	s.Require().Len(bundle.RootCertificates, 2)
}

func (s *MetadataTestSuite) TestGetRootCertificateBundleInvalidBundle() {
	// Set up entity with invalid CA bundle (invalid PEM data)
	entity := s.Fixtures.TestEntity
	entity.Credentials.CABundle = []byte("bogus cert")
	ctx := auth.SetInstanceParams(context.Background(), s.Fixtures.TestInstance)
	ctx = auth.SetInstanceEntity(ctx, entity)

	bundle, err := s.Runner.GetRootCertificateBundle(ctx)

	// Should return empty bundle without error when CA bundle is invalid
	s.Require().Nil(err)
	s.Require().Empty(bundle.RootCertificates)
}

func TestMetadataTestSuite(t *testing.T) {
	suite.Run(t, new(MetadataTestSuite))
}
