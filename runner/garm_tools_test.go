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

//go:build testing
// +build testing

package runner

import (
	"bytes"
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
)

var (
	windowsAMD64ToolsTags = []string{
		garmAgentFileTag,
		garmAgentOSTypeWindowsTag,
		garmAgentOSArchAMD64Tag,
	}
	windowsARM64ToolsTags = []string{
		garmAgentFileTag,
		garmAgentOSTypeWindowsTag,
		garmAgentOSArchARM64Tag,
	}
	linuxARM64ToolsTags = []string{
		garmAgentFileTag,
		garmAgentOSTypeLinuxTag,
		garmAgentOSArchARM64Tag,
	}
)

type GARMToolsTestSuite struct {
	suite.Suite
	AdminContext        context.Context
	UnauthorizedContext context.Context
	Store               dbCommon.Store
	Runner              *Runner
}

func (s *GARMToolsTestSuite) SetupTest() {
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(context.Background(), dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	adminCtx := garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())

	s.AdminContext = adminCtx
	s.UnauthorizedContext = context.Background()
	s.Store = db

	runner := &Runner{
		ctx:   adminCtx,
		store: db,
	}
	s.Runner = runner
}

func (s *GARMToolsTestSuite) TestCreateGARMToolUnauthorized() {
	param := params.CreateGARMToolParams{
		Name:        "garm-agent-linux-amd64",
		Description: "GARM agent for Linux AMD64",
		Size:        1024,
		OSType:      "linux",
		OSArch:      "amd64",
		Version:     "v1.0.0",
	}

	reader := bytes.NewReader([]byte("test binary content"))
	_, err := s.Runner.CreateGARMTool(s.UnauthorizedContext, param, reader)

	s.Require().Error(err)
	s.Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *GARMToolsTestSuite) TestCreateGARMToolMissingVersion() {
	param := params.CreateGARMToolParams{
		Name:        "garm-agent-linux-amd64",
		Description: "GARM agent for Linux AMD64",
		Size:        1024,
		OSType:      "linux",
		OSArch:      "amd64",
		Version:     "",
	}

	reader := bytes.NewReader([]byte("test binary content"))
	_, err := s.Runner.CreateGARMTool(s.AdminContext, param, reader)

	s.Require().Error(err)
	s.Contains(err.Error(), "version is required")
}

func (s *GARMToolsTestSuite) TestCreateGARMToolInvalidOSType() {
	param := params.CreateGARMToolParams{
		Name:        "garm-agent-invalid-amd64",
		Description: "Invalid OS type",
		Size:        1024,
		OSType:      "invalid",
		OSArch:      "amd64",
		Version:     "v1.0.0",
	}

	reader := bytes.NewReader([]byte("test binary content"))
	_, err := s.Runner.CreateGARMTool(s.AdminContext, param, reader)

	s.Require().Error(err)
	s.Contains(err.Error(), "invalid os_type")
}

func (s *GARMToolsTestSuite) TestCreateGARMToolInvalidOSArch() {
	param := params.CreateGARMToolParams{
		Name:        "garm-agent-linux-invalid",
		Description: "Invalid arch",
		Size:        1024,
		OSType:      "linux",
		OSArch:      "invalid",
		Version:     "v1.0.0",
	}

	reader := bytes.NewReader([]byte("test binary content"))
	_, err := s.Runner.CreateGARMTool(s.AdminContext, param, reader)

	s.Require().Error(err)
	s.Contains(err.Error(), "invalid os_arch")
}

func (s *GARMToolsTestSuite) TestCreateGARMToolSuccess() {
	content := []byte("test binary content for linux amd64")
	param := params.CreateGARMToolParams{
		Name:        "garm-agent-linux-amd64",
		Description: "GARM agent for Linux AMD64",
		Size:        int64(len(content)),
		OSType:      commonParams.Linux,
		OSArch:      commonParams.Amd64,
		Version:     "v1.0.0",
	}

	reader := bytes.NewReader(content)
	tool, err := s.Runner.CreateGARMTool(s.AdminContext, param, reader)

	s.Require().NoError(err)
	s.Equal(param.Name, tool.Name)
	s.Equal(param.Description, tool.Description)
	s.Equal(int64(len(content)), tool.Size)

	// Verify tags
	expectedTags := []string{
		"category=garm-agent",
		"os_type=linux",
		"os_arch=amd64",
		"version=v1.0.0",
	}
	s.ElementsMatch(expectedTags, tool.Tags)
}

