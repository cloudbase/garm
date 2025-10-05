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

package sql

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type FileStoreTestFixtures struct {
	FileObjects []params.FileObject
}

type FileStoreTestSuite struct {
	suite.Suite
	Store    dbCommon.Store
	ctx      context.Context
	adminCtx context.Context
	Fixtures *FileStoreTestFixtures
}

func (s *FileStoreTestSuite) TearDownTest() {
	watcher.CloseWatcher()
}

func (s *FileStoreTestSuite) SetupTest() {
	ctx := context.Background()
	watcher.InitWatcher(ctx)

	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db

	adminCtx := garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())
	s.adminCtx = adminCtx
	s.ctx = adminCtx

	// Create test file objects
	fileObjects := []params.FileObject{}

	// File 1: Small text file with tags
	content1 := []byte("Hello, World! This is test file 1.")
	fileObj1, err := s.Store.CreateFileObject(s.ctx, "test-file-1.txt", int64(len(content1)), []string{"test", "text"}, bytes.NewReader(content1))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test file 1: %s", err))
	}
	fileObjects = append(fileObjects, fileObj1)

	// File 2: Binary-like content with different tags
	content2 := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00} // PNG header-like
	fileObj2, err := s.Store.CreateFileObject(s.ctx, "test-image.png", int64(len(content2)), []string{"image", "binary"}, bytes.NewReader(content2))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test file 2: %s", err))
	}
	fileObjects = append(fileObjects, fileObj2)

	// File 3: No tags
	content3 := []byte("File without tags.")
	fileObj3, err := s.Store.CreateFileObject(s.ctx, "no-tags.txt", int64(len(content3)), []string{}, bytes.NewReader(content3))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test file 3: %s", err))
	}
	fileObjects = append(fileObjects, fileObj3)

	s.Fixtures = &FileStoreTestFixtures{
		FileObjects: fileObjects,
	}
}

func (s *FileStoreTestSuite) TestCreateFileObject() {
	content := []byte("New test file content")
	tags := []string{"new", "test"}

	fileObj, err := s.Store.CreateFileObject(s.ctx, "new-file.txt", int64(len(content)), tags, bytes.NewReader(content))
	s.Require().Nil(err)
	s.Require().NotZero(fileObj.ID)
	s.Require().Equal("new-file.txt", fileObj.Name)
	s.Require().Equal(int64(len(content)), fileObj.Size)
	s.Require().ElementsMatch(tags, fileObj.Tags)
	s.Require().NotEmpty(fileObj.SHA256)
	s.Require().NotEmpty(fileObj.FileType)

	// Verify SHA256 is correct
	expectedHash := sha256.Sum256(content)
	expectedHashStr := hex.EncodeToString(expectedHash[:])
	s.Require().Equal(expectedHashStr, fileObj.SHA256)
}

func (s *FileStoreTestSuite) TestCreateFileObjectEmpty() {
	content := []byte{}
	fileObj, err := s.Store.CreateFileObject(s.ctx, "empty-file.txt", 0, []string{}, bytes.NewReader(content))
	s.Require().Nil(err)
	s.Require().NotZero(fileObj.ID)
	s.Require().Equal("empty-file.txt", fileObj.Name)
	s.Require().Equal(int64(0), fileObj.Size)
}

func (s *FileStoreTestSuite) TestGetFileObject() {
	fileObj, err := s.Store.GetFileObject(s.ctx, s.Fixtures.FileObjects[0].ID)
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.FileObjects[0].ID, fileObj.ID)
	s.Require().Equal(s.Fixtures.FileObjects[0].Name, fileObj.Name)
	s.Require().Equal(s.Fixtures.FileObjects[0].Size, fileObj.Size)
	s.Require().Equal(s.Fixtures.FileObjects[0].SHA256, fileObj.SHA256)
	s.Require().ElementsMatch(s.Fixtures.FileObjects[0].Tags, fileObj.Tags)
}

