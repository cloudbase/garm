package sql

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util"
)

func (s *sqlDatabase) isPostgres() bool {
	return s.cfg.DbBackend == config.PostgreSQLBackend
}

// rawObjectsDB returns the *sql.DB for the objects store.
// For SQLite it is the dedicated single-connection blob database file; for
// PostgreSQL it is derived from objectsConn, which shares the main connection
// pool (the objects store is not a separate database in PostgreSQL).
func (s *sqlDatabase) rawObjectsDB() *sql.DB {
	if s.objectsSQLDB != nil {
		return s.objectsSQLDB
	}
	// DB() only returns an error if the dialector does not expose a *sql.DB,
	// which is not the case for the SQL dialectors we use.
	db, _ := s.objectsConn.DB()
	return db
}

// streamBlobContent opens a raw SQLite blob handle, streams initialData followed
// by the rest of r into it, and returns the hex-encoded SHA256 of the written content.
// The raw *sql.Conn is closed before returning so the caller can safely use
// s.objectsConn afterwards without pool starvation.
func (s *sqlDatabase) streamBlobContent(ctx context.Context, blobID uint, initialData []byte, r io.Reader) (string, error) {
	conn, err := s.objectsSQLDB.Conn(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get connection from pool: %w", err)
	}
	defer conn.Close()

	var sha256sum string
	err = conn.Raw(func(driverConn any) error {
		sqliteConn := driverConn.(*sqlite3.SQLiteConn)

		blob, err := sqliteConn.Blob("main", "file_blobs", "content", int64(blobID), 1)
		if err != nil {
			return fmt.Errorf("failed to open blob: %w", err)
		}
		defer blob.Close()

		hasher := sha256.New()

		if _, err := blob.Write(initialData); err != nil {
			return fmt.Errorf("failed to write blob initial buffer: %w", err)
		}
		hasher.Write(initialData)

		if _, err := io.Copy(io.MultiWriter(blob, hasher), r); err != nil {
			return fmt.Errorf("failed to write blob: %w", err)
		}

		sha256sum = hex.EncodeToString(hasher.Sum(nil))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to write blob: %w", err)
	}
	return sha256sum, nil
}

// streamToLargeObject writes r into a new PostgreSQL Large Object (pg_largeobject)
// while computing SHA256, and returns the assigned OID and checksum.
// It does not create any application rows; the caller is responsible for storing
// the returned OID in the FileBlob row.
func (s *sqlDatabase) streamToLargeObject(ctx context.Context, sqlDB *sql.DB, r io.Reader) (uint32, string, error) {
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get connection: %w", err)
	}
	defer conn.Close()

	var oid uint32
	var sha256sum string
	err = conn.Raw(func(driverConn any) error {
		pgxConn := driverConn.(*stdlib.Conn).Conn()

		tx, err := pgxConn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		los := tx.LargeObjects()
		oid, err = los.Create(ctx, 0)
		if err != nil {
			_ = tx.Rollback(context.Background())
			return fmt.Errorf("failed to create large object: %w", err)
		}

		lo, err := los.Open(ctx, oid, pgx.LargeObjectModeWrite)
		if err != nil {
			_ = tx.Rollback(context.Background())
			return fmt.Errorf("failed to open large object for writing: %w", err)
		}

		hasher := sha256.New()
		if _, err := io.Copy(io.MultiWriter(lo, hasher), r); err != nil {
			_ = tx.Rollback(context.Background())
			return fmt.Errorf("failed to write to large object: %w", err)
		}

		if err := lo.Close(); err != nil {
			_ = tx.Rollback(context.Background())
			return fmt.Errorf("failed to close large object: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit large object transaction: %w", err)
		}

		sha256sum = hex.EncodeToString(hasher.Sum(nil))
		return nil
	})
	if err != nil {
		return 0, "", err
	}
	return oid, sha256sum, nil
}

func (s *sqlDatabase) CreateFileObject(ctx context.Context, param params.CreateFileObjectParams, reader io.Reader) (fileObjParam params.FileObject, err error) {
	// Save the file to temporary storage first. This allows us to accept a
	// potentially slow reader (e.g. an HTTP request body) without tying up a
	// database resource during the transfer.
	// On SQLite, writing a blob locks the entire database (including readers) for the duration.
	// On PostgreSQL, streaming directly from a slow reader would hold a connection and an open
	// transaction for the full transfer, exhausting the connection pool.
	tmpFile, err := util.GetTmpFileHandle("")
	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to create tmp file: %w", err)
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name()) //nolint:gosec // G703 - path from os.CreateTemp, not user input
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

	if s.isPostgres() {
		return s.createFileObjectPostgres(ctx, param, fileObj, buffer[:n], tmpFile)
	}
	return s.createFileObjectSQLite(ctx, param, fileObj, buffer[:n], tmpFile)
}

