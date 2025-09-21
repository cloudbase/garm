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
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	runnerCommonMocks "github.com/cloudbase/garm/runner/common/mocks"
	runnerMocks "github.com/cloudbase/garm/runner/mocks"
)

type TemplateTestFixtures struct {
	AdminContext    context.Context
	Store          dbCommon.Store
	Templates      []params.Template
	Providers      map[string]common.Provider
	ProviderMock   *runnerCommonMocks.Provider
	PoolMgrCtrlMock *runnerMocks.PoolManagerController
	CreateTemplateParams params.CreateTemplateParams
	UpdateTemplateParams params.UpdateTemplateParams
}

type TemplateTestSuite struct {
	suite.Suite
	Fixtures *TemplateTestFixtures
	Runner   *Runner

	adminCtx       context.Context
	nonAdminCtx    context.Context
	githubEndpoint params.ForgeEndpoint
}

func (s *TemplateTestSuite) SetupTest() {
	// create testing sqlite database
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(context.Background(), dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	s.adminCtx = garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())
	s.nonAdminCtx = context.Background() // Non-admin context for unauthorized tests

	s.githubEndpoint = garmTesting.CreateDefaultGithubEndpoint(s.adminCtx, db, s.T())

	// Create test templates
	template1, err := db.CreateTemplate(s.adminCtx, params.CreateTemplateParams{
		Name:        "test-template-1",
		Description: "Test template 1",
		OSType:      commonParams.Linux,
		ForgeType:   params.GithubEndpointType,
		Data:        []byte(`{"provider": "lxd", "image": "ubuntu:22.04"}`),
	})
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test template 1: %s", err))
	}

	template2, err := db.CreateTemplate(s.adminCtx, params.CreateTemplateParams{
		Name:        "test-template-2",
		Description: "Test template 2",
		OSType:      commonParams.Windows,
		ForgeType:   params.GithubEndpointType,
		Data:        []byte(`{"provider": "azure", "image": "windows-2022"}`),
	})
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test template 2: %s", err))
	}

	templates := []params.Template{template1, template2}

	providerMock := runnerCommonMocks.NewProvider(s.T())
	fixtures := &TemplateTestFixtures{
		AdminContext: s.adminCtx,
		Store:        db,
		Templates:    templates,
		Providers: map[string]common.Provider{
			"test-provider": providerMock,
		},
		ProviderMock:    providerMock,
		PoolMgrCtrlMock: runnerMocks.NewPoolManagerController(s.T()),
		CreateTemplateParams: params.CreateTemplateParams{
			Name:        "new-template",
			Description: "New test template",
			OSType:      commonParams.Linux,
			ForgeType:   params.GithubEndpointType,
			Data:        []byte(`{"provider": "lxd", "image": "ubuntu:20.04"}`),
		},
		UpdateTemplateParams: params.UpdateTemplateParams{
			Name:        garmTesting.Ptr("updated-template-name"),
			Description: garmTesting.Ptr("Updated description"),
			Data:        []byte(`{"provider": "updated", "image": "updated:latest"}`),
		},
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
}

func (s *TemplateTestSuite) TestCreateTemplate() {
	template, err := s.Runner.CreateTemplate(s.adminCtx, s.Fixtures.CreateTemplateParams)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.CreateTemplateParams.Name, template.Name)
	s.Require().Equal(s.Fixtures.CreateTemplateParams.Description, template.Description)
	s.Require().Equal(s.Fixtures.CreateTemplateParams.OSType, template.OSType)
	s.Require().Equal(s.Fixtures.CreateTemplateParams.ForgeType, template.ForgeType)
}

func (s *TemplateTestSuite) TestCreateTemplateUnauthorized() {
	_, err := s.Runner.CreateTemplate(s.nonAdminCtx, s.Fixtures.CreateTemplateParams)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *TemplateTestSuite) TestCreateTemplateInvalidParams() {
	invalidParams := params.CreateTemplateParams{
		Name:        "", // Empty name should fail validation
		Description: "Invalid template",
		OSType:      commonParams.Linux,
		ForgeType:   params.GithubEndpointType,
		Data:        []byte(`{"provider": "lxd"}`),
	}

	_, err := s.Runner.CreateTemplate(s.adminCtx, invalidParams)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "invalid create params")
}

func (s *TemplateTestSuite) TestGetTemplate() {
	template, err := s.Runner.GetTemplate(s.adminCtx, s.Fixtures.Templates[0].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Templates[0].ID, template.ID)
	s.Require().Equal(s.Fixtures.Templates[0].Name, template.Name)
	s.Require().Equal(s.Fixtures.Templates[0].Description, template.Description)
}

func (s *TemplateTestSuite) TestGetTemplateUnauthorized() {
	_, err := s.Runner.GetTemplate(s.nonAdminCtx, s.Fixtures.Templates[0].ID)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *TemplateTestSuite) TestGetTemplateNotFound() {
	_, err := s.Runner.GetTemplate(s.adminCtx, 9999)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "failed to get template")
}