func (s *FileStoreTestSuite) TestGetFileObjectNotFound() {
	_, err := s.Store.GetFileObject(s.ctx, 99999)
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *FileStoreTestSuite) TestOpenFileObjectContent() {
	// Create a file with known content
	content := []byte("Test content for reading")
	fileObj, err := s.Store.CreateFileObject(s.ctx, "read-test.txt", int64(len(content)), []string{"read"}, bytes.NewReader(content))
	s.Require().Nil(err)

	// Open and read the content
	reader, err := s.Store.OpenFileObjectContent(s.ctx, fileObj.ID)
	s.Require().Nil(err)
	s.Require().NotNil(reader)
	defer reader.Close()

	readContent, err := io.ReadAll(reader)
	s.Require().Nil(err)
	s.Require().Equal(content, readContent)
}

func (s *FileStoreTestSuite) TestOpenFileObjectContentNotFound() {
	_, err := s.Store.OpenFileObjectContent(s.ctx, 99999)
	s.Require().NotNil(err)
}

func (s *FileStoreTestSuite) TestListFileObjects() {
	result, err := s.Store.ListFileObjects(s.ctx, 1, 10)
	s.Require().Nil(err)
	s.Require().GreaterOrEqual(len(result.Results), len(s.Fixtures.FileObjects))
	s.Require().Equal(uint64(1), result.CurrentPage)
	s.Require().GreaterOrEqual(result.Pages, uint64(1))
	s.Require().GreaterOrEqual(result.TotalCount, uint64(len(s.Fixtures.FileObjects)))

	// First page should not have previous page
	s.Require().Nil(result.PreviousPage)

	// If there are more pages, next page should be set
	if result.Pages > 1 {
		s.Require().NotNil(result.NextPage)
		s.Require().Equal(uint64(2), *result.NextPage)
	}
}

func (s *FileStoreTestSuite) TestListFileObjectsPagination() {
	// Create more files to test pagination
	for i := 0; i < 5; i++ {
		content := []byte(fmt.Sprintf("File %d", i))
		_, err := s.Store.CreateFileObject(s.ctx, fmt.Sprintf("page-test-%d.txt", i), int64(len(content)), []string{"pagination"}, bytes.NewReader(content))
		s.Require().Nil(err)
	}

	// Test first page with page size of 2
	page1, err := s.Store.ListFileObjects(s.ctx, 1, 2)
	s.Require().Nil(err)
	s.Require().Equal(2, len(page1.Results))
	s.Require().Equal(uint64(1), page1.CurrentPage)
	s.Require().GreaterOrEqual(page1.TotalCount, uint64(5))
	s.Require().Nil(page1.PreviousPage, "First page should not have previous page")
	s.Require().NotNil(page1.NextPage, "First page should have next page")
	s.Require().Equal(uint64(2), *page1.NextPage)

	// Test second page
	page2, err := s.Store.ListFileObjects(s.ctx, 2, 2)
	s.Require().Nil(err)
	s.Require().Equal(2, len(page2.Results))
	s.Require().Equal(uint64(2), page2.CurrentPage)
	s.Require().Equal(page1.Pages, page2.Pages)
	s.Require().Equal(page1.TotalCount, page2.TotalCount)
	s.Require().NotNil(page2.PreviousPage, "Second page should have previous page")
	s.Require().Equal(uint64(1), *page2.PreviousPage)
	s.Require().NotNil(page2.NextPage, "Second page should have next page")
	s.Require().Equal(uint64(3), *page2.NextPage)

	// Verify different results on different pages
	if len(page1.Results) > 0 && len(page2.Results) > 0 {
		page1File := page1.Results[0]
		page2File := page2.Results[0]
		s.Require().NotEqual(page1File.ID, page2File.ID)
	}
}

func (s *FileStoreTestSuite) TestListFileObjectsDefaultPagination() {
	// Test default values (page 0 should become 1, pageSize 0 should become 20)
	result, err := s.Store.ListFileObjects(s.ctx, 0, 0)
	s.Require().Nil(err)
	s.Require().Equal(uint64(1), result.CurrentPage)
	s.Require().LessOrEqual(len(result.Results), 20)
}