func (s *GARMToolsTestSuite) TestCreateGARMToolCleansUpOldVersions() {
	// Upload version 1.0.0
	param1 := params.CreateGARMToolParams{
		Name:        "garm-agent-linux-arm64",
		Description: "GARM agent for Linux ARM64 v1.0.0",
		Size:        1024,
		OSType:      commonParams.Linux,
		OSArch:      commonParams.Arm64,
		Version:     "v1.0.0",
	}
	reader1 := bytes.NewReader([]byte("version 1.0.0 binary"))
	tool1, err := s.Runner.CreateGARMTool(s.AdminContext, param1, reader1)
	s.Require().NoError(err)

	// Upload version 1.1.0
	param2 := params.CreateGARMToolParams{
		Name:        "garm-agent-linux-arm64",
		Description: "GARM agent for Linux ARM64 v1.1.0",
		Size:        2048,
		OSType:      commonParams.Linux,
		OSArch:      commonParams.Arm64,
		Version:     "v1.1.0",
	}
	reader2 := bytes.NewReader([]byte("version 1.1.0 binary"))
	tool2, err := s.Runner.CreateGARMTool(s.AdminContext, param2, reader2)
	s.Require().NoError(err)

	// Verify v1.0.0 was deleted
	_, err = s.Store.GetFileObject(s.AdminContext, tool1.ID)
	s.Require().Error(err)
	s.Contains(err.Error(), "could not find file object")

	// Verify v1.1.0 still exists
	existing, err := s.Store.GetFileObject(s.AdminContext, tool2.ID)
	s.Require().NoError(err)
	s.Equal(tool2.ID, existing.ID)

	// Search for all linux/arm64 tools - should only find v1.1.0
	results, err := s.Store.SearchFileObjectByTags(s.AdminContext, linuxARM64ToolsTags, 1, 10)
	s.Require().NoError(err)
	s.Equal(uint64(1), results.TotalCount)
	s.Len(results.Results, 1)
	s.Equal(tool2.ID, results.Results[0].ID)
}

func (s *GARMToolsTestSuite) TestCreateGARMToolPaginationCleanup() {
	// Upload 150 old versions for windows/amd64
	for i := 1; i <= 150; i++ {
		param := params.CreateGARMToolParams{
			Name:        fmt.Sprintf("garm-agent-windows-amd64-v%d", i),
			Description: fmt.Sprintf("Version %d", i),
			Size:        int64(i * 100),
			OSType:      commonParams.Windows,
			OSArch:      commonParams.Amd64,
			Version:     fmt.Sprintf("v0.0.%d", i),
		}
		reader := bytes.NewReader([]byte(fmt.Sprintf("version %d binary", i)))
		_, err := s.Runner.CreateGARMTool(s.AdminContext, param, reader)
		s.Require().NoError(err)
	}

	// Upload the latest version
	latestParam := params.CreateGARMToolParams{
		Name:        "garm-agent-windows-amd64",
		Description: "Latest version",
		Size:        99999,
		OSType:      commonParams.Windows,
		OSArch:      commonParams.Amd64,
		Version:     "v2.0.0",
	}
	reader := bytes.NewReader([]byte("latest version binary"))
	latestTool, err := s.Runner.CreateGARMTool(s.AdminContext, latestParam, reader)
	s.Require().NoError(err)

	// Verify only the latest version exists
	results, err := s.Store.SearchFileObjectByTags(s.AdminContext, windowsAMD64ToolsTags, 1, 200)
	s.Require().NoError(err)
	s.Equal(uint64(1), results.TotalCount)
	s.Len(results.Results, 1)
	s.Equal(latestTool.ID, results.Results[0].ID)
	s.Equal("v2.0.0", getVersionFromTags(results.Results[0].Tags))
}