func (s *TemplateTestSuite) TestListTemplates() {
	templates, err := s.Runner.ListTemplates(s.adminCtx, nil, nil, nil)

	s.Require().Nil(err)
	s.Require().GreaterOrEqual(len(templates), len(s.Fixtures.Templates))
	
	// Find our test templates in the results
	foundNames := make(map[string]bool)
	for _, template := range templates {
		foundNames[template.Name] = true
	}
	
	for _, expected := range s.Fixtures.Templates {
		s.Require().True(foundNames[expected.Name], "Expected template %s not found", expected.Name)
	}
}

func (s *TemplateTestSuite) TestListTemplatesUnauthorized() {
	_, err := s.Runner.ListTemplates(s.nonAdminCtx, nil, nil, nil)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *TemplateTestSuite) TestListTemplatesWithOSTypeFilter() {
	osType := commonParams.Linux
	templates, err := s.Runner.ListTemplates(s.adminCtx, &osType, nil, nil)

	s.Require().Nil(err)
	s.Require().GreaterOrEqual(len(templates), 1)
	
	// Verify all returned templates have the correct OS type
	for _, template := range templates {
		s.Require().Equal(commonParams.Linux, template.OSType)
	}
	
	// Find our test template
	found := false
	for _, template := range templates {
		if template.Name == "test-template-1" {
			found = true
			break
		}
	}
	s.Require().True(found, "Expected test-template-1 not found")
}

func (s *TemplateTestSuite) TestListTemplatesWithForgeTypeFilter() {
	forgeType := params.GithubEndpointType
	templates, err := s.Runner.ListTemplates(s.adminCtx, nil, &forgeType, nil)

	s.Require().Nil(err)
	s.Require().GreaterOrEqual(len(templates), 2)
	
	// Verify all returned templates have the correct forge type
	for _, template := range templates {
		s.Require().Equal(params.GithubEndpointType, template.ForgeType)
	}
}

func (s *TemplateTestSuite) TestListTemplatesWithNameFilter() {
	partialName := "test-template-1"
	templates, err := s.Runner.ListTemplates(s.adminCtx, nil, nil, &partialName)

	s.Require().Nil(err)
	s.Require().GreaterOrEqual(len(templates), 1)
	
	found := false
	for _, template := range templates {
		if template.Name == "test-template-1" {
			found = true
			break
		}
	}
	s.Require().True(found, "Expected test-template-1 not found")
}

func (s *TemplateTestSuite) TestUpdateTemplate() {
	template, err := s.Runner.UpdateTemplate(s.adminCtx, s.Fixtures.Templates[0].ID, s.Fixtures.UpdateTemplateParams)

	s.Require().Nil(err)
	s.Require().Equal(*s.Fixtures.UpdateTemplateParams.Name, template.Name)
	s.Require().Equal(*s.Fixtures.UpdateTemplateParams.Description, template.Description)
}

func (s *TemplateTestSuite) TestUpdateTemplateUnauthorized() {
	_, err := s.Runner.UpdateTemplate(s.nonAdminCtx, s.Fixtures.Templates[0].ID, s.Fixtures.UpdateTemplateParams)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *TemplateTestSuite) TestUpdateTemplateInvalidParams() {
	invalidParams := params.UpdateTemplateParams{
		Name: garmTesting.Ptr(""), // Empty name should fail validation
	}

	_, err := s.Runner.UpdateTemplate(s.adminCtx, s.Fixtures.Templates[0].ID, invalidParams)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "invalid update params")
}

func (s *TemplateTestSuite) TestUpdateTemplateNotFound() {
	_, err := s.Runner.UpdateTemplate(s.adminCtx, 9999, s.Fixtures.UpdateTemplateParams)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "failed to update template")
}

func (s *TemplateTestSuite) TestDeleteTemplate() {
	err := s.Runner.DeleteTemplate(s.adminCtx, s.Fixtures.Templates[1].ID)

	s.Require().Nil(err)

	// Verify template is deleted by trying to get it
	_, err = s.Runner.GetTemplate(s.adminCtx, s.Fixtures.Templates[1].ID)
	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "failed to get template")
}

func (s *TemplateTestSuite) TestDeleteTemplateUnauthorized() {
	err := s.Runner.DeleteTemplate(s.nonAdminCtx, s.Fixtures.Templates[0].ID)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *TemplateTestSuite) TestDeleteTemplateNotFound() {
	// The DeleteTemplate function silently handles ErrNotFound, so this should not error
	err := s.Runner.DeleteTemplate(s.adminCtx, 9999)

	s.Require().Nil(err) // Should not error for not found templates
}

func TestTemplateTestSuite(t *testing.T) {
	suite.Run(t, new(TemplateTestSuite))
}