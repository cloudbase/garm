package sql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util"
)

// func (s *sqlDatabase) CreateFileObject(ctx context.Context, name string, size int64, tags []string, reader io.Reader) (fileObjParam params.FileObject, err error) {
func (s *sqlDatabase) CreateFileObject(ctx context.Context, param params.CreateFileObjectParams, reader io.Reader) (fileObjParam params.FileObject, err error) {
	// Save the file to temporary storage first. This allows us to accept the entire file, even over
	// a slow connection, without locking the database as we stream the file to the DB.
	// SQLite will lock the entire database (including for readers) when the data is being committed.
	tmpFile, err := util.GetTmpFileHandle("")
	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to create tmp file: %w", err)
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()
	if _, err := io.Copy(tmpFile, reader); err != nil {
		return params.FileObject{}, fmt.Errorf("failed to copy data: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return params.FileObject{}, fmt.Errorf("failed to flush data to disk: %w", err)
	}
	// File has been transferred. We need to seek to the beginning of the file. This same handler will be used
	// to streab the data to the database.
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return params.FileObject{}, fmt.Errorf("failed to seek to beginning: %w", err)
	}
	// Read first 8KB for type detection
	buffer := make([]byte, 8192)
	n, _ := io.ReadFull(tmpFile, buffer)
	fileType := util.DetectFileType(buffer[:n])

	fileObj := FileObject{
		Name:        param.Name,
		Description: param.Description,
		FileType:    fileType,
		Size:        param.Size,
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.FileObjectEntityType, common.CreateOperation, fileObjParam)
		}
	}()

	var fileBlob FileBlob
	err = s.objectsConn.Transaction(func(tx *gorm.DB) error {
		// Create the file first
		if err := tx.Create(&fileObj).Error; err != nil {
			return fmt.Errorf("failed to create file object: %w", err)
		}

		// create the file blob, without any space allocated for the blob.
		fileBlob = FileBlob{
			FileObjectID: fileObj.ID,
		}
		if err := tx.Create(&fileBlob).Error; err != nil {
			return fmt.Errorf("failed to create file blob object: %w", err)
		}

		// allocate space for the blob using the zeroblob() function. This will allow us to avoid
		// having to allocate potentially huge byte arrays in memory and writing that huge blob to
		// disk.
		query := `UPDATE file_blobs SET content = zeroblob(?) WHERE id = ?`
		if err := tx.Exec(query, param.Size, fileBlob.ID).Error; err != nil {
			return fmt.Errorf("failed to allocate disk space: %w", err)
		}
		// Create tag entries
		for _, tag := range param.Tags {
			fileObjTag := FileObjectTag{
				FileObjectID: fileObj.ID,
				Tag:          tag,
			}
			if err := tx.Create(&fileObjTag).Error; err != nil {
				return fmt.Errorf("failed to add tag: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to create database entry for blob: %w", err)
	}
	// Stream file to blob and compute SHA256
	conn, err := s.objectsSQLDB.Conn(ctx)
	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to get connection from pool: %w", err)
	}
	defer conn.Close()

	var sha256sum string
	err = conn.Raw(func(driverConn any) error {
		sqliteConn := driverConn.(*sqlite3.SQLiteConn)

		blob, err := sqliteConn.Blob("main", "file_blobs", "content", int64(fileBlob.ID), 1)
		if err != nil {
			return fmt.Errorf("failed to open blob: %w", err)
		}
		defer blob.Close()

		// Create SHA256 hasher
		hasher := sha256.New()

		// Write the buffered data first
		if _, err := blob.Write(buffer[:n]); err != nil {
			return fmt.Errorf("failed to write blob initial buffer: %w", err)
		}
		hasher.Write(buffer[:n])

		// Stream the rest with hash computation
		_, err = io.Copy(io.MultiWriter(blob, hasher), tmpFile)
		if err != nil {
			return fmt.Errorf("failed to write blob: %w", err)
		}

		// Get final hash
		sha256sum = hex.EncodeToString(hasher.Sum(nil))
		return nil
	})
	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to write blob: %w", err)
	}

	// Update document with SHA256
	if err := s.objectsConn.Model(&fileObj).Update("sha256", sha256sum).Error; err != nil {
		return params.FileObject{}, fmt.Errorf("failed to update sha256sum: %w", err)
	}

	// Reload document with tags
	if err := s.objectsConn.Preload("TagsList").Omit("content").First(&fileObj, fileObj.ID).Error; err != nil {
		return params.FileObject{}, fmt.Errorf("failed to get file object: %w", err)
	}
	return s.sqlFileObjectToCommonParams(fileObj), nil
}