func (s *GARMToolsTestSuite) TestCreateGARMToolDoesNotAffectOtherPlatforms() {
	// Upload linux/amd64 v1.0.0
	param1 := params.CreateGARMToolParams{
		Name:        "garm-agent-linux-amd64",
		Description: "Linux AMD64",
		Size:        1024,
		OSType:      commonParams.Linux,
		OSArch:      commonParams.Amd64,
		Version:     "v1.0.0",
	}
	reader1 := bytes.NewReader([]byte("linux amd64 binary"))
	tool1, err := s.Runner.CreateGARMTool(s.AdminContext, param1, reader1)
	s.Require().NoError(err)

	// Upload windows/amd64 v1.0.0
	param2 := params.CreateGARMToolParams{
		Name:        "garm-agent-windows-amd64",
		Description: "Windows AMD64",
		Size:        2048,
		OSType:      commonParams.Windows,
		OSArch:      commonParams.Amd64,
		Version:     "v1.0.0",
	}
	reader2 := bytes.NewReader([]byte("windows amd64 binary"))
	tool2, err := s.Runner.CreateGARMTool(s.AdminContext, param2, reader2)
	s.Require().NoError(err)

	// Verify both still exist
	_, err = s.Store.GetFileObject(s.AdminContext, tool1.ID)
	s.Require().NoError(err)

	_, err = s.Store.GetFileObject(s.AdminContext, tool2.ID)
	s.Require().NoError(err)
}

func (s *GARMToolsTestSuite) TestDeleteGarmToolUnauthorized() {
	err := s.Runner.DeleteGarmTool(s.UnauthorizedContext, "linux", "amd64")
	s.Require().Error(err)
	s.Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *GARMToolsTestSuite) TestDeleteGarmToolInvalidOSType() {
	err := s.Runner.DeleteGarmTool(s.AdminContext, "invalid", "amd64")
	s.Require().Error(err)
	s.Contains(err.Error(), "invalid os_type")
}

func (s *GARMToolsTestSuite) TestDeleteGarmToolInvalidOSArch() {
	err := s.Runner.DeleteGarmTool(s.AdminContext, "linux", "invalid")
	s.Require().Error(err)
	s.Contains(err.Error(), "invalid os_arch")
}

func (s *GARMToolsTestSuite) TestDeleteGarmToolNotFound() {
	err := s.Runner.DeleteGarmTool(s.AdminContext, "linux", "amd64")
	s.Require().Error(err)
	s.Contains(err.Error(), "no garm-agent tools found")
}

func (s *GARMToolsTestSuite) TestDeleteGarmToolSuccess() {
	// Create a tool
	param := params.CreateGARMToolParams{
		Name:        "garm-agent-linux-amd64",
		Description: "GARM agent for Linux AMD64",
		Size:        1024,
		OSType:      commonParams.Linux,
		OSArch:      commonParams.Amd64,
		Version:     "v1.0.0",
	}
	reader := bytes.NewReader([]byte("test binary"))
	tool, err := s.Runner.CreateGARMTool(s.AdminContext, param, reader)
	s.Require().NoError(err)

	// Delete it
	err = s.Runner.DeleteGarmTool(s.AdminContext, "linux", "amd64")
	s.Require().NoError(err)

	// Verify it's gone
	_, err = s.Store.GetFileObject(s.AdminContext, tool.ID)
	s.Require().Error(err)
	s.Contains(err.Error(), "could not find file object")
}

