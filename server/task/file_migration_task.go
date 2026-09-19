package task

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/url"
	"nysoure/server/dao"
	"nysoure/server/model"
	"nysoure/server/storage"
	"nysoure/server/utils"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
)

type FileMigrationTask struct {
	id              string
	fileID          uint
	fileUUID        string
	filename        string
	sourceStorageID uint
	targetStorageID uint
	totalSize       int64

	ctx    context.Context
	cancel context.CancelFunc

	transferredBytes atomic.Int64

	mu         sync.RWMutex
	status     TaskStatus
	err        error
	finishTime time.Time
}

func NewFileMigrationTask(fileID uint, fileUUID, filename string, sourceStorageID, targetStorageID uint, totalSize int64) *FileMigrationTask {
	ctx, cancel := context.WithCancel(context.Background())
	return &FileMigrationTask{
		id:              uuid.NewString(),
		fileID:          fileID,
		fileUUID:        fileUUID,
		filename:        filename,
		sourceStorageID: sourceStorageID,
		targetStorageID: targetStorageID,
		totalSize:       totalSize,
		ctx:             ctx,
		cancel:          cancel,
		status:          TaskStatusPending,
	}
}

func (t *FileMigrationTask) ID() string {
	return t.id
}

func (t *FileMigrationTask) Run() error {
	if !t.setRunning() {
		return model.NewRequestError("task is already finished")
	}
	defer t.cancel()

	file, err := dao.GetFileByID(t.fileID)
	if err != nil {
		return t.fail(model.NewNotFoundError("file not found"))
	}
	if file.StorageID == nil || file.StorageKey == "" {
		return t.fail(model.NewRequestError("file is not available"))
	}

	sourceStorage, err := dao.GetStorage(t.sourceStorageID)
	if err != nil {
		log.Error("failed to get source storage: ", err)
		return t.fail(model.NewInternalServerError("failed to get source storage"))
	}
	iSourceStorage := storage.NewStorage(sourceStorage)
	if iSourceStorage == nil {
		return t.fail(model.NewInternalServerError("failed to find source storage"))
	}

	targetStorage, err := dao.GetStorage(t.targetStorageID)
	if err != nil {
		log.Error("failed to get target storage: ", err)
		return t.fail(model.NewInternalServerError("failed to get target storage"))
	}
	iTargetStorage := storage.NewStorage(targetStorage)
	if iTargetStorage == nil {
		return t.fail(model.NewInternalServerError("failed to find target storage"))
	}

	sourcePathOrURL, err := iSourceStorage.Download(file.StorageKey, file.Filename)
	if err != nil {
		if errors.Is(err, storage.ErrFileUnavailable) {
			return t.fail(model.NewRequestError("source file is unavailable"))
		}
		log.Error("failed to get source file download path: ", err)
		return t.fail(model.NewInternalServerError("failed to read source file"))
	}

	tempDir := filepath.Join(utils.GetStoragePath(), "temp")
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		log.Error("failed to create temp dir: ", err)
		return t.fail(model.NewInternalServerError("failed to create temp dir"))
	}
	tempPath := filepath.Join(tempDir, uuid.NewString())
	defer func() {
		if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			log.Error("failed to remove temp file: ", removeErr)
		}
	}()

	if strings.HasPrefix(strings.ToLower(sourcePathOrURL), "http") {
		u, err := url.Parse(sourcePathOrURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			log.Error("invalid source file URL: ", sourcePathOrURL)
			return t.fail(model.NewRequestError("invalid source file URL"))
		}
		q := u.Query()
		if len(q) == 0 {
			token, expiresAt := utils.GenerateDownloadToken(u)
			q.Set("token", token)
			q.Set("expires_at", strconv.FormatInt(expiresAt, 10))
			u.RawQuery = q.Encode()
			sourcePathOrURL = u.String()
		}
		_, err = downloadFileWithProgress(t.ctx, sourcePathOrURL, tempPath, func(transferred int64) {
			t.transferredBytes.Store(transferred)
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || t.ctx.Err() != nil {
				return t.handleCanceled()
			}
			log.Error("failed to download source file: ", err)
			return t.fail(model.NewInternalServerError("failed to download source file"))
		}
	} else {
		err = copyLocalFileWithProgress(t.ctx, sourcePathOrURL, tempPath, func(transferred int64) {
			t.transferredBytes.Store(transferred)
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || t.ctx.Err() != nil {
				return t.handleCanceled()
			}
			if errors.Is(err, os.ErrNotExist) {
				return t.fail(model.NewRequestError("source file is unavailable"))
			}
			log.Error("failed to copy source file: ", err)
			return t.fail(model.NewInternalServerError("failed to copy source file"))
		}
	}

	if err := t.ctx.Err(); err != nil {
		return t.handleCanceled()
	}

	st, err := os.Stat(tempPath)
	if err != nil {
		log.Error("failed to get migrated temp file info: ", err)
		return t.fail(model.NewInternalServerError("failed to read migrated file"))
	}
	size := st.Size()
	if size <= 0 {
		return t.fail(model.NewInternalServerError("migrated file is empty"))
	}
	if t.totalSize <= 0 {
		t.totalSize = size
	}

	newStorageKey, err := iTargetStorage.Upload(tempPath, file.Filename)
	if err != nil {
		log.Error("failed to upload file to target storage: ", err)
		return t.fail(model.NewInternalServerError("failed to upload file to target storage"))
	}

	oldStorageKey := file.StorageKey
	if err := dao.UpdateFileStorage(t.fileUUID, t.targetStorageID, newStorageKey); err != nil {
		log.Error("failed to update file storage info: ", err)
		_ = iTargetStorage.Delete(newStorageKey)
		return t.fail(model.NewInternalServerError("failed to update file storage info"))
	}
	if err := dao.AddStorageUsage(t.targetStorageID, size); err != nil {
		log.Error("failed to update target storage usage: ", err)
		_ = dao.UpdateFileStorage(t.fileUUID, t.sourceStorageID, oldStorageKey)
		_ = iTargetStorage.Delete(newStorageKey)
		return t.fail(model.NewInternalServerError("failed to update storage usage"))
	}

	if err := iSourceStorage.Delete(oldStorageKey); err != nil {
		log.Error("failed to delete source file after migration: ", err)
	} else {
		if err := dao.AddStorageUsage(t.sourceStorageID, -size); err != nil {
			log.Error("failed to update source storage usage: ", err)
		}
	}

	t.transferredBytes.Store(size)
	t.finishWith(TaskStatusCompleted, nil)
	return nil
}

func (t *FileMigrationTask) Progress() float64 {
	if t.Status() == TaskStatusCompleted {
		return 1
	}
	if t.totalSize <= 0 {
		return 0
	}
	progress := float64(t.transferredBytes.Load()) / float64(t.totalSize)
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

func (t *FileMigrationTask) Status() TaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

func (t *FileMigrationTask) Error() error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.err
}

func (t *FileMigrationTask) Stop() {
	if t.isTerminal() {
		return
	}
	t.cancel()
	_ = t.fail(model.NewRequestError("task stopped"))
}

func (t *FileMigrationTask) FinishTime() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.finishTime
}