func (s *sqlDatabase) createFileObjectPostgres(ctx context.Context, param params.CreateFileObjectParams, fileObj FileObject, bufHead []byte, tmpFile io.ReadSeeker) (params.FileObject, error) {
	// Seek tmpFile back so that io.MultiReader can reconstruct the full stream.
	if _, err := tmpFile.Seek(int64(len(bufHead)), 0); err != nil {
		return params.FileObject{}, fmt.Errorf("failed to seek tmp file: %w", err)
	}
	// bufHead holds the first 8 KB that were already read for type detection; the
	// rest of the content follows in tmpFile (positioned right after those bytes).
	fullReader := io.MultiReader(bytes.NewReader(bufHead), tmpFile)

	oid, sha256sum, err := s.streamToLargeObject(ctx, s.rawObjectsDB(), fullReader)
	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to stream to large object: %w", err)
	}

	fileObj.SHA256 = sha256sum

	var fileBlob FileBlob
	err = s.objectsConn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&fileObj).Error; err != nil {
			return fmt.Errorf("failed to create file object: %w", err)
		}
		fileBlob = FileBlob{
			FileObjectID: fileObj.ID,
			LOOID:        oid,
		}
		if err := tx.Create(&fileBlob).Error; err != nil {
			return fmt.Errorf("failed to create file blob: %w", err)
		}
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
		// Best-effort cleanup of the orphaned Large Object. Log on failure so the
		// OID can be recovered manually via vacuumlo if needed.
		if unlinkErr := s.objectsConn.Exec("SELECT lo_unlink(?)", oid).Error; unlinkErr != nil {
			slog.With(slog.Any("error", unlinkErr)).Error("failed to clean up orphaned large object", "oid", oid)
		}
		return params.FileObject{}, fmt.Errorf("failed to create database entry for blob: %w", err)
	}

	if err := s.objectsConn.Preload("TagsList").Omit("content").First(&fileObj, fileObj.ID).Error; err != nil {
		return params.FileObject{}, fmt.Errorf("failed to get file object: %w", err)
	}
	return s.sqlFileObjectToCommonParams(fileObj), nil
}

func (s *sqlDatabase) createFileObjectSQLite(ctx context.Context, param params.CreateFileObjectParams, fileObj FileObject, bufHead []byte, tmpFile io.ReadSeeker) (params.FileObject, error) {
	// Position tmpFile right after the bytes already captured in bufHead.
	// streamBlobContent writes bufHead first and then copies the rest from
	// tmpFile, so tmpFile must start at len(bufHead).
	if _, err := tmpFile.Seek(int64(len(bufHead)), 0); err != nil {
		return params.FileObject{}, fmt.Errorf("failed to seek tmp file: %w", err)
	}
	var fileBlob FileBlob
	err := s.objectsConn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&fileObj).Error; err != nil {
			return fmt.Errorf("failed to create file object: %w", err)
		}

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

	// Stream file to blob and compute SHA256.
	// We obtain a raw *sql.Conn for the SQLite blob API, which pins a connection
	// from the pool. We must close it before using s.objectsConn again.
	sha256sum, err := s.streamBlobContent(ctx, fileBlob.ID, bufHead, tmpFile)
	if err != nil {
		return params.FileObject{}, err
	}

	if err := s.objectsConn.Model(&fileObj).Update("sha256", sha256sum).Error; err != nil {
		return params.FileObject{}, fmt.Errorf("failed to update sha256sum: %w", err)
	}

	if err := s.objectsConn.Preload("TagsList").Omit("content").First(&fileObj, fileObj.ID).Error; err != nil {
		return params.FileObject{}, fmt.Errorf("failed to get file object: %w", err)
	}
	return s.sqlFileObjectToCommonParams(fileObj), nil
}

