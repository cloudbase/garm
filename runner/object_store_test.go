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
	"io"
	"testing"

	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type ObjectStoreTestFixtures struct {
	AdminContext        context.Context
	UnauthorizedContext context.Context
	Store               dbCommon.Store
	CreateObjectParams  params.CreateFileObjectParams
	UpdateObjectParams  params.UpdateFileObjectParams
	TestFileObject      params.FileObject
	TestFileContent     []byte
}

type ObjectStoreTestSuite struct {
	suite.Suite
	Fixtures *ObjectStoreTestFixtures
	Runner   *Runner
}

func (s *ObjectStoreTestSuite) SetupTest() {
	// create testing sqlite database
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(context.Background(), dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	adminCtx := garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())

	// Create a test file object
	testContent := []byte("test file content for object store")
	param := params.CreateFileObjectParams{
		Name: "test-file.bin",
		Size: int64(len(testContent)),
		Tags: []string{"test", "binary"},
	}
	fileObj, err := db.CreateFileObject(adminCtx, param, bytes.NewReader(testContent))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test file object: %s", err))
	}

	updatedName := "updated-file.txt"
	// Setup fixtures
	fixtures := &ObjectStoreTestFixtures{
		AdminContext:        adminCtx,
		UnauthorizedContext: context.Background(),
		Store:               db,
		CreateObjectParams: params.CreateFileObjectParams{
			Name: "new-file.txt",
			Size: 100,
			Tags: []string{"new", "test"},
		},
		UpdateObjectParams: params.UpdateFileObjectParams{
			Name: &updatedName,
			Tags: []string{"updated", "test"},
		},
		TestFileObject:  fileObj,
		TestFileContent: testContent,
	}
	s.Fixtures = fixtures

	// Setup test runner
	runner := &Runner{
		ctx:   fixtures.AdminContext,
		store: fixtures.Store,
	}
	s.Runner = runner
}

func (s *ObjectStoreTestSuite) TestCreateFileObject() {
	content := []byte("new file content")
	reader := bytes.NewReader(content)

	createParams := params.CreateFileObjectParams{
		Name: "create-test.txt",
		Size: int64(len(content)),
		Tags: []string{"create", "test"},
	}

	fileObj, err := s.Runner.CreateFileObject(s.Fixtures.AdminContext, createParams, reader)

	s.Require().Nil(err)
	s.Require().NotEmpty(fileObj.ID)
	s.Require().Equal(createParams.Name, fileObj.Name)
	s.Require().Equal(createParams.Size, fileObj.Size)
	s.Require().ElementsMatch(createParams.Tags, fileObj.Tags)
	s.Require().NotEmpty(fileObj.SHA256)
}

func (s *ObjectStoreTestSuite) TestCreateFileObjectUnauthorized() {
	content := []byte("unauthorized content")
	reader := bytes.NewReader(content)

	_, err := s.Runner.CreateFileObject(s.Fixtures.UnauthorizedContext, s.Fixtures.CreateObjectParams, reader)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *ObjectStoreTestSuite) TestCreateFileObjectWithGarmAgentTag() {
	content := []byte("garm-agent tool content")
	reader := bytes.NewReader(content)

	createParams := params.CreateFileObjectParams{
		Name: "garm-agent-tool.bin",
		Size: int64(len(content)),
		Tags: []string{garmAgentFileTag, "test"},
	}

	_, err := s.Runner.CreateFileObject(s.Fixtures.AdminContext, createParams, reader)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "cannot create garm-agent tools via object storage API")
}

func (s *ObjectStoreTestSuite) TestGetFileObject() {
	fileObj, err := s.Runner.GetFileObject(s.Fixtures.AdminContext, s.Fixtures.TestFileObject.ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.TestFileObject.ID, fileObj.ID)
	s.Require().Equal(s.Fixtures.TestFileObject.Name, fileObj.Name)
	s.Require().Equal(s.Fixtures.TestFileObject.Size, fileObj.Size)
	s.Require().ElementsMatch(s.Fixtures.TestFileObject.Tags, fileObj.Tags)
}

func (s *ObjectStoreTestSuite) TestGetFileObjectUnauthorized() {
	_, err := s.Runner.GetFileObject(s.Fixtures.UnauthorizedContext, s.Fixtures.TestFileObject.ID)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *ObjectStoreTestSuite) TestGetFileObjectNotFound() {
	_, err := s.Runner.GetFileObject(s.Fixtures.AdminContext, 99999)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "failed to get file object")
}