func (s *FileStoreTestSuite) TestUpdateFileObjectName() {
	newName := "updated-name.txt"
	updated, err := s.Store.UpdateFileObject(s.ctx, s.Fixtures.FileObjects[0].ID, params.UpdateFileObjectParams{
		Name: &newName,
	})
	s.Require().Nil(err)
	s.Require().Equal(newName, updated.Name)
	s.Require().Equal(s.Fixtures.FileObjects[0].ID, updated.ID)
	s.Require().Equal(s.Fixtures.FileObjects[0].Size, updated.Size) // Size should not change
	s.Require().Equal(s.Fixtures.FileObjects[0].SHA256, updated.SHA256) // SHA256 should not change

	// Verify the change persists
	retrieved, err := s.Store.GetFileObject(s.ctx, s.Fixtures.FileObjects[0].ID)
	s.Require().Nil(err)
	s.Require().Equal(newName, retrieved.Name)
}

func (s *FileStoreTestSuite) TestUpdateFileObjectTags() {
	newTags := []string{"updated", "tags", "here"}
	updated, err := s.Store.UpdateFileObject(s.ctx, s.Fixtures.FileObjects[0].ID, params.UpdateFileObjectParams{
		Tags: newTags,
	})
	s.Require().Nil(err)
	s.Require().ElementsMatch(newTags, updated.Tags)
	s.Require().Equal(s.Fixtures.FileObjects[0].Name, updated.Name) // Name should not change

	// Verify the change persists
	retrieved, err := s.Store.GetFileObject(s.ctx, s.Fixtures.FileObjects[0].ID)
	s.Require().Nil(err)
	s.Require().ElementsMatch(newTags, retrieved.Tags)
}

func (s *FileStoreTestSuite) TestUpdateFileObjectNameAndTags() {
	newName := "completely-updated.txt"
	newTags := []string{"both", "changed"}

	updated, err := s.Store.UpdateFileObject(s.ctx, s.Fixtures.FileObjects[0].ID, params.UpdateFileObjectParams{
		Name: &newName,
		Tags: newTags,
	})
	s.Require().Nil(err)
	s.Require().Equal(newName, updated.Name)
	s.Require().ElementsMatch(newTags, updated.Tags)
}

func (s *FileStoreTestSuite) TestUpdateFileObjectEmptyTags() {
	// Test clearing all tags
	emptyTags := []string{}
	updated, err := s.Store.UpdateFileObject(s.ctx, s.Fixtures.FileObjects[0].ID, params.UpdateFileObjectParams{
		Tags: emptyTags,
	})
	s.Require().Nil(err)
	s.Require().Empty(updated.Tags)

	// Verify the change persists
	retrieved, err := s.Store.GetFileObject(s.ctx, s.Fixtures.FileObjects[0].ID)
	s.Require().Nil(err)
	s.Require().Empty(retrieved.Tags)
}

func (s *FileStoreTestSuite) TestUpdateFileObjectNoChanges() {
	// Update with no changes
	updated, err := s.Store.UpdateFileObject(s.ctx, s.Fixtures.FileObjects[0].ID, params.UpdateFileObjectParams{})
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.FileObjects[0].Name, updated.Name)
	s.Require().ElementsMatch(s.Fixtures.FileObjects[0].Tags, updated.Tags)
}

func (s *FileStoreTestSuite) TestUpdateFileObjectNotFound() {
	newName := "does-not-exist.txt"
	_, err := s.Store.UpdateFileObject(s.ctx, 99999, params.UpdateFileObjectParams{
		Name: &newName,
	})
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *FileStoreTestSuite) TestUpdateFileObjectEmptyName() {
	emptyName := ""
	_, err := s.Store.UpdateFileObject(s.ctx, s.Fixtures.FileObjects[0].ID, params.UpdateFileObjectParams{
		Name: &emptyName,
	})
	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "name cannot be empty")
}