func (t *FileMigrationTask) setRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status != TaskStatusPending {
		return false
	}
	t.status = TaskStatusRunning
	return true
}

func (t *FileMigrationTask) finishWith(status TaskStatus, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status == TaskStatusCompleted || t.status == TaskStatusFailed {
		return
	}
	t.status = status
	t.err = err
	t.finishTime = time.Now()
}

func (t *FileMigrationTask) fail(err error) error {
	t.finishWith(TaskStatusFailed, err)
	return err
}

func (t *FileMigrationTask) handleCanceled() error {
	return t.fail(model.NewRequestError("task stopped"))
}

func (t *FileMigrationTask) isTerminal() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status == TaskStatusCompleted || t.status == TaskStatusFailed
}

func copyLocalFileWithProgress(ctx context.Context, sourcePath, targetPath string, onProgress func(transferred int64)) error {
	if _, err := os.Stat(targetPath); err == nil {
		_ = os.Remove(targetPath)
	}

	input, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer output.Close()

	writer := bufio.NewWriter(output)
	buf := make([]byte, 64*1024)
	var transferred int64

	for {
		if ctx.Err() != nil {
			return context.Canceled
		}
		n, readErr := input.Read(buf)
		if n > 0 {
			if _, writeErr := writer.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			transferred += int64(n)
			onProgress(transferred)
		}
		if readErr != nil {
			if readErr == io.EOF {
				return writer.Flush()
			}
			return readErr
		}
	}
}
