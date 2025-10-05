package sql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util"
	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
)

func (s *sqlDatabase) CreateFileObject(ctx context.Context, name string, size int64, tags []string, reader io.Reader) (fileObjParam params.FileObject, err error) {
	// Read first 8KB for type detection
	buffer := make([]byte, 8192)
	n, _ := io.ReadFull(reader, buffer)
	fileType := util.DetectFileType(buffer[:n])
	// Create document with pre-allocated blob
	fileObj := FileObject{
		Name:     name,
		FileType: fileType,
		Size:     size,
		Content:  make([]byte, size),
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.FileObjectEntityType, common.CreateOperation, fileObjParam)
		}
	}()

	if err := s.conn.Create(&fileObj).Error; err != nil {
		return params.FileObject{}, fmt.Errorf("failed to create file object: %w", err)
	}

	// Stream file to blob and compute SHA256
	conn, err := s.sqlDB.Conn(ctx)
	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to get connection from pool: %w", err)
	}
	defer conn.Close()

	var sha256sum string
	err = conn.Raw(func(driverConn any) error {
		sqliteConn := driverConn.(*sqlite3.SQLiteConn)

		blob, err := sqliteConn.Blob("main", fileObj.TableName(), "content", int64(fileObj.ID), 1)
		if err != nil {
			return err
		}
		defer blob.Close()

		// Create SHA256 hasher
		hasher := sha256.New()

		// Write the buffered data first
		if _, err := blob.Write(buffer[:n]); err != nil {
			return err
		}
		hasher.Write(buffer[:n])

		// Stream the rest with hash computation
		_, err = io.Copy(io.MultiWriter(blob, hasher), reader)
		if err != nil {
			return err
		}

		// Get final hash
		sha256sum = hex.EncodeToString(hasher.Sum(nil))
		return nil
	})

	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to write blob: %w", err)
	}

	// Update document with SHA256
	if err := s.conn.Model(&fileObj).Update("sha256", sha256sum).Error; err != nil {
		return params.FileObject{}, fmt.Errorf("failed to update sha256sum: %w", err)
	}

	// Create tag entries
	for _, tag := range tags {
		fileObjTag := FileObjectTag{
			FileObjectID: fileObj.ID,
			Tag:          tag,
		}
		if err := s.conn.Create(&fileObjTag).Error; err != nil {
			return params.FileObject{}, fmt.Errorf("failed to add tag: %w", err)
		}
	}

	// Reload document with tags
	if err := s.conn.Preload("TagsList").Omit("content").First(&fileObj, fileObj.ID).Error; err != nil {
		return params.FileObject{}, fmt.Errorf("failed to get file object: %w", err)
	}
	return s.sqlFileObjectToCommonParams(fileObj), nil
}

func (s *sqlDatabase) UpdateFileObject(ctx context.Context, objID uint, param params.UpdateFileObjectParams) (fileObjParam params.FileObject, err error) {
	if err := param.Validate(); err != nil {
		return params.FileObject{}, fmt.Errorf("failed to validate update params: %w", err)
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.FileObjectEntityType, common.UpdateOperation, fileObjParam)
		}
	}()

	var fileObj FileObject
	err = s.conn.Transaction(func(tx *gorm.DB) error {
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

func (s *sqlDatabase) DeleteFileObject(ctx context.Context, objID uint) (err error) {
	var fileObjParam params.FileObject
	var noop bool
	defer func() {
		if err == nil && !noop {
			s.sendNotify(common.FileObjectEntityType, common.DeleteOperation, fileObjParam)
		}
	}()

	var fileObj FileObject
	err = s.conn.Transaction(func(tx *gorm.DB) error {
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

func (s *sqlDatabase) GetFileObject(ctx context.Context, objID uint) (params.FileObject, error) {
	var fileObj FileObject
	if err := s.conn.Preload("TagsList").Where("id = ?", objID).Omit("content").First(&fileObj).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return params.FileObject{}, runnerErrors.NewNotFoundError("could not find file object with ID: %d", objID)
		}
		return params.FileObject{}, fmt.Errorf("error trying to find file object: %w", err)
	}
	return s.sqlFileObjectToCommonParams(fileObj), nil
}

func (s *sqlDatabase) SearchFileObjectByTags(ctx context.Context, tags []string, page, pageSize uint64) (params.FileObjectPaginatedResponse, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

	var fileObjectRes []FileObject
	query := s.conn.Model(&FileObject{}).Preload("TagsList").Omit("content")
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

	if err := query.
		Limit(int(pageSize)).
		Offset(int(offset)).
		Order("created_at DESC").
		Omit("content").
		Find(&fileObjectRes).Error; err != nil {
		return params.FileObjectPaginatedResponse{}, fmt.Errorf("failed to query database: %w", err)
	}

	ret := make([]params.FileObject, len(fileObjectRes))
	for idx, val := range fileObjectRes {
		ret[idx] = s.sqlFileObjectToCommonParams(val)
	}

	return params.FileObjectPaginatedResponse{
		Pages:       totalPages,
		CurrentPage: page,
		Results:     ret,
	}, nil
}

// OpenFileObjectContent opens a blob for reading and returns an io.ReadCloser
func (s *sqlDatabase) OpenFileObjectContent(ctx context.Context, objID uint) (io.ReadCloser, error) {
	conn, err := s.sqlDB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	var blobReader io.ReadCloser
	err = conn.Raw(func(driverConn any) error {
		sqliteConn := driverConn.(*sqlite3.SQLiteConn)

		blob, err := sqliteConn.Blob("main", (FileObject{}).TableName(), "content", int64(objID), 0)
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

func (s *sqlDatabase) ListFileObjects(ctx context.Context, page, pageSize uint64) (params.FileObjectPaginatedResponse, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

	var total int64
	if err := s.conn.Model(&FileObject{}).Count(&total).Error; err != nil {
		return params.FileObjectPaginatedResponse{}, fmt.Errorf("failed to count file objects: %w", err)
	}

	totalPages := uint64(0)
	if total > 0 {
		totalPages = (uint64(total) + pageSize - 1) / pageSize
	}

	offset := (page - 1) * pageSize
	var fileObjs []FileObject
	if err := s.conn.Preload("TagsList").Omit("content").
		Limit(int(pageSize)).
		Offset(int(offset)).
		Order("created_at DESC").
		Find(&fileObjs).Error; err != nil {
		return params.FileObjectPaginatedResponse{}, fmt.Errorf("failed to list file objects: %w", err)
	}

	results := make([]params.FileObject, len(fileObjs))
	for i, obj := range fileObjs {
		results[i] = s.sqlFileObjectToCommonParams(obj)
	}

	return params.FileObjectPaginatedResponse{
		Pages:       totalPages,
		CurrentPage: page,
		Results:     results,
	}, nil
}

func (s *sqlDatabase) sqlFileObjectToCommonParams(obj FileObject) params.FileObject {
	tags := make([]string, len(obj.TagsList))
	for idx, val := range obj.TagsList {
		tags[idx] = val.Tag
	}
	return params.FileObject{
		ID:        obj.ID,
		CreatedAt: obj.CreatedAt,
		UpdatedAt: obj.UpdatedAt,
		Name:      obj.Name,
		Size:      obj.Size,
		FileType:  obj.FileType,
		SHA256:    obj.SHA256,
		Tags:      tags,
	}
}