func (s *FileStoreTestSuite) TestDeleteFileObject() {
	// Create a file to delete
	content := []byte("To be deleted")
	fileObj, err := s.Store.CreateFileObject(s.ctx, "delete-me.txt", int64(len(content)), []string{"delete"}, bytes.NewReader(content))
	s.Require().Nil(err)

	// Delete the file
	err = s.Store.DeleteFileObject(s.ctx, fileObj.ID)
	s.Require().Nil(err)

	// Verify it's deleted
	_, err = s.Store.GetFileObject(s.ctx, fileObj.ID)
	s.Require().NotNil(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *FileStoreTestSuite) TestDeleteFileObjectNotFound() {
	// Deleting non-existent file should not error
	err := s.Store.DeleteFileObject(s.ctx, 99999)
	s.Require().Nil(err)
}

func (s *FileStoreTestSuite) TestCreateFileObjectLargeContent() {
	// Test with larger content (1MB)
	size := 1024 * 1024
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}

	fileObj, err := s.Store.CreateFileObject(s.ctx, "large-file.bin", int64(size), []string{"large", "binary"}, bytes.NewReader(content))
	s.Require().Nil(err)
	s.Require().Equal(int64(size), fileObj.Size)

	// Verify we can read it back
	reader, err := s.Store.OpenFileObjectContent(s.ctx, fileObj.ID)
	s.Require().Nil(err)
	defer reader.Close()

	readContent, err := io.ReadAll(reader)
	s.Require().Nil(err)
	s.Require().Equal(content, readContent)
}

func (s *FileStoreTestSuite) TestFileObjectImmutableFields() {
	// Create a file
	content := []byte("Immutable test content")
	fileObj, err := s.Store.CreateFileObject(s.ctx, "immutable-test.txt", int64(len(content)), []string{"original"}, bytes.NewReader(content))
	s.Require().Nil(err)

	originalSize := fileObj.Size
	originalSHA256 := fileObj.SHA256
	originalFileType := fileObj.FileType

	// Update name and tags
	newName := "updated-immutable-test.txt"
	updated, err := s.Store.UpdateFileObject(s.ctx, fileObj.ID, params.UpdateFileObjectParams{
		Name: &newName,
		Tags: []string{"updated"},
	})
	s.Require().Nil(err)

	// Verify immutable fields haven't changed
	s.Require().Equal(originalSize, updated.Size)
	s.Require().Equal(originalSHA256, updated.SHA256)
	s.Require().Equal(originalFileType, updated.FileType)

	// Verify content hasn't changed
	reader, err := s.Store.OpenFileObjectContent(s.ctx, fileObj.ID)
	s.Require().Nil(err)
	defer reader.Close()

	readContent, err := io.ReadAll(reader)
	s.Require().Nil(err)
	s.Require().Equal(content, readContent)
}

func (s *FileStoreTestSuite) TestSearchFileObjectByTags() {
	// Create files with specific tags for searching
	content1 := []byte("File with tag1 and tag2")
	file1, err := s.Store.CreateFileObject(s.ctx, "search-file-1.txt", int64(len(content1)), []string{"tag1", "tag2"}, bytes.NewReader(content1))
	s.Require().Nil(err)

	content2 := []byte("File with tag1, tag2, and tag3")
	file2, err := s.Store.CreateFileObject(s.ctx, "search-file-2.txt", int64(len(content2)), []string{"tag1", "tag2", "tag3"}, bytes.NewReader(content2))
	s.Require().Nil(err)

	content3 := []byte("File with only tag1")
	file3, err := s.Store.CreateFileObject(s.ctx, "search-file-3.txt", int64(len(content3)), []string{"tag1"}, bytes.NewReader(content3))
	s.Require().Nil(err)

	content4 := []byte("File with tag3 only")
	_, err = s.Store.CreateFileObject(s.ctx, "search-file-4.txt", int64(len(content4)), []string{"tag3"}, bytes.NewReader(content4))
	s.Require().Nil(err)

	// Search for files with tag1 - should return 3 files
	result, err := s.Store.SearchFileObjectByTags(s.ctx, []string{"tag1"}, 1, 10)
	s.Require().Nil(err)
	s.Require().GreaterOrEqual(len(result.Results), 3)

	// Verify the expected files are in the results
	foundIDs := make(map[uint]bool)
	for _, fileObj := range result.Results {
		foundIDs[fileObj.ID] = true
	}
	s.Require().True(foundIDs[file1.ID], "file1 should be in results")
	s.Require().True(foundIDs[file2.ID], "file2 should be in results")
	s.Require().True(foundIDs[file3.ID], "file3 should be in results")
}