func (s *ObjectStoreTestSuite) TestDeleteFileObject() {
	// Create a file to delete
	content := []byte("file to delete")
	param := params.CreateFileObjectParams{
		Name: "delete-test.txt",
		Size: int64(len(content)),
		Tags: []string{"delete"},
	}
	fileObj, err := s.Fixtures.Store.CreateFileObject(s.Fixtures.AdminContext, param, bytes.NewReader(content))
	s.Require().Nil(err)

	err = s.Runner.DeleteFileObject(s.Fixtures.AdminContext, fileObj.ID)

	s.Require().Nil(err)

	// Verify it's deleted
	_, err = s.Fixtures.Store.GetFileObject(s.Fixtures.AdminContext, fileObj.ID)
	s.Require().NotNil(err)
}

func (s *ObjectStoreTestSuite) TestDeleteFileObjectUnauthorized() {
	err := s.Runner.DeleteFileObject(s.Fixtures.UnauthorizedContext, s.Fixtures.TestFileObject.ID)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *ObjectStoreTestSuite) TestDeleteFileObjectNotFound() {
	// Delete of non-existent object is a noop and returns nil (idempotent)
	err := s.Runner.DeleteFileObject(s.Fixtures.AdminContext, 99999)

	s.Require().Nil(err)
}

func (s *ObjectStoreTestSuite) TestDeleteFileObjectWithGarmAgentTag() {
	// Create a file with garm-agent tag
	content := []byte("garm-agent tool")
	param := params.CreateFileObjectParams{
		Name: "garm-agent-delete-test.bin",
		Size: int64(len(content)),
		Tags: []string{garmAgentFileTag},
	}
	// Create directly via store to bypass the API restriction
	fileObj, err := s.Fixtures.Store.CreateFileObject(s.Fixtures.AdminContext, param, bytes.NewReader(content))
	s.Require().Nil(err)

	// Try to delete via API
	err = s.Runner.DeleteFileObject(s.Fixtures.AdminContext, fileObj.ID)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "cannot delete garm-agent tools via object storage API")

	// Verify file still exists
	_, err = s.Fixtures.Store.GetFileObject(s.Fixtures.AdminContext, fileObj.ID)
	s.Require().Nil(err)
}

func (s *ObjectStoreTestSuite) TestListFileObjects() {
	// Create additional test files
	for i := 1; i <= 3; i++ {
		content := []byte(fmt.Sprintf("test file %d", i))
		param := params.CreateFileObjectParams{
			Name: fmt.Sprintf("list-test-%d.txt", i),
			Size: int64(len(content)),
			Tags: []string{"list", "test"},
		}
		_, err := s.Fixtures.Store.CreateFileObject(
			s.Fixtures.AdminContext,
			param,
			bytes.NewReader(content),
		)
		s.Require().Nil(err)
	}

	resp, err := s.Runner.ListFileObjects(s.Fixtures.AdminContext, 0, 25, nil)

	s.Require().Nil(err)
	s.Require().NotNil(resp.Results)
	s.Require().GreaterOrEqual(len(resp.Results), 4) // At least the test file + 3 new ones
}

func (s *ObjectStoreTestSuite) TestListFileObjectsUnauthorized() {
	_, err := s.Runner.ListFileObjects(s.Fixtures.UnauthorizedContext, 0, 25, nil)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *ObjectStoreTestSuite) TestListFileObjectsWithTags() {
	// Create files with specific tags
	specificTag := "specific-list-tag"
	for i := 1; i <= 2; i++ {
		content := []byte(fmt.Sprintf("tagged file %d", i))
		param := params.CreateFileObjectParams{
			Name: fmt.Sprintf("tagged-list-%d.txt", i),
			Size: int64(len(content)),
			Tags: []string{specificTag, "test"},
		}
		_, err := s.Fixtures.Store.CreateFileObject(
			s.Fixtures.AdminContext,
			param,
			bytes.NewReader(content),
		)
		s.Require().Nil(err)
	}

	resp, err := s.Runner.ListFileObjects(s.Fixtures.AdminContext, 0, 25, []string{specificTag})

	s.Require().Nil(err)
	s.Require().NotNil(resp.Results)
	s.Require().GreaterOrEqual(len(resp.Results), 2)
	// Verify all results have the specific tag
	for _, obj := range resp.Results {
		s.Require().Contains(obj.Tags, specificTag)
	}
}

