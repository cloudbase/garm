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

const (
	testTemplate1Name = "test-template-1"
)

type TemplateTestFixtures struct {
	AdminContext         context.Context
	Store                dbCommon.Store
	Templates            []params.Template
	Providers            map[string]common.Provider
	ProviderMock         *runnerCommonMocks.Provider
	PoolMgrCtrlMock      *runnerMocks.PoolManagerController
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
		Name:        testTemplate1Name,
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
		if template.Name == testTemplate1Name {
			found = true
			break
		}
	}
	s.Require().True(found, "Expected %s not found", testTemplate1Name)
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
	partialName := testTemplate1Name
	templates, err := s.Runner.ListTemplates(s.adminCtx, nil, nil, &partialName)

	s.Require().Nil(err)
	s.Require().GreaterOrEqual(len(templates), 1)

	found := false
	for _, template := range templates {
		if template.Name == testTemplate1Name {
			found = true
			break
		}
	}
	s.Require().True(found, "Expected %s not found", testTemplate1Name)
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

func (s *TemplateTestSuite) TestRestoreTemplateSpecific() {
	osType := commonParams.Linux
	forgeType := params.GithubEndpointType
	templates, err := s.Runner.ListTemplates(s.adminCtx, &osType, &forgeType, nil)
	s.Require().Nil(err)
	s.Require().GreaterOrEqual(len(templates), 1, "Expected at least one github_linux template from migration")

	var systemTemplate *params.Template
	for _, tpl := range templates {
		if tpl.Owner == params.SystemUser {
			systemTemplate = &tpl
			break
		}
	}
	s.Require().NotNil(systemTemplate, "Expected system template for github_linux")

	modifiedName := "modified_template_name"
	modifiedData := []byte("modified template content for testing")
	updateParams := params.UpdateTemplateParams{
		Name: &modifiedName,
		Data: modifiedData,
	}
	_, err = s.Runner.UpdateTemplate(s.adminCtx, systemTemplate.ID, updateParams)
	s.Require().Nil(err)

	updatedTemplate, err := s.Runner.GetTemplate(s.adminCtx, systemTemplate.ID)
	s.Require().Nil(err)
	s.Require().Equal(modifiedName, updatedTemplate.Name)
	s.Require().Equal(modifiedData, updatedTemplate.Data)

	restoreParams := params.RestoreTemplateRequest{
		Forge:      params.GithubEndpointType,
		OSType:     commonParams.Linux,
		RestoreAll: false,
	}
	err = s.Runner.RestoreTemplate(s.adminCtx, restoreParams)
	s.Require().Nil(err)

	// Verify the template was restored
	restoredTemplate, err := s.Runner.GetTemplate(s.adminCtx, systemTemplate.ID)
	s.Require().Nil(err)
	// Name should be restored to the system default
	s.Require().Equal("github_linux", restoredTemplate.Name)
	// Data should be restored to original template content (not the modified content)
	s.Require().NotEqual(modifiedData, restoredTemplate.Data)
	// Should match the original template data or be close to it (content from internal/templates)
	s.Require().NotEmpty(restoredTemplate.Data)
	// Verify it's still a system template
	s.Require().Equal(params.SystemUser, restoredTemplate.Owner)

	// Verify the data is different from what we modified (restored back to system template)
	s.Require().NotEqual(string(modifiedData), string(restoredTemplate.Data))
}

func (s *TemplateTestSuite) TestRestoreTemplateAll() {
	// Get all system templates
	allTemplates, err := s.Runner.ListTemplates(s.adminCtx, nil, nil, nil)
	s.Require().Nil(err)

	// Find all system templates
	systemTemplates := []params.Template{}
	for _, tpl := range allTemplates {
		if tpl.Owner == params.SystemUser {
			systemTemplates = append(systemTemplates, tpl)
		}
	}
	// We should have at least 4 system templates (github/gitea x linux/windows)
	s.Require().GreaterOrEqual(len(systemTemplates), 4, "Expected at least 4 system templates")

	// Modify all system templates
	modifiedTemplateIDs := make(map[uint]struct {
		originalName string
		originalData []byte
	})

	for _, tpl := range systemTemplates {
		modifiedName := "modified_" + tpl.Name
		modifiedData := []byte("modified content for " + tpl.Name)

		updateParams := params.UpdateTemplateParams{
			Name: &modifiedName,
			Data: modifiedData,
		}
		_, err := s.Runner.UpdateTemplate(s.adminCtx, tpl.ID, updateParams)
		s.Require().Nil(err)

		modifiedTemplateIDs[tpl.ID] = struct {
			originalName string
			originalData []byte
		}{
			originalName: tpl.Name,
			originalData: tpl.Data,
		}
	}

	// Verify all templates were modified
	for templateID := range modifiedTemplateIDs {
		template, err := s.Runner.GetTemplate(s.adminCtx, templateID)
		s.Require().Nil(err)
		s.Require().Contains(template.Name, "modified_", "Template name should contain 'modified_'")
	}

	// Restore all templates
	restoreParams := params.RestoreTemplateRequest{
		RestoreAll: true,
	}
	err = s.Runner.RestoreTemplate(s.adminCtx, restoreParams)
	s.Require().Nil(err)

	// Verify all templates were restored
	for templateID := range modifiedTemplateIDs {
		template, err := s.Runner.GetTemplate(s.adminCtx, templateID)
		s.Require().Nil(err)

		// Name should not contain "modified_" anymore
		s.Require().NotContains(template.Name, "modified_", "Template name should be restored")
		// Should still be a system template
		s.Require().Equal(params.SystemUser, template.Owner)
		// Data should be restored from internal/templates
		s.Require().NotEmpty(template.Data)
		s.Require().NotContains(string(template.Data), "modified content", "Template data should be restored")
	}

	// Verify we can still find templates for each OS/Forge combination
	combinations := []struct {
		os    commonParams.OSType
		forge params.EndpointType
	}{
		{commonParams.Linux, params.GithubEndpointType},
		{commonParams.Windows, params.GithubEndpointType},
		{commonParams.Linux, params.GiteaEndpointType},
		{commonParams.Windows, params.GiteaEndpointType},
	}

	for _, combo := range combinations {
		templates, err := s.Runner.ListTemplates(s.adminCtx, &combo.os, &combo.forge, nil)
		s.Require().Nil(err)

		foundSystem := false
		for _, tpl := range templates {
			if tpl.Owner == params.SystemUser {
				foundSystem = true
				break
			}
		}
		s.Require().True(foundSystem, "Should have system template for %s/%s", combo.forge, combo.os)
	}
}