func (s *sqlDatabase) UpdateFileObject(_ context.Context, objID uint, param params.UpdateFileObjectParams) (fileObjParam params.FileObject, err error) {
	if err := param.Validate(); err != nil {
		return params.FileObject{}, fmt.Errorf("failed to validate update params: %w", err)
	}

	var rowsAffected int64
	defer func() {
		if err == nil && rowsAffected > 0 {
			s.sendNotify(common.FileObjectEntityType, common.UpdateOperation, fileObjParam)
		}
	}()

	var fileObj FileObject
	err = s.objectsConn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", objID).Omit("content").First(&fileObj).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return runnerErrors.NewNotFoundError("could not find file object with ID: %d", objID)
			}
			return fmt.Errorf("error trying to find file object: %w", err)
		}

		updates := make(map[string]interface{})

		// Update name if provided
		if param.Name != nil {
			updates["name"] = *param.Name
		}

		if param.Description != nil && *param.Description != fileObj.Description {
			updates["description"] = *param.Description
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
		if len(updates) > 0 {
			result := tx.Model(&fileObj).Omit("content").Updates(updates)
			if result.Error != nil {
				return fmt.Errorf("failed to update file object: %w", result.Error)
			}
			rowsAffected = result.RowsAffected
		} else if param.Tags != nil {
			// If only tags changed, we still want to notify
			rowsAffected = 1
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

		if s.isPostgres() {
			if err := tx.Exec("SELECT lo_unlink(lo_oid::oid) FROM file_blobs WHERE file_object_id = ? AND lo_oid != 0", objID).Error; err != nil {
				return fmt.Errorf("failed to unlink large objects: %w", err)
			}
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
			query = query.Where("EXISTS (SELECT 1 FROM file_object_tags WHERE file_object_tags.file_object_id = file_objects.id AND LOWER(file_object_tags.tag) = LOWER(?))", tag)
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

		if s.isPostgres() {
			if err := tx.Exec("SELECT lo_unlink(lo_oid::oid) FROM file_blobs WHERE file_object_id IN ? AND lo_oid != 0", fileObjIDs).Error; err != nil {
				return fmt.Errorf("failed to unlink large objects: %w", err)
			}
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
		query = query.Where("EXISTS (SELECT 1 FROM file_object_tags WHERE file_object_tags.file_object_id = file_objects.id AND LOWER(file_object_tags.tag) = LOWER(?))", t)
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

// OpenFileObjectContent opens a blob for reading and returns an io.ReadCloser.
func (s *sqlDatabase) OpenFileObjectContent(ctx context.Context, objID uint) (io.ReadCloser, error) {
	// Query the blob metadata first, before pinning a raw connection.
	// With MaxOpenConns(1), pinning the connection before this query would
	// deadlock because GORM needs the same pooled connection.
	var fileBlob FileBlob
	if err := s.objectsConn.Where("file_object_id = ?", objID).Omit("content").First(&fileBlob).Error; err != nil {
		return nil, fmt.Errorf("failed to get file blob: %w", err)
	}

	if s.isPostgres() {
		if fileBlob.LOOID == 0 {
			return nil, runnerErrors.ErrNotFound
		}
		return s.openLargeObject(ctx, fileBlob.LOOID)
	}
	return s.openSQLiteBlob(ctx, fileBlob.ID)
}

// openLargeObject opens a PostgreSQL Large Object for reading and returns an
// io.ReadCloser. The underlying connection and transaction remain open until
// Close() is called.
func (s *sqlDatabase) openLargeObject(ctx context.Context, looid uint32) (io.ReadCloser, error) {
	conn, err := s.rawObjectsDB().Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	var lo *pgx.LargeObject
	var tx pgx.Tx
	err = conn.Raw(func(driverConn any) error {
		pgxConn := driverConn.(*stdlib.Conn).Conn()
		var err error
		tx, err = pgxConn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		los := tx.LargeObjects()
		lo, err = los.Open(ctx, looid, pgx.LargeObjectModeRead)
		if err != nil {
			_ = tx.Rollback(context.Background())
			return fmt.Errorf("failed to open large object for reading: %w", err)
		}
		return nil
	})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &loReadCloser{
		lo:      lo,
		tx:      tx,
		sqlConn: conn,
	}, nil
}

// openSQLiteBlob opens the SQLite incremental blob for the given FileBlob row.
func (s *sqlDatabase) openSQLiteBlob(ctx context.Context, blobID uint) (io.ReadCloser, error) {
	conn, err := s.objectsSQLDB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	var blobReader io.ReadCloser
	err = conn.Raw(func(driverConn any) error {
		sqliteConn := driverConn.(*sqlite3.SQLiteConn)

		blob, err := sqliteConn.Blob("main", "file_blobs", "content", int64(blobID), 0)
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
		_ = conn.Close()
		return nil, fmt.Errorf("failed to open blob for reading: %w", err)
	}

	return blobReader, nil
}

// loReadCloser wraps a PostgreSQL Large Object handle, its transaction, and the
// pinned database/sql connection. Close commits the transaction and releases the
// connection back to the pool.
type loReadCloser struct {
	lo      *pgx.LargeObject
	tx      pgx.Tx
	sqlConn *sql.Conn
}

func (r *loReadCloser) Read(p []byte) (int, error) {
	return r.lo.Read(p)
}

func (r *loReadCloser) Close() error {
	// Use a fresh context for cleanup so that a canceled request context does
	// not prevent the transaction from being committed and the connection from
	// being returned to the pool in a clean state.
	// PostgreSQL closes open LO file descriptors automatically when the
	// transaction ends, so lo.Close() errors are intentionally ignored.
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = r.lo.Close()
	txErr := r.tx.Commit(cleanupCtx)
	connErr := r.sqlConn.Close()
	if txErr != nil {
		return txErr
	}
	return connErr
}

// blobReadCloser wraps both the SQLite blob and connection for proper cleanup
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