func (s *FileStoreTestSuite) TestSearchFileObjectByTagsMultipleTags() {
	// Create files with various tag combinations
	content1 := []byte("File with search1 and search2")
	file1, err := s.Store.CreateFileObject(s.ctx, "multi-search-1.txt", int64(len(content1)), []string{"search1", "search2"}, bytes.NewReader(content1))
	s.Require().Nil(err)

	content2 := []byte("File with search1, search2, and search3")
	file2, err := s.Store.CreateFileObject(s.ctx, "multi-search-2.txt", int64(len(content2)), []string{"search1", "search2", "search3"}, bytes.NewReader(content2))
	s.Require().Nil(err)

	content3 := []byte("File with only search1")
	_, err = s.Store.CreateFileObject(s.ctx, "multi-search-3.txt", int64(len(content3)), []string{"search1"}, bytes.NewReader(content3))
	s.Require().Nil(err)

	// Search for files with both search1 AND search2 - should return only 2 files
	result, err := s.Store.SearchFileObjectByTags(s.ctx, []string{"search1", "search2"}, 1, 10)
	s.Require().Nil(err)
	s.Require().Equal(2, len(result.Results))

	// Verify the correct files are returned
	foundIDs := make(map[uint]bool)
	for _, fileObj := range result.Results {
		foundIDs[fileObj.ID] = true
	}
	s.Require().True(foundIDs[file1.ID], "file1 should be in results")
	s.Require().True(foundIDs[file2.ID], "file2 should be in results")
}

func (s *FileStoreTestSuite) TestSearchFileObjectByTagsNoResults() {
	// Search for a tag that doesn't exist
	result, err := s.Store.SearchFileObjectByTags(s.ctx, []string{"nonexistent-tag"}, 1, 10)
	s.Require().Nil(err)
	s.Require().Equal(0, len(result.Results))
	s.Require().Equal(uint64(0), result.Pages)
	s.Require().Equal(uint64(1), result.CurrentPage)
}

func (s *FileStoreTestSuite) TestSearchFileObjectByTagsEmptyTags() {
	// Search with empty tag list - should return all files
	result, err := s.Store.SearchFileObjectByTags(s.ctx, []string{}, 1, 100)
	s.Require().Nil(err)
	// Should return all files (fixtures + any created in other tests)
	s.Require().GreaterOrEqual(len(result.Results), len(s.Fixtures.FileObjects))
}

func (s *FileStoreTestSuite) TestSearchFileObjectByTagsPagination() {
	// Create multiple files with the same tag
	for i := 0; i < 5; i++ {
		content := []byte(fmt.Sprintf("Pagination test file %d", i))
		_, err := s.Store.CreateFileObject(s.ctx, fmt.Sprintf("page-search-%d.txt", i), int64(len(content)), []string{"pagination-test"}, bytes.NewReader(content))
		s.Require().Nil(err)
	}

	// Test first page with page size of 2
	page1, err := s.Store.SearchFileObjectByTags(s.ctx, []string{"pagination-test"}, 1, 2)
	s.Require().Nil(err)
	s.Require().Equal(2, len(page1.Results))
	s.Require().Equal(uint64(1), page1.CurrentPage)
	s.Require().GreaterOrEqual(page1.Pages, uint64(3)) // At least 3 pages for 5 files

	// Test second page
	page2, err := s.Store.SearchFileObjectByTags(s.ctx, []string{"pagination-test"}, 2, 2)
	s.Require().Nil(err)
	s.Require().Equal(2, len(page2.Results))
	s.Require().Equal(uint64(2), page2.CurrentPage)

	// Verify different results on different pages
	if len(page1.Results) > 0 && len(page2.Results) > 0 {
		page1File := page1.Results[0]
		page2File := page2.Results[0]
		s.Require().NotEqual(page1File.ID, page2File.ID)
	}
}

