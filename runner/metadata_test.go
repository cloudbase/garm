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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	runnerCommonMocks "github.com/cloudbase/garm/runner/common/mocks"
	runnerMocks "github.com/cloudbase/garm/runner/mocks"
)

// mockTokenGetter is a simple mock implementation of auth.InstanceTokenGetter
type mockTokenGetter struct{}

func (m *mockTokenGetter) NewInstanceJWTToken(_ params.Instance, _ params.ForgeEntity, _ uint) (string, error) {
	return "mock-instance-jwt-token", nil
}

func (m *mockTokenGetter) NewAgentJWTToken(_ params.Instance, _ params.ForgeEntity) (string, error) {
	return "mock-agent-jwt-token", nil
}

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
	org, err := db.CreateOrganization(s.adminCtx, "test-org", testCreds, "test-webhook-secret", params.PoolBalancerTypeRoundRobin, false)
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

	// setup test runner with mock token getter
	runner := &Runner{
		providers:       fixtures.Providers,
		ctx:             fixtures.AdminContext,
		store:           fixtures.Store,
		poolManagerCtrl: fixtures.PoolMgrCtrlMock,
		tokenGetter:     &mockTokenGetter{},
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
	// Set up github tools cache for the entity
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/actions-runner-linux-x64-2.0.0.tar.gz"),
			Filename:       garmTesting.Ptr("actions-runner-linux-x64-2.0.0.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	script, err := s.Runner.GetRunnerInstallScript(s.instanceCtx)

	s.Require().Nil(err)
	s.Require().NotEmpty(script)
	// Should contain the template content
	s.Require().Contains(string(script), "Installing runner")
}

func (s *MetadataTestSuite) TestGetRunnerInstallScriptUnauthorized() {
	_, err := s.Runner.GetRunnerInstallScript(s.unauthorizedCtx)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetRunnerInstallScriptInvalidState() {
	// Set up cache even for invalid state to ensure it's the state check that fails
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/actions-runner.tar.gz"),
			Filename:       garmTesting.Ptr("actions-runner.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	_, err := s.Runner.GetRunnerInstallScript(s.invalidInstanceCtx)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetRunnerInstallScriptNoToolsInCache() {
	// Don't set up cache - should fail with tools not found error
	_, err := s.Runner.GetRunnerInstallScript(s.instanceCtx)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "failed to get tools")
}

func (s *MetadataTestSuite) TestGetRunnerInstallScriptWithExtraSpecs() {
	// Set up github tools cache
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/actions-runner.tar.gz"),
			Filename:       garmTesting.Ptr("actions-runner.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	// Update pool with extra specs containing custom context
	extraSpecs := json.RawMessage(`{"extra_context": {"custom_var": "custom_value"}}`)
	pool, err := s.Fixtures.Store.UpdateEntityPool(s.adminCtx, s.Fixtures.TestEntity, s.Fixtures.TestPool.ID, params.UpdatePoolParams{
		ExtraSpecs: extraSpecs,
	})
	s.Require().NoError(err)
	s.Require().NotNil(pool)

	script, err := s.Runner.GetRunnerInstallScript(s.instanceCtx)

	s.Require().Nil(err)
	s.Require().NotEmpty(script)
}

func (s *MetadataTestSuite) TestGetRunnerInstallScriptNoTemplate() {
	// Set up github tools cache
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/actions-runner.tar.gz"),
			Filename:       garmTesting.Ptr("actions-runner.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	// Create a new pool without a template
	poolNoTemplate, err := s.Fixtures.Store.CreateEntityPool(s.adminCtx, s.Fixtures.TestEntity, params.CreatePoolParams{
		ProviderName:           "test-provider",
		MaxRunners:             2,
		MinIdleRunners:         1,
		Image:                  "ubuntu:22.04",
		Flavor:                 "medium",
		OSType:                 commonParams.Linux,
		OSArch:                 commonParams.Amd64,
		Tags:                   []string{"linux", "amd64"},
		RunnerBootstrapTimeout: 10,
		// No TemplateID specified
	})
	s.Require().NoError(err)

	// Create instance with this pool
	instance, err := s.Fixtures.Store.CreateInstance(s.adminCtx, poolNoTemplate.ID, params.CreateInstanceParams{
		Name:   "test-instance-no-template",
		OSType: commonParams.Linux,
		OSArch: commonParams.Amd64,
	})
	s.Require().NoError(err)

	// Create context for this instance
	ctx := auth.SetInstanceParams(context.Background(), instance)
	ctx = auth.SetInstanceRunnerStatus(ctx, params.RunnerPending)
	ctx = auth.SetInstanceEntity(ctx, s.Fixtures.TestEntity)
	ctx = auth.SetInstanceAuthToken(ctx, "test-auth-token")

	_, err = s.Runner.GetRunnerInstallScript(ctx)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "no template associated")
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

func (s *MetadataTestSuite) TestGetInstanceMetadata() {
	// Set up github tools cache for the entity
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/actions-runner-linux-x64-2.0.0.tar.gz"),
			Filename:       garmTesting.Ptr("actions-runner-linux-x64-2.0.0.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	metadata, err := s.Runner.GetInstanceMetadata(s.instanceCtx)

	s.Require().Nil(err)
	s.Require().NotEmpty(metadata.RunnerName)
	s.Require().Equal(s.Fixtures.TestInstance.Name, metadata.RunnerName)
	s.Require().Equal(params.GithubEndpointType, metadata.ForgeType)
	s.Require().False(metadata.JITEnabled)
	s.Require().False(metadata.AgentMode)
	s.Require().NotNil(metadata.RunnerTools)
	// Metadata access details are populated from instance
	s.Require().NotNil(metadata.MetadataAccess)
}

func (s *MetadataTestSuite) TestGetInstanceMetadataUnauthorized() {
	_, err := s.Runner.GetInstanceMetadata(s.unauthorizedCtx)
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetInstanceMetadataInvalidState() {
	_, err := s.Runner.GetInstanceMetadata(s.invalidInstanceCtx)
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetInstanceMetadataNoPoolOrScaleSet() {
	// Create instance without pool or scale set
	instanceNoPool := s.Fixtures.TestInstance
	instanceNoPool.PoolID = ""
	instanceNoPool.ScaleSetID = 0

	ctx := auth.SetInstanceParams(context.Background(), instanceNoPool)
	ctx = auth.SetInstanceRunnerStatus(ctx, params.RunnerPending)

	_, err := s.Runner.GetInstanceMetadata(ctx)
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetInstanceMetadataWithJIT() {
	// Set up github tools cache
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/runner.tar.gz"),
			Filename:       garmTesting.Ptr("runner.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	metadata, err := s.Runner.GetInstanceMetadata(s.jitInstanceCtx)

	s.Require().Nil(err)
	s.Require().True(metadata.JITEnabled)
	s.Require().NotNil(metadata.RunnerTools)
}

func (s *MetadataTestSuite) TestGetInstanceMetadataWithExtraSpecs() {
	// Set up github tools cache
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/runner.tar.gz"),
			Filename:       garmTesting.Ptr("runner.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	// Update pool with extra specs
	extraSpecs := json.RawMessage(`{"custom_key": "custom_value", "another_key": 123}`)
	_, err := s.Fixtures.Store.UpdateEntityPool(s.adminCtx, s.Fixtures.TestEntity, s.Fixtures.TestPool.ID, params.UpdatePoolParams{
		ExtraSpecs: extraSpecs,
	})
	s.Require().NoError(err)

	metadata, err := s.Runner.GetInstanceMetadata(s.instanceCtx)

	s.Require().Nil(err)
	s.Require().NotNil(metadata.ExtraSpecs)
	s.Require().Contains(metadata.ExtraSpecs, "custom_key")
	s.Require().Equal("custom_value", metadata.ExtraSpecs["custom_key"])
}

func (s *MetadataTestSuite) TestGetInstanceMetadataBasicFields() {
	// Test that all basic metadata fields are populated correctly
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/runner.tar.gz"),
			Filename:       garmTesting.Ptr("runner.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	metadata, err := s.Runner.GetInstanceMetadata(s.instanceCtx)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.TestInstance.Name, metadata.RunnerName)
	s.Require().NotEmpty(metadata.RunnerRegistrationURL)
	s.Require().False(metadata.AgentShellEnabled)
}

func (s *MetadataTestSuite) TestGetInstanceMetadataWithAgentModeNoTools() {
	// Set up runner tools cache
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/runner.tar.gz"),
			Filename:       garmTesting.Ptr("runner.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	// Enable agent mode on the organization
	agentMode := true
	_, err := s.Fixtures.Store.UpdateOrganization(s.adminCtx, s.Fixtures.TestEntity.ID, params.UpdateEntityParams{
		AgentMode: &agentMode,
	})
	s.Require().NoError(err)

	metadata, err := s.Runner.GetInstanceMetadata(s.instanceCtx)

	s.Require().Nil(err)
	// AgentMode should be disabled because no agent tools are available
	s.Require().False(metadata.AgentMode, "AgentMode should be false when no agent tools available")
	s.Require().Nil(metadata.AgentTools)
	s.Require().Empty(metadata.AgentToken)
}

func (s *MetadataTestSuite) TestGetInstanceMetadataAgentModeDisabledByDefault() {
	// Test that agent mode is disabled by default
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/runner.tar.gz"),
			Filename:       garmTesting.Ptr("runner.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	metadata, err := s.Runner.GetInstanceMetadata(s.instanceCtx)

	s.Require().Nil(err)
	s.Require().False(metadata.AgentMode)
	s.Require().Nil(metadata.AgentTools)
	s.Require().Empty(metadata.AgentToken)
}

func (s *MetadataTestSuite) TestGetInstanceMetadataWithAgentModeToolsCountZero() {
	// Set up runner tools cache
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/runner.tar.gz"),
			Filename:       garmTesting.Ptr("runner.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	// Enable agent mode on the organization
	agentMode := true
	_, err := s.Fixtures.Store.UpdateOrganization(s.adminCtx, s.Fixtures.TestEntity.ID, params.UpdateEntityParams{
		AgentMode: &agentMode,
	})
	s.Require().NoError(err)

	// GetGARMTools will search for files with category=garm-agent tag
	// Since no such files exist, it returns TotalCount=0
	metadata, err := s.Runner.GetInstanceMetadata(s.instanceCtx)

	s.Require().Nil(err)
	// AgentMode should be disabled because TotalCount is 0
	s.Require().False(metadata.AgentMode, "AgentMode should be false when GetGARMTools returns TotalCount=0")
	s.Require().Nil(metadata.AgentTools)
	s.Require().Empty(metadata.AgentToken)
}

func (s *MetadataTestSuite) TestGetInstanceMetadataWithAgentModeGetToolsReturnsNotFoundError() {
	// Set up runner tools cache
	tools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/runner.tar.gz"),
			Filename:       garmTesting.Ptr("runner.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, tools)

	// Enable agent mode on the organization
	agentMode := true
	_, err := s.Fixtures.Store.UpdateOrganization(s.adminCtx, s.Fixtures.TestEntity.ID, params.UpdateEntityParams{
		AgentMode: &agentMode,
	})
	s.Require().NoError(err)

	// When GetGARMTools returns ErrNotFound (which happens when TotalCount=0 and no files found),
	// it should log the error but continue and disable AgentMode
	metadata, err := s.Runner.GetInstanceMetadata(s.instanceCtx)

	s.Require().Nil(err)
	// Should continue execution and disable AgentMode
	s.Require().False(metadata.AgentMode)
	s.Require().Nil(metadata.AgentTools)
	s.Require().Empty(metadata.AgentToken)
}

func (s *MetadataTestSuite) TestGetInstanceMetadataWithAgentModeAndToolsAvailable() {
	// Set up runner tools cache for GitHub runner
	runnerTools := []commonParams.RunnerApplicationDownload{
		{
			OS:             garmTesting.Ptr("linux"),
			Architecture:   garmTesting.Ptr("x64"),
			DownloadURL:    garmTesting.Ptr("https://example.com/runner.tar.gz"),
			Filename:       garmTesting.Ptr("runner.tar.gz"),
			SHA256Checksum: garmTesting.Ptr("abc123"),
		},
	}
	cache.SetGithubToolsCache(s.Fixtures.TestEntity, runnerTools)

	// Enable agent mode on the organization
	agentMode := true
	_, err := s.Fixtures.Store.UpdateOrganization(s.adminCtx, s.Fixtures.TestEntity.ID, params.UpdateEntityParams{
		AgentMode: &agentMode,
	})
	s.Require().NoError(err)

	// Create GARM agent tools using the CreateGARMTool API
	agentBinary := []byte("fake garm agent binary content")
	agentToolParam := params.CreateGARMToolParams{
		Name:        "garm-agent-linux-amd64",
		Description: "GARM agent for Linux AMD64",
		Size:        int64(len(agentBinary)),
		OSType:      commonParams.Linux,
		OSArch:      commonParams.Amd64,
		Version:     "v1.0.0",
	}

	agentTool, err := s.Runner.CreateGARMTool(s.adminCtx, agentToolParam, bytes.NewReader(agentBinary))
	s.Require().NoError(err)
	s.Require().NotNil(agentTool)

	// Now GetInstanceMetadata should succeed with AgentMode enabled
	metadata, err := s.Runner.GetInstanceMetadata(s.instanceCtx)

	s.Require().Nil(err)
	// AgentMode should remain enabled because tools are available
	s.Require().True(metadata.AgentMode, "AgentMode should be true when agent tools are available")
	// Agent tools should be populated
	s.Require().NotNil(metadata.AgentTools, "AgentTools should be populated")
	s.Require().Equal(agentTool.ID, metadata.AgentTools.ID)
	s.Require().Equal("garm-agent-linux-amd64", metadata.AgentTools.Name)
	s.Require().Equal(commonParams.OSType("linux"), metadata.AgentTools.OSType)
	s.Require().Equal(commonParams.OSArch("amd64"), metadata.AgentTools.OSArch)
	s.Require().Equal("v1.0.0", metadata.AgentTools.Version)
	s.Require().NotEmpty(metadata.AgentTools.DownloadURL)
	// Agent token should be generated
	s.Require().NotEmpty(metadata.AgentToken, "AgentToken should be generated when tools available")
}

func (s *MetadataTestSuite) TestFileObjectToGARMTool() {
	tests := []struct {
		name        string
		fileObject  params.FileObject
		downloadURL string
		wantErr     bool
		errMsg      string
	}{
		{
			name: "Valid file object with all tags",
			fileObject: params.FileObject{
				ID:          1,
				Name:        "garm-agent-linux-amd64",
				Size:        1024,
				SHA256:      "abc123",
				Description: "GARM agent for Linux AMD64",
				FileType:    "binary",
				Tags:        []string{"version=1.0.0", "os_type=linux", "os_arch=amd64"},
			},
			downloadURL: "http://example.com/download",
			wantErr:     false,
		},
		{
			name: "Missing version tag",
			fileObject: params.FileObject{
				ID:   2,
				Name: "garm-agent",
				Tags: []string{"os_type=linux", "os_arch=amd64"},
			},
			downloadURL: "http://example.com/download",
			wantErr:     true,
			errMsg:      "missing version",
		},
		{
			name: "Missing os_type tag",
			fileObject: params.FileObject{
				ID:   3,
				Name: "garm-agent",
				Tags: []string{"version=1.0.0", "os_arch=amd64"},
			},
			downloadURL: "http://example.com/download",
			wantErr:     true,
			errMsg:      "missing os_type",
		},
		{
			name: "Missing os_arch tag",
			fileObject: params.FileObject{
				ID:   4,
				Name: "garm-agent",
				Tags: []string{"version=1.0.0", "os_type=linux"},
			},
			downloadURL: "http://example.com/download",
			wantErr:     true,
			errMsg:      "missing os_arch",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result, err := fileObjectToGARMTool(tt.fileObject, tt.downloadURL)

			if tt.wantErr {
				s.Require().NotNil(err)
				s.Require().Contains(err.Error(), tt.errMsg)
			} else {
				s.Require().Nil(err)
				s.Require().Equal(tt.fileObject.ID, result.ID)
				s.Require().Equal(tt.fileObject.Name, result.Name)
				s.Require().Equal(tt.fileObject.Size, result.Size)
				s.Require().Equal(tt.fileObject.SHA256, result.SHA256SUM)
				s.Require().Equal(tt.downloadURL, result.DownloadURL)
			}
		})
	}
}

func (s *MetadataTestSuite) TestGetGARMTools() {
	// GetGARMTools requires file objects in database
	// This is tested in file_store_test.go and garm_tools_test.go
	// Here we just test the authorization paths
	_, err := s.Runner.GetGARMTools(s.instanceCtx, 0, 25)

	// Should not error on authorization (might have no results)
	s.Require().NoError(err)
}

func (s *MetadataTestSuite) TestGetGARMToolsUnauthorized() {
	_, err := s.Runner.GetGARMTools(s.unauthorizedCtx, 0, 25)
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetGARMToolsInvalidState() {
	_, err := s.Runner.GetGARMTools(s.invalidInstanceCtx, 0, 25)
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestShowGARMToolsUnauthorized() {
	_, err := s.Runner.ShowGARMTools(s.unauthorizedCtx, 1)
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestShowGARMToolsInvalidState() {
	_, err := s.Runner.ShowGARMTools(s.invalidInstanceCtx, 1)
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetGARMToolsReadHandlerUnauthorized() {
	_, err := s.Runner.GetGARMToolsReadHandler(s.unauthorizedCtx, 1)
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestValidateInstanceState() {
	tests := []struct {
		name    string
		ctx     context.Context
		wantErr bool
	}{
		{
			name:    "Valid pending instance",
			ctx:     s.instanceCtx,
			wantErr: false,
		},
		{
			name: "Valid installing instance",
			ctx: func() context.Context {
				ctx := auth.SetInstanceParams(context.Background(), s.Fixtures.TestInstance)
				ctx = auth.SetInstanceRunnerStatus(ctx, params.RunnerInstalling)
				return ctx
			}(),
			wantErr: false,
		},
		{
			name:    "Invalid state - active",
			ctx:     s.invalidInstanceCtx,
			wantErr: true,
		},
		{
			name:    "Unauthorized - no instance",
			ctx:     s.unauthorizedCtx,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			instance, err := validateInstanceState(tt.ctx)

			if tt.wantErr {
				s.Require().NotNil(err)
				s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
			} else {
				s.Require().Nil(err)
				s.Require().NotEmpty(instance.Name)
			}
		})
	}
}

func (s *MetadataTestSuite) TestGetJITConfigFileInvalidState() {
	ctx := auth.SetInstanceParams(context.Background(), s.Fixtures.TestInstance)
	ctx = auth.SetInstanceRunnerStatus(ctx, params.RunnerActive)

	jitInstance := s.Fixtures.TestInstance
	jitInstance.JitConfiguration = map[string]string{
		".runner": base64.StdEncoding.EncodeToString([]byte("runner config")),
	}
	ctx = auth.SetInstanceParams(ctx, jitInstance)
	ctx = auth.SetInstanceHasJITConfig(ctx, jitInstance.JitConfiguration)

	_, err := s.Runner.GetJITConfigFile(ctx, ".runner")
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *MetadataTestSuite) TestGetJITConfigFileInvalidBase64() {
	jitInstance := s.Fixtures.TestInstance
	jitInstance.JitConfiguration = map[string]string{
		".runner": "invalid-base64!!!",
	}

	ctx := auth.SetInstanceParams(context.Background(), jitInstance)
	ctx = auth.SetInstanceRunnerStatus(ctx, params.RunnerPending)
	ctx = auth.SetInstanceHasJITConfig(ctx, jitInstance.JitConfiguration)

	_, err := s.Runner.GetJITConfigFile(ctx, ".runner")
	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "error decoding file contents")
}

func (s *MetadataTestSuite) TestGetJITConfigFileMultipleFiles() {
	jitInstance := s.Fixtures.TestInstance
	jitInstance.JitConfiguration = map[string]string{
		".runner":      base64.StdEncoding.EncodeToString([]byte("runner config")),
		".credentials": base64.StdEncoding.EncodeToString([]byte("credentials config")),
		".env":         base64.StdEncoding.EncodeToString([]byte("env config")),
	}

	ctx := auth.SetInstanceParams(context.Background(), jitInstance)
	ctx = auth.SetInstanceRunnerStatus(ctx, params.RunnerPending)
	ctx = auth.SetInstanceHasJITConfig(ctx, jitInstance.JitConfiguration)

	// Test each file can be retrieved
	runnerContent, err := s.Runner.GetJITConfigFile(ctx, ".runner")
	s.Require().Nil(err)
	s.Require().Equal("runner config", string(runnerContent))

	credContent, err := s.Runner.GetJITConfigFile(ctx, ".credentials")
	s.Require().Nil(err)
	s.Require().Equal("credentials config", string(credContent))

	envContent, err := s.Runner.GetJITConfigFile(ctx, ".env")
	s.Require().Nil(err)
	s.Require().Equal("env config", string(envContent))
}

func (s *MetadataTestSuite) TestGenerateSystemdUnitFileGiteaWithDefaultUser() {
	entity := s.Fixtures.TestEntity
	entity.Credentials.ForgeType = params.GiteaEndpointType
	ctx := auth.SetInstanceEntity(context.Background(), entity)

	unitFile, err := s.Runner.GenerateSystemdUnitFile(ctx, "")

	s.Require().Nil(err)
	s.Require().NotEmpty(unitFile)
	s.Require().Contains(string(unitFile), "Act Runner")
	s.Require().Contains(string(unitFile), "act_runner daemon --once")
	s.Require().Contains(string(unitFile), "Restart=always")
}

func (s *MetadataTestSuite) TestGetLabelsForInstanceWithCache() {
	// This test would require setting up the cache properly
	// For now, we test that it doesn't panic with empty cache
	instance := s.Fixtures.TestInstance
	labels := getLabelsForInstance(instance)
	s.Require().NotNil(labels)
}

func (s *MetadataTestSuite) TestGetLabelsForInstanceWithScaleSetAndJIT() {
	// Test instance with both scale set and JIT config
	// JIT should take precedence
	instance := s.Fixtures.TestInstance
	instance.ScaleSetID = 123
	instance.JitConfiguration = map[string]string{"test": "config"}

	labels := getLabelsForInstance(instance)
	s.Require().Empty(labels)
}

func (s *MetadataTestSuite) TestGetServiceNameForEntityAllTypes() {
	tests := []struct {
		name       string
		entityType params.ForgeEntityType
		owner      string
		repoName   string
		expected   string
		wantErr    bool
	}{
		{
			name:       "Enterprise",
			entityType: params.ForgeEntityTypeEnterprise,
			owner:      "my-enterprise",
			expected:   "actions.runner.my-enterprise",
			wantErr:    false,
		},
		{
			name:       "Organization",
			entityType: params.ForgeEntityTypeOrganization,
			owner:      "my-org",
			expected:   "actions.runner.my-org",
			wantErr:    false,
		},
		{
			name:       "Repository",
			entityType: params.ForgeEntityTypeRepository,
			owner:      "my-owner",
			repoName:   "my-repo",
			expected:   "actions.runner.my-owner.my-repo",
			wantErr:    false,
		},
		{
			name:       "Invalid type",
			entityType: "invalid-type",
			owner:      "test",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			entity := params.ForgeEntity{
				EntityType: tt.entityType,
				Owner:      tt.owner,
				Name:       tt.repoName,
			}

			serviceName, err := s.Runner.getServiceNameForEntity(entity)

			if tt.wantErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().Equal(tt.expected, serviceName)
			}
		})
	}
}

func (s *MetadataTestSuite) TestFileObjectToGARMToolWithOptionalFields() {
	fileObject := params.FileObject{
		ID:          10,
		Name:        "test-agent",
		Size:        2048,
		SHA256:      "def456",
		Description: "Test description",
		FileType:    "executable",
		Tags: []string{
			"version=2.0.0",
			"os_type=windows",
			"os_arch=arm64",
			"extra_tag=value",
		},
	}

	result, err := fileObjectToGARMTool(fileObject, "http://test.com/dl")

	s.Require().NoError(err)
	s.Require().Equal(uint(10), result.ID)
	s.Require().Equal("test-agent", result.Name)
	s.Require().Equal(int64(2048), result.Size)
	s.Require().Equal("def456", result.SHA256SUM)
	s.Require().Equal("Test description", result.Description)
	s.Require().Equal("executable", result.FileType)
	s.Require().Equal("2.0.0", result.Version)
	s.Require().Equal(commonParams.OSType("windows"), result.OSType)
	s.Require().Equal(commonParams.OSArch("arm64"), result.OSArch)
	s.Require().Equal("http://test.com/dl", result.DownloadURL)
}

func TestMetadataTestSuite(t *testing.T) {
	suite.Run(t, new(MetadataTestSuite))
}