func (s *ObjectStoreTestSuite) TestListFileObjectsPagination() {
	// Create multiple files for pagination test
	for i := 1; i <= 10; i++ {
		content := []byte(fmt.Sprintf("pagination file %d", i))
		param := params.CreateFileObjectParams{
			Name: fmt.Sprintf("page-test-%d.txt", i),
			Size: int64(len(content)),
			Tags: []string{"pagination"},
		}
		_, err := s.Fixtures.Store.CreateFileObject(
			s.Fixtures.AdminContext,
			param,
			bytes.NewReader(content),
		)
		s.Require().Nil(err)
	}

	// Get first page
	resp1, err := s.Runner.ListFileObjects(s.Fixtures.AdminContext, 1, 5, []string{"pagination"})
	s.Require().Nil(err)
	s.Require().Len(resp1.Results, 5)

	// Get second page
	resp2, err := s.Runner.ListFileObjects(s.Fixtures.AdminContext, 2, 5, []string{"pagination"})
	s.Require().Nil(err)
	s.Require().Len(resp2.Results, 5)

	// Verify different results on different pages
	s.Require().NotEqual(resp1.Results[0].ID, resp2.Results[0].ID)
}

func (s *ObjectStoreTestSuite) TestUpdateFileObject() {
	// Create a file to update
	content := []byte("original content")
	param := params.CreateFileObjectParams{
		Name: "update-test.txt",
		Size: int64(len(content)),
		Tags: []string{"original"},
	}
	fileObj, err := s.Fixtures.Store.CreateFileObject(
		s.Fixtures.AdminContext,
		param,
		bytes.NewReader(content),
	)
	s.Require().Nil(err)

	newName := "updated-name.txt"
	updateParams := params.UpdateFileObjectParams{
		Name: &newName,
		Tags: []string{"updated", "modified"},
	}

	updatedObj, err := s.Runner.UpdateFileObject(s.Fixtures.AdminContext, fileObj.ID, updateParams)

	s.Require().Nil(err)
	s.Require().Equal(*updateParams.Name, updatedObj.Name)
	s.Require().ElementsMatch(updateParams.Tags, updatedObj.Tags)
	s.Require().Equal(fileObj.ID, updatedObj.ID)
}

func (s *ObjectStoreTestSuite) TestUpdateFileObjectUnauthorized() {
	_, err := s.Runner.UpdateFileObject(s.Fixtures.UnauthorizedContext, s.Fixtures.TestFileObject.ID, s.Fixtures.UpdateObjectParams)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *ObjectStoreTestSuite) TestUpdateFileObjectNotFound() {
	_, err := s.Runner.UpdateFileObject(s.Fixtures.AdminContext, 99999, s.Fixtures.UpdateObjectParams)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "failed to query object in DB")
}

func (s *ObjectStoreTestSuite) TestUpdateFileObjectWithGarmAgentTag() {
	// Create a file with garm-agent tag
	content := []byte("garm-agent tool")
	param := params.CreateFileObjectParams{
		Name: "garm-agent-update-test.bin",
		Size: int64(len(content)),
		Tags: []string{garmAgentFileTag},
	}
	// Create directly via store to bypass the API restriction
	fileObj, err := s.Fixtures.Store.CreateFileObject(s.Fixtures.AdminContext, param, bytes.NewReader(content))
	s.Require().Nil(err)

	// Try to update via API
	newName := "updated-agent-tool.bin"
	updateParams := params.UpdateFileObjectParams{
		Name: &newName,
		Tags: []string{"updated"},
	}
	_, err = s.Runner.UpdateFileObject(s.Fixtures.AdminContext, fileObj.ID, updateParams)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "cannot update garm-agent tools via object storage API")

	// Verify file is unchanged
	unchanged, err := s.Fixtures.Store.GetFileObject(s.Fixtures.AdminContext, fileObj.ID)
	s.Require().Nil(err)
	s.Require().Equal(param.Name, unchanged.Name)
}

func (s *ObjectStoreTestSuite) TestUpdateFileObjectAddingGarmAgentTag() {
	// Create a regular file
	content := []byte("regular file")
	param := params.CreateFileObjectParams{
		Name: "regular-file.txt",
		Size: int64(len(content)),
		Tags: []string{"regular"},
	}
	fileObj, err := s.Fixtures.Store.CreateFileObject(s.Fixtures.AdminContext, param, bytes.NewReader(content))
	s.Require().Nil(err)

	// Try to add garm-agent tag via update
	updateParams := params.UpdateFileObjectParams{
		Tags: []string{garmAgentFileTag, "updated"},
	}
	_, err = s.Runner.UpdateFileObject(s.Fixtures.AdminContext, fileObj.ID, updateParams)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "cannot update garm-agent tools via object storage API")

	// Verify file tags are unchanged
	unchanged, err := s.Fixtures.Store.GetFileObject(s.Fixtures.AdminContext, fileObj.ID)
	s.Require().Nil(err)
	s.Require().ElementsMatch(param.Tags, unchanged.Tags)
}