func (s *FileStoreTestSuite) TestSearchFileObjectByTagsDefaultPagination() {
	// Create a file with a unique tag
	content := []byte("Default pagination test")
	_, err := s.Store.CreateFileObject(s.ctx, "default-page-search.txt", int64(len(content)), []string{"default-pagination"}, bytes.NewReader(content))
	s.Require().Nil(err)

	// Test default values (page 0 should become 1, pageSize 0 should become 20)
	result, err := s.Store.SearchFileObjectByTags(s.ctx, []string{"default-pagination"}, 0, 0)
	s.Require().Nil(err)
	s.Require().Equal(uint64(1), result.CurrentPage)
	s.Require().LessOrEqual(len(result.Results), 20)
	s.Require().GreaterOrEqual(len(result.Results), 1)
}

func (s *FileStoreTestSuite) TestSearchFileObjectByTagsAllTagsRequired() {
	// Test that search requires ALL specified tags (AND logic, not OR)
	content1 := []byte("Has A and B")
	file1, err := s.Store.CreateFileObject(s.ctx, "and-test-1.txt", int64(len(content1)), []string{"tagA", "tagB"}, bytes.NewReader(content1))
	s.Require().Nil(err)

	content2 := []byte("Has A, B, and C")
	file2, err := s.Store.CreateFileObject(s.ctx, "and-test-2.txt", int64(len(content2)), []string{"tagA", "tagB", "tagC"}, bytes.NewReader(content2))
	s.Require().Nil(err)

	content3 := []byte("Has only A")
	_, err = s.Store.CreateFileObject(s.ctx, "and-test-3.txt", int64(len(content3)), []string{"tagA"}, bytes.NewReader(content3))
	s.Require().Nil(err)

	content4 := []byte("Has only B")
	_, err = s.Store.CreateFileObject(s.ctx, "and-test-4.txt", int64(len(content4)), []string{"tagB"}, bytes.NewReader(content4))
	s.Require().Nil(err)

	// Search for files with BOTH tagA AND tagB
	result, err := s.Store.SearchFileObjectByTags(s.ctx, []string{"tagA", "tagB"}, 1, 10)
	s.Require().Nil(err)
	s.Require().Equal(2, len(result.Results), "Should only return files with BOTH tags")

	// Verify the correct files are returned
	foundIDs := make(map[uint]bool)
	for _, fileObj := range result.Results {
		foundIDs[fileObj.ID] = true
		// Verify each result has both tags
		s.Require().Contains(fileObj.Tags, "tagA")
		s.Require().Contains(fileObj.Tags, "tagB")
	}
	s.Require().True(foundIDs[file1.ID])
	s.Require().True(foundIDs[file2.ID])
}

func (s *FileStoreTestSuite) TestSearchFileObjectByTagsCaseSensitive() {
	// Test case sensitivity of tag search
	content1 := []byte("File with lowercase tag")
	file1, err := s.Store.CreateFileObject(s.ctx, "case-test-1.txt", int64(len(content1)), []string{"lowercase"}, bytes.NewReader(content1))
	s.Require().Nil(err)

	content2 := []byte("File with UPPERCASE tag")
	file2, err := s.Store.CreateFileObject(s.ctx, "case-test-2.txt", int64(len(content2)), []string{"UPPERCASE"}, bytes.NewReader(content2))
	s.Require().Nil(err)

	// Search for lowercase - should only return file1
	result, err := s.Store.SearchFileObjectByTags(s.ctx, []string{"lowercase"}, 1, 10)
	s.Require().Nil(err)
	s.Require().Equal(1, len(result.Results))
	s.Require().Equal(file1.ID, result.Results[0].ID)

	// Search for UPPERCASE - should only return file2
	result, err = s.Store.SearchFileObjectByTags(s.ctx, []string{"UPPERCASE"}, 1, 10)
	s.Require().Nil(err)
	s.Require().Equal(1, len(result.Results))
	s.Require().Equal(file2.ID, result.Results[0].ID)
}