func (s *sqlDatabase) UpdateFileObject(_ context.Context, objID uint, param params.UpdateFileObjectParams) (fileObjParam params.FileObject, err error) {
	if err := param.Validate(); err != nil {
		return params.FileObject{}, fmt.Errorf("failed to validate update params: %w", err)
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.FileObjectEntityType, common.UpdateOperation, fileObjParam)
		}
	}()

	var fileObj FileObject
	err = s.objectsConn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", objID).Omit("content").First(&fileObj).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return runnerErrors.NewNotFoundError("could not find file object with ID: %d", objID)
			}
			return fmt.Errorf("error trying to find file object: %w", err)
		}

		// Update name if provided
		if param.Name != nil {
			fileObj.Name = *param.Name
		}

		if param.Description != nil {
			fileObj.Description = *param.Description
		}

		// Update tags if provided
		if param.Tags != nil {
			// Delete existing tags
			if err := tx.Where("file_object_id = ?", objID).Delete(&FileObjectTag{}).Error; err != nil {
				return fmt.Errorf("failed to delete existing tags: %w", err)
			}

			// Create new tags
			for _, tag := range param.Tags {
				fileObjTag := FileObjectTag{
					FileObjectID: fileObj.ID,
					Tag:          tag,
				}
				if err := tx.Create(&fileObjTag).Error; err != nil {
					return fmt.Errorf("failed to add tag: %w", err)
				}
			}
		}

		// Save the updated file object
		if err := tx.Omit("content").Save(&fileObj).Error; err != nil {
			return fmt.Errorf("failed to update file object: %w", err)
		}

		// Reload with tags
		if err := tx.Preload("TagsList").Omit("content").First(&fileObj, objID).Error; err != nil {
			return fmt.Errorf("failed to reload file object: %w", err)
		}

		return nil
	})
	if err != nil {
		return params.FileObject{}, err
	}

	return s.sqlFileObjectToCommonParams(fileObj), nil
}

func (s *sqlDatabase) DeleteFileObject(_ context.Context, objID uint) (err error) {
	var fileObjParam params.FileObject
	var noop bool
	defer func() {
		if err == nil && !noop {
			s.sendNotify(common.FileObjectEntityType, common.DeleteOperation, fileObjParam)
		}
	}()

	var fileObj FileObject
	err = s.objectsConn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", objID).Omit("content").First(&fileObj).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return runnerErrors.ErrNotFound
			}
			return fmt.Errorf("failed to find file obj: %w", err)
		}
		if q := tx.Unscoped().Where("id = ?", objID).Delete(&FileObject{}); q.Error != nil {
			if errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return runnerErrors.ErrNotFound
			}
			return fmt.Errorf("error deleting file object: %w", q.Error)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, runnerErrors.ErrNotFound) {
			noop = true
			return nil
		}
		return fmt.Errorf("failed to delete file object: %w", err)
	}
	return nil
}