func (s *GARMToolsTestSuite) TestDeleteGarmToolDeletesAllVersions() {
	// Create multiple versions using windows/arm64
	for i := 1; i <= 5; i++ {
		reader := bytes.NewReader([]byte(fmt.Sprintf("version %d", i)))
		// CreateGARMTool only keeps the latest, so we need to use the store directly
		// to create multiple versions
		tags := windowsARM64ToolsTags
		tags = append(tags, fmt.Sprintf("version=v1.%d.0", i))
		createParam := params.CreateFileObjectParams{
			Name:        fmt.Sprintf("garm-agent-windows-arm64-v%d", i),
			Description: fmt.Sprintf("Version %d", i),
			Size:        int64(i * 100),
			Tags:        tags,
		}
		_, err := s.Store.CreateFileObject(s.AdminContext, createParam, reader)
		s.Require().NoError(err)
	}

	// Verify we have 5 versions
	results, err := s.Store.SearchFileObjectByTags(s.AdminContext, windowsARM64ToolsTags, 1, 10)
	s.Require().NoError(err)
	s.Equal(uint64(5), results.TotalCount)

	// Delete all
	err = s.Runner.DeleteGarmTool(s.AdminContext, "windows", "arm64")
	s.Require().NoError(err)

	// Verify all are gone
	results, err = s.Store.SearchFileObjectByTags(s.AdminContext, windowsARM64ToolsTags, 1, 10)
	s.Require().NoError(err)
	s.Equal(uint64(0), results.TotalCount)
}

func (s *GARMToolsTestSuite) TestListGARMToolsUnauthorized() {
	_, err := s.Runner.ListGARMTools(s.UnauthorizedContext)
	s.Require().Error(err)
	s.Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *GARMToolsTestSuite) TestListGARMToolsEmpty() {
	tools, err := s.Runner.ListGARMTools(s.AdminContext)
	s.Require().NoError(err)
	s.Empty(tools)
}

func (s *GARMToolsTestSuite) TestListGARMToolsSinglePlatform() {
	// Create one tool
	param := params.CreateGARMToolParams{
		Name:        "garm-agent-linux-amd64",
		Description: "GARM agent for Linux AMD64",
		Size:        1024,
		OSType:      commonParams.Linux,
		OSArch:      commonParams.Amd64,
		Version:     "v1.0.0",
	}
	reader := bytes.NewReader([]byte("test binary"))
	_, err := s.Runner.CreateGARMTool(s.AdminContext, param, reader)
	s.Require().NoError(err)

	tools, err := s.Runner.ListGARMTools(s.AdminContext)
	s.Require().NoError(err)
	s.Len(tools, 1)
	s.Equal("linux", string(tools[0].OSType))
	s.Equal("amd64", string(tools[0].OSArch))
}

func (s *GARMToolsTestSuite) TestListGARMToolsMultiplePlatforms() {
	// Create tools for all supported platforms
	platforms := []struct {
		osType commonParams.OSType
		osArch commonParams.OSArch
	}{
		{commonParams.Linux, commonParams.Amd64},
		{commonParams.Linux, commonParams.Arm64},
		{commonParams.Windows, commonParams.Amd64},
		{commonParams.Windows, commonParams.Arm64},
	}

	for _, p := range platforms {
		param := params.CreateGARMToolParams{
			Name:        fmt.Sprintf("garm-agent-%s-%s", p.osType, p.osArch),
			Description: fmt.Sprintf("GARM agent for %s %s", p.osType, p.osArch),
			Size:        1024,
			OSType:      p.osType,
			OSArch:      p.osArch,
			Version:     "v1.0.0",
		}
		reader := bytes.NewReader([]byte(fmt.Sprintf("%s %s binary", p.osType, p.osArch)))
		_, err := s.Runner.CreateGARMTool(s.AdminContext, param, reader)
		s.Require().NoError(err)
	}

	tools, err := s.Runner.ListGARMTools(s.AdminContext)
	s.Require().NoError(err)
	s.Len(tools, 4)

	// Verify we have all platforms
	foundPlatforms := make(map[string]bool)
	for _, tool := range tools {
		key := fmt.Sprintf("%s-%s", tool.OSType, tool.OSArch)
		foundPlatforms[key] = true
	}
	s.True(foundPlatforms["linux-amd64"])
	s.True(foundPlatforms["linux-arm64"])
	s.True(foundPlatforms["windows-amd64"])
	s.True(foundPlatforms["windows-arm64"])
}

// Helper function to extract version from tags
func getVersionFromTags(tags []string) string {
	for _, tag := range tags {
		if len(tag) > 8 && tag[:8] == "version=" {
			return tag[8:]
		}
	}
	return ""
}

func TestGARMToolsTestSuite(t *testing.T) {
	suite.Run(t, new(GARMToolsTestSuite))
}