func (s *ObjectStoreTestSuite) TestGetFileObjectReader() {
	reader, err := s.Runner.GetFileObjectReader(s.Fixtures.AdminContext, s.Fixtures.TestFileObject.ID)

	s.Require().Nil(err)
	s.Require().NotNil(reader)
	defer reader.Close()

	// Read the content
	content, err := io.ReadAll(reader)
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.TestFileContent, content)
}

func (s *ObjectStoreTestSuite) TestGetFileObjectReaderUnauthorized() {
	_, err := s.Runner.GetFileObjectReader(s.Fixtures.UnauthorizedContext, s.Fixtures.TestFileObject.ID)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *ObjectStoreTestSuite) TestGetFileObjectReaderNotFound() {
	_, err := s.Runner.GetFileObjectReader(s.Fixtures.AdminContext, 99999)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "failed to open file object")
}

func (s *ObjectStoreTestSuite) TestDeleteFileObjectsByTags() {
	// Create multiple test files with specific tags
	for i := 1; i <= 5; i++ {
		content := []byte(fmt.Sprintf("test file %d", i))
		var tags []string
		if i <= 3 {
			// First 3 files have matching tags
			tags = []string{"tag1=value1", "tag2=value2", "test"}
		} else {
			// Last 2 files have different tags
			tags = []string{"tag1=value1", "other"}
		}
		param := params.CreateFileObjectParams{
			Name: fmt.Sprintf("bulk-delete-test-%d.txt", i),
			Size: int64(len(content)),
			Tags: tags,
		}
		_, err := s.Fixtures.Store.CreateFileObject(s.Fixtures.AdminContext, param, bytes.NewReader(content))
		s.Require().Nil(err)
	}

	// Delete files matching BOTH tags
	deletedCount, err := s.Runner.DeleteFileObjectsByTags(
		s.Fixtures.AdminContext,
		[]string{"tag1=value1", "tag2=value2"},
	)

	s.Require().Nil(err)
	s.Require().Equal(int64(3), deletedCount)

	// Verify the right files were deleted
	allObjects, err := s.Fixtures.Store.ListFileObjects(s.Fixtures.AdminContext, 0, 100)
	s.Require().Nil(err)

	// Count how many bulk-delete-test files remain
	remainingCount := 0
	for _, obj := range allObjects.Results {
		if bytes.Contains([]byte(obj.Name), []byte("bulk-delete-test")) {
			remainingCount++
			// Should only be the last 2 files
			s.Require().Contains(obj.Tags, "other")
		}
	}
	s.Require().Equal(2, remainingCount)
}

func (s *ObjectStoreTestSuite) TestDeleteFileObjectsByTagsUnauthorized() {
	deletedCount, err := s.Runner.DeleteFileObjectsByTags(
		s.Fixtures.UnauthorizedContext,
		[]string{"tag1", "tag2"},
	)

	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
	s.Require().Equal(int64(0), deletedCount)
}

func (s *ObjectStoreTestSuite) TestDeleteFileObjectsByTagsWithGarmAgentTag() {
	// Try to delete with garm-agent tag
	deletedCount, err := s.Runner.DeleteFileObjectsByTags(
		s.Fixtures.AdminContext,
		[]string{"category=garm-agent", "test"},
	)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "cannot delete garm-agent tools via object storage API")
	s.Require().Equal(int64(0), deletedCount)
}

func (s *ObjectStoreTestSuite) TestDeleteFileObjectsByTagsNoMatches() {
	// Try to delete with tags that don't match anything
	deletedCount, err := s.Runner.DeleteFileObjectsByTags(
		s.Fixtures.AdminContext,
		[]string{"nonexistent-tag1", "nonexistent-tag2"},
	)

	s.Require().Nil(err)
	s.Require().Equal(int64(0), deletedCount)
}

func (s *ObjectStoreTestSuite) TestDeleteFileObjectsByTagsEmptyTags() {
	// Try to delete with empty tags list
	deletedCount, err := s.Runner.DeleteFileObjectsByTags(
		s.Fixtures.AdminContext,
		[]string{},
	)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "no tags provided")
	s.Require().Equal(int64(0), deletedCount)
}

func TestObjectStoreTestSuite(t *testing.T) {
	suite.Run(t, new(ObjectStoreTestSuite))
}