func (s *FileStoreTestSuite) TestSearchFileObjectByTagsOrderByCreatedAt() {
	// Create files with same tag at different times to test ordering
	tag := "order-test"

	content1 := []byte("First file")
	file1, err := s.Store.CreateFileObject(s.ctx, "order-1.txt", int64(len(content1)), []string{tag}, bytes.NewReader(content1))
	s.Require().Nil(err)

	content2 := []byte("Second file")
	file2, err := s.Store.CreateFileObject(s.ctx, "order-2.txt", int64(len(content2)), []string{tag}, bytes.NewReader(content2))
	s.Require().Nil(err)

	content3 := []byte("Third file")
	file3, err := s.Store.CreateFileObject(s.ctx, "order-3.txt", int64(len(content3)), []string{tag}, bytes.NewReader(content3))
	s.Require().Nil(err)

	// Search and verify order (should be DESC by created_at, so newest first)
	result, err := s.Store.SearchFileObjectByTags(s.ctx, []string{tag}, 1, 10)
	s.Require().Nil(err)
	s.Require().GreaterOrEqual(len(result.Results), 3)

	// The most recently created files should be first
	// We can at least verify that file3 comes before file1 in the results
	var file1Idx, file3Idx int
	for i, fileObj := range result.Results {
		if fileObj.ID == file1.ID {
			file1Idx = i
		}
		if fileObj.ID == file3.ID {
			file3Idx = i
		}
	}
	s.Require().Less(file3Idx, file1Idx, "Newer file (file3) should appear before older file (file1)")

	// Also verify file2 comes before file1
	var file2Idx int
	for i, fileObj := range result.Results {
		if fileObj.ID == file2.ID {
			file2Idx = i
		}
	}
	s.Require().Less(file2Idx, file1Idx, "Newer file (file2) should appear before older file (file1)")
}

func (s *FileStoreTestSuite) TestPaginationFieldsLastPage() {
	// Create exactly 5 files
	for i := 0; i < 5; i++ {
		content := []byte(fmt.Sprintf("Last page test %d", i))
		_, err := s.Store.CreateFileObject(s.ctx, fmt.Sprintf("last-page-test-%d.txt", i), int64(len(content)), []string{"last-page"}, bytes.NewReader(content))
		s.Require().Nil(err)
	}

	// Get the last page (should have 1 item with pageSize=2)
	// Total: 5 items, pageSize: 2, so pages: 3 (2, 2, 1)
	lastPage, err := s.Store.SearchFileObjectByTags(s.ctx, []string{"last-page"}, 3, 2)
	s.Require().Nil(err)
	s.Require().Equal(uint64(3), lastPage.CurrentPage)
	s.Require().Equal(uint64(3), lastPage.Pages)
	s.Require().Equal(uint64(5), lastPage.TotalCount)
	s.Require().Equal(1, len(lastPage.Results), "Last page should have 1 item")
	s.Require().NotNil(lastPage.PreviousPage, "Last page should have previous page")
	s.Require().Equal(uint64(2), *lastPage.PreviousPage)
	s.Require().Nil(lastPage.NextPage, "Last page should not have next page")
}

func (s *FileStoreTestSuite) TestPaginationFieldsSinglePage() {
	// Test when all results fit in a single page
	content := []byte("Single page test")
	_, err := s.Store.CreateFileObject(s.ctx, "single-page-test.txt", int64(len(content)), []string{"single-page-unique-tag"}, bytes.NewReader(content))
	s.Require().Nil(err)

	result, err := s.Store.SearchFileObjectByTags(s.ctx, []string{"single-page-unique-tag"}, 1, 20)
	s.Require().Nil(err)
	s.Require().Equal(uint64(1), result.TotalCount)
	s.Require().Equal(uint64(1), result.Pages)
	s.Require().Equal(uint64(1), result.CurrentPage)
	s.Require().Nil(result.PreviousPage, "Single page should not have previous")
	s.Require().Nil(result.NextPage, "Single page should not have next")
}

func TestFileStoreTestSuite(t *testing.T) {
	suite.Run(t, new(FileStoreTestSuite))
}