func (s *TemplateTestSuite) TestRestoreTemplateMissingTemplate() {
	// Delete a system template
	osType := commonParams.Windows
	forgeType := params.GiteaEndpointType
	templates, err := s.Runner.ListTemplates(s.adminCtx, &osType, &forgeType, nil)
	s.Require().Nil(err)

	var systemTemplate *params.Template
	for _, tpl := range templates {
		if tpl.Owner == params.SystemUser {
			systemTemplate = &tpl
			break
		}
	}
	s.Require().NotNil(systemTemplate, "Expected system template for gitea_windows")

	// Delete the template
	err = s.Runner.DeleteTemplate(s.adminCtx, systemTemplate.ID)
	s.Require().Nil(err)

	// Verify it's deleted
	templates, err = s.Runner.ListTemplates(s.adminCtx, &osType, &forgeType, nil)
	s.Require().Nil(err)

	foundSystem := false
	for _, tpl := range templates {
		if tpl.Owner == params.SystemUser {
			foundSystem = true
			break
		}
	}
	s.Require().False(foundSystem, "System template should be deleted")

	restoreParams := params.RestoreTemplateRequest{
		Forge:      params.GiteaEndpointType,
		OSType:     commonParams.Windows,
		RestoreAll: false,
	}
	err = s.Runner.RestoreTemplate(s.adminCtx, restoreParams)
	s.Require().Nil(err)

	templates, err = s.Runner.ListTemplates(s.adminCtx, &osType, &forgeType, nil)
	s.Require().Nil(err)

	foundSystem = false
	var recreatedTemplateID uint
	for _, tpl := range templates {
		if tpl.Owner == params.SystemUser {
			foundSystem = true
			recreatedTemplateID = tpl.ID
			break
		}
	}
	s.Require().True(foundSystem, "System template should be recreated")
	s.Require().NotZero(recreatedTemplateID)

	// Get the full template with data
	recreatedTemplate, err := s.Runner.GetTemplate(s.adminCtx, recreatedTemplateID)
	s.Require().Nil(err)
	s.Require().Equal("gitea_windows", recreatedTemplate.Name)
	s.Require().NotEmpty(recreatedTemplate.Data)
}

func (s *TemplateTestSuite) TestRestoreTemplateUnauthorized() {
	restoreParams := params.RestoreTemplateRequest{
		Forge:      params.GithubEndpointType,
		OSType:     commonParams.Linux,
		RestoreAll: false,
	}

	err := s.Runner.RestoreTemplate(s.nonAdminCtx, restoreParams)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *TemplateTestSuite) TestRestoreTemplatePreservesUserTemplates() {
	// Create a user template with the same OS/Forge as a system template
	userTemplate, err := s.Runner.CreateTemplate(s.adminCtx, params.CreateTemplateParams{
		Name:        "user-github-linux-template",
		Description: "User's custom template",
		OSType:      commonParams.Linux,
		ForgeType:   params.GithubEndpointType,
		Data:        []byte("user custom template data"),
	})
	s.Require().Nil(err)
	s.Require().NotEqual(params.SystemUser, userTemplate.Owner, "User template should not be system owned")

	// Restore all templates
	restoreParams := params.RestoreTemplateRequest{
		RestoreAll: true,
	}
	err = s.Runner.RestoreTemplate(s.adminCtx, restoreParams)
	s.Require().Nil(err)

	// Verify user template still exists and wasn't modified
	userTemplateAfter, err := s.Runner.GetTemplate(s.adminCtx, userTemplate.ID)
	s.Require().Nil(err)
	s.Require().Equal(userTemplate.Name, userTemplateAfter.Name)
	s.Require().Equal(userTemplate.Data, userTemplateAfter.Data)
	s.Require().NotEqual(params.SystemUser, userTemplateAfter.Owner)
}

func TestTemplateTestSuite(t *testing.T) {
	suite.Run(t, new(TemplateTestSuite))
}