func (s *sqlDatabase) DeleteFileObjectsByTags(_ context.Context, tags []string) (int64, error) {
	if len(tags) == 0 {
		return 0, fmt.Errorf("no tags provided")
	}

	var deletedCount int64

	err := s.objectsConn.Transaction(func(tx *gorm.DB) error {
		// Build query to find all file objects matching ALL tags
		query := tx.Model(&FileObject{}).Preload("TagsList").Omit("content")
		for _, tag := range tags {
			query = query.Where("EXISTS (SELECT 1 FROM file_object_tags WHERE file_object_tags.file_object_id = file_objects.id AND file_object_tags.tag = ?)", tag)
		}

		// Get matching objects with their full details (except content blob)
		var fileObjects []FileObject
		if err := query.Find(&fileObjects).Error; err != nil {
			return fmt.Errorf("failed to find matching objects: %w", err)
		}

		if len(fileObjects) == 0 {
			// No objects match - not an error, just nothing to delete
			return nil
		}

		// Extract IDs for deletion
		fileObjIDs := make([]uint, len(fileObjects))
		for i, obj := range fileObjects {
			fileObjIDs[i] = obj.ID
		}

		// Delete all matching objects (hard delete with Unscoped)
		result := tx.Unscoped().Where("id IN ?", fileObjIDs).Delete(&FileObject{})
		if result.Error != nil {
			return fmt.Errorf("failed to delete objects: %w", result.Error)
		}

		deletedCount = result.RowsAffected

		// Send notifications with full object details for each deleted object
		for _, obj := range fileObjects {
			s.sendNotify(common.FileObjectEntityType, common.DeleteOperation, s.sqlFileObjectToCommonParams(obj))
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	// NOTE: Same as DeleteFileObject - deleted file objects leave empty space
	// in the database. Users should run VACUUM manually to reclaim space.
	// See DeleteFileObject for performance details.

	return deletedCount, nil
}

func (s *sqlDatabase) GetFileObject(_ context.Context, objID uint) (params.FileObject, error) {
	var fileObj FileObject
	if err := s.objectsConn.Preload("TagsList").Where("id = ?", objID).Omit("content").First(&fileObj).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return params.FileObject{}, runnerErrors.NewNotFoundError("could not find file object with ID: %d", objID)
		}
		return params.FileObject{}, fmt.Errorf("error trying to find file object: %w", err)
	}
	return s.sqlFileObjectToCommonParams(fileObj), nil
}

func (s *sqlDatabase) SearchFileObjectByTags(_ context.Context, tags []string, page, pageSize uint64) (params.FileObjectPaginatedResponse, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

	var fileObjectRes []FileObject
	query := s.objectsConn.Model(&FileObject{}).Preload("TagsList").Omit("content")
	for _, t := range tags {
		query = query.Where("EXISTS (SELECT 1 FROM file_object_tags WHERE file_object_tags.file_object_id = file_objects.id AND file_object_tags.tag = ?)", t)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return params.FileObjectPaginatedResponse{}, fmt.Errorf("failed to count results: %w", err)
	}

	totalPages := uint64(0)
	if total > 0 {
		totalPages = (uint64(total) + pageSize - 1) / pageSize
	}

	offset := (page - 1) * pageSize

	queryPageSize := math.MaxInt
	if pageSize <= math.MaxInt {
		queryPageSize = int(pageSize)
	}

	var queryOffset int
	if offset <= math.MaxInt {
		queryOffset = int(offset)
	} else {
		return params.FileObjectPaginatedResponse{}, fmt.Errorf("offset excedes max int size: %d", math.MaxInt)
	}
	if err := query.
		Limit(queryPageSize).
		Offset(queryOffset).
		Order("id DESC").
		Omit("content").
		Find(&fileObjectRes).Error; err != nil {
		return params.FileObjectPaginatedResponse{}, fmt.Errorf("failed to query database: %w", err)
	}

	ret := make([]params.FileObject, len(fileObjectRes))
	for idx, val := range fileObjectRes {
		ret[idx] = s.sqlFileObjectToCommonParams(val)
	}

	// Calculate next and previous page numbers
	var nextPage, previousPage *uint64
	if page < totalPages {
		next := page + 1
		nextPage = &next
	}
	if page > 1 {
		prev := page - 1
		previousPage = &prev
	}

	return params.FileObjectPaginatedResponse{
		TotalCount:   uint64(total),
		Pages:        totalPages,
		CurrentPage:  page,
		NextPage:     nextPage,
		PreviousPage: previousPage,
		Results:      ret,
	}, nil
}

// OpenFileObjectContent opens a blob for reading and returns an io.ReadCloser
func (s *sqlDatabase) OpenFileObjectContent(ctx context.Context, objID uint) (io.ReadCloser, error) {
	conn, err := s.objectsSQLDB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	var fileBlob FileBlob
	if err := s.objectsConn.Where("file_object_id = ?", objID).Omit("content").First(&fileBlob).Error; err != nil {
		return nil, fmt.Errorf("failed to get file blob: %w", err)
	}
	var blobReader io.ReadCloser
	err = conn.Raw(func(driverConn any) error {
		sqliteConn := driverConn.(*sqlite3.SQLiteConn)

		blob, err := sqliteConn.Blob("main", "file_blobs", "content", int64(fileBlob.ID), 0)
		if err != nil {
			return fmt.Errorf("failed to open blob: %w", err)
		}

		// Wrap blob and connection so both are closed when reader is closed
		blobReader = &blobReadCloser{
			blob: blob,
			conn: conn,
		}
		return nil
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open blob for reading: %w", err)
	}

	return blobReader, nil
}

// blobReadCloser wraps both the blob and connection for proper cleanup
type blobReadCloser struct {
	blob io.ReadCloser
	conn *sql.Conn
}

func (b *blobReadCloser) Read(p []byte) (n int, err error) {
	return b.blob.Read(p)
}

func (b *blobReadCloser) Close() error {
	blobErr := b.blob.Close()
	connErr := b.conn.Close()
	if blobErr != nil {
		return blobErr
	}
	return connErr
}

func (s *sqlDatabase) ListFileObjects(_ context.Context, page, pageSize uint64) (params.FileObjectPaginatedResponse, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

	var total int64
	if err := s.objectsConn.Model(&FileObject{}).Count(&total).Error; err != nil {
		return params.FileObjectPaginatedResponse{}, fmt.Errorf("failed to count file objects: %w", err)
	}

	totalPages := uint64(0)
	if total > 0 {
		totalPages = (uint64(total) + pageSize - 1) / pageSize
	}

	offset := (page - 1) * pageSize

	queryPageSize := math.MaxInt
	if pageSize <= math.MaxInt {
		queryPageSize = int(pageSize)
	}

	var queryOffset int
	if offset <= math.MaxInt {
		queryOffset = int(offset)
	} else {
		return params.FileObjectPaginatedResponse{}, fmt.Errorf("offset excedes max int size: %d", math.MaxInt)
	}

	var fileObjs []FileObject
	if err := s.objectsConn.Preload("TagsList").Omit("content").
		Limit(queryPageSize).
		Offset(queryOffset).
		Order("id DESC").
		Find(&fileObjs).Error; err != nil {
		return params.FileObjectPaginatedResponse{}, fmt.Errorf("failed to list file objects: %w", err)
	}

	results := make([]params.FileObject, len(fileObjs))
	for i, obj := range fileObjs {
		results[i] = s.sqlFileObjectToCommonParams(obj)
	}

	// Calculate next and previous page numbers
	var nextPage, previousPage *uint64
	if page < totalPages {
		next := page + 1
		nextPage = &next
	}
	if page > 1 {
		prev := page - 1
		previousPage = &prev
	}

	return params.FileObjectPaginatedResponse{
		TotalCount:   uint64(total),
		Pages:        totalPages,
		CurrentPage:  page,
		NextPage:     nextPage,
		PreviousPage: previousPage,
		Results:      results,
	}, nil
}

func (s *sqlDatabase) sqlFileObjectToCommonParams(obj FileObject) params.FileObject {
	tags := make([]string, len(obj.TagsList))
	for idx, val := range obj.TagsList {
		tags[idx] = val.Tag
	}
	return params.FileObject{
		ID:          obj.ID,
		CreatedAt:   obj.CreatedAt,
		UpdatedAt:   obj.UpdatedAt,
		Name:        obj.Name,
		Size:        obj.Size,
		FileType:    obj.FileType,
		SHA256:      obj.SHA256,
		Description: obj.Description,
		Tags:        tags,
	}
}
