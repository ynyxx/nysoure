package service

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nysoure/server/config"
	ctx2 "nysoure/server/ctx"
	"nysoure/server/dao"
	"nysoure/server/model"
	"nysoure/server/storage"
	"nysoure/server/task"
	"nysoure/server/utils"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
)

const (
	blockSize                  = 4 * 1024 * 1024           // 4MB
	storageKeyUnavailable      = "storage_key_unavailable" // Placeholder for unavailable storage key
	MinUnrequireVerifyFileSize = 10 * 1024 * 1024          // 10MB
)

var bannedRedirectDomains []string

func getUploadingSize() int64 {
	return dao.GetStatistic("uploading_size")
}

func updateUploadingSize(offset int64) {
	_ = dao.UpdateStatistic("uploading_size", offset)
}

func getTempDir() (string, error) {
	name := uuid.NewString()
	path := filepath.Join(utils.GetStoragePath(), "uploading", name)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return "", err
	}
	return path, nil
}

func init() {
	domains := os.Getenv("BANNED_REDIRECT_DOMAINS")
	if domains != "" {
		bannedRedirectDomains = strings.Split(domains, ",")
	}
	go func() {
		// Wait for 1 minute to ensure the database is ready
		time.Sleep(time.Minute)
		for {
			oneDayAgo := time.Now().Add(-24 * time.Hour)
			oldFiles, err := dao.GetUploadingFilesOlderThan(oneDayAgo)
			if err != nil {
				log.Error("failed to get old uploading files: ", err)
			} else {
				for _, file := range oldFiles {
					if err := os.RemoveAll(file.TempPath); err != nil {
						log.Error("failed to remove temp dir: ", err)
					}
					if err := dao.DeleteUploadingFile(file.ID); err != nil {
						log.Error("failed to delete uploading file: ", err)
					}
					updateUploadingSize(-file.TotalSize)
				}
			}
			// Sleep for 1 hour
			time.Sleep(1 * time.Hour)
		}
	}()
}

func CreateUploadingFile(c ctx2.Context, filename string, description string, fileSize int64, resourceID, storageID uint, tag string) (*model.UploadingFileView, error) {
	if filename == "" {
		return nil, model.NewRequestError("filename is empty")
	}
	if len([]rune(filename)) > 128 {
		return nil, model.NewRequestError("filename is too long")
	}
	canUpload := c.UserPermission() >= model.PermissionUploader
	if !canUpload {
		if !config.AllowNormalUserUpload() || fileSize > config.MaxNormalUserUploadSize()*1024*1024 {
			return nil, model.NewUnAuthorizedError("user cannot upload file")
		}
	}

	if fileSize > config.MaxFileSize() {
		return nil, model.NewRequestError("file size exceeds the limit")
	}

	currentUploadingSize := getUploadingSize()
	if currentUploadingSize+fileSize > config.MaxUploadingSize() {
		log.Info("A new uploading file is rejected due to max uploading size limit")
		return nil, model.NewRequestError("server is busy, please try again later")
	}

	tempPath, err := getTempDir()
	if err != nil {
		log.Error("failed to create temp dir: ", err)
		return nil, model.NewInternalServerError("failed to create temp dir")
	}
	uid := c.MustUserID()
	uploadingFile, err := dao.CreateUploadingFile(filename, description, fileSize, blockSize, tempPath, resourceID, storageID, uid, tag)
	if err != nil {
		log.Error("failed to create uploading file: ", err)
		_ = os.Remove(tempPath)
		return nil, model.NewInternalServerError("failed to create uploading file")
	}
	updateUploadingSize(fileSize)
	return uploadingFile.ToView(), nil
}

func UploadBlock(uid uint, fid uint, index int, data []byte) error {
	uploadingFile, err := dao.GetUploadingFile(fid)
	if err != nil {
		log.Error("failed to get uploading file: ", err)
		return model.NewNotFoundError("file not found")
	}
	if uploadingFile.UserID != uid {
		return model.NewUnAuthorizedError("user cannot upload file")
	}
	if len(data) > int(uploadingFile.BlockSize) {
		return model.NewRequestError("block size exceeds the limit")
	}
	if index != uploadingFile.BlocksCount()-1 && len(data) != int(uploadingFile.BlockSize) {
		return model.NewRequestError("block size is not correct")
	}
	if index < 0 || index >= uploadingFile.BlocksCount() {
		return model.NewRequestError("block index is not correct")
	}

	path := filepath.Join(uploadingFile.TempPath, strconv.Itoa(index))
	if err := os.WriteFile(path, data, os.ModePerm); err != nil {
		log.Error("failed to write block file: ", err)
		return model.NewInternalServerError("failed to write block file")
	}
	uploadingFile.Blocks[index] = true
	if err := dao.UpdateUploadingBlock(fid, index); err != nil {
		log.Error("failed to update uploading file: ", err)
		_ = os.Remove(path)
		return model.NewInternalServerError("failed to update uploading file")
	}

	return nil
}

func FinishUploadingFile(uid uint, fid uint, md5Str string) (*model.FileView, error) {
	uploadingFile, err := dao.GetUploadingFile(fid)
	if err != nil {
		log.Error("failed to get uploading file: ", err)
		return nil, model.NewNotFoundError("file not found")
	}
	if uploadingFile.UserID != uid {
		return nil, model.NewUnAuthorizedError("user cannot upload file")
	}

	for i := 0; i < uploadingFile.BlocksCount(); i++ {
		if !uploadingFile.Blocks[i] {
			return nil, model.NewRequestError("file is not completely uploaded")
		}
	}

	tempRemoved := false

	defer func() {
		if !tempRemoved {
			if err := os.RemoveAll(uploadingFile.TempPath); err != nil {
				log.Error("failed to remove temp dir: ", err)
			}
		}
		if err := dao.DeleteUploadingFile(fid); err != nil {
			log.Error("failed to delete uploading file: ", err)
		}
		updateUploadingSize(-uploadingFile.TotalSize)
	}()

	resultFilePath := filepath.Join(utils.GetStoragePath(), uuid.NewString())
	file, err := os.OpenFile(resultFilePath, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		log.Error("failed to open result file: ", err)
		return nil, model.NewInternalServerError("failed to finish uploading file. please re-upload")
	}

	h := md5.New()

	for i := 0; i < uploadingFile.BlocksCount(); i++ {
		blockPath := filepath.Join(uploadingFile.TempPath, strconv.Itoa(i))
		data, err := os.ReadFile(blockPath)
		if err != nil {
			log.Error("failed to read block file: ", err)
			_ = file.Close()
			_ = os.Remove(resultFilePath)
			return nil, model.NewInternalServerError("failed to finish uploading file. please re-upload")
		}
		_, err = h.Write(data)
		if err != nil {
			log.Error("failed to write block data to sha1: ", err)
			_ = file.Close()
			_ = os.Remove(resultFilePath)
			return nil, model.NewInternalServerError("failed to finish uploading file. please re-upload")
		}
		if _, err := file.Write(data); err != nil {
			log.Error("failed to write result file: ", err)
			_ = file.Close()
			_ = os.Remove(resultFilePath)
			return nil, model.NewInternalServerError("failed to finish uploading file. please re-upload")
		}
	}

	_ = file.Close()
	_ = os.RemoveAll(uploadingFile.TempPath)
	tempRemoved = true

	sum := h.Sum(nil)
	sumStr := hex.EncodeToString(sum)
	if sumStr != md5Str {
		_ = os.Remove(resultFilePath)
		return nil, model.NewRequestError("md5 checksum is not correct")
	}

	s, err := dao.GetStorage(uploadingFile.TargetStorageID)
	if err != nil {
		log.Error("failed to get storage: ", err)
		_ = os.Remove(resultFilePath)
		return nil, model.NewInternalServerError("failed to finish uploading file. please re-upload")
	}

	iStorage := storage.NewStorage(s)
	if iStorage == nil {
		log.Error("failed to find storage: ", err)
		_ = os.Remove(resultFilePath)
		return nil, model.NewInternalServerError("failed to finish uploading file. please re-upload")
	}

	dbFile, err := dao.CreateFile(uploadingFile.Filename, uploadingFile.Description, uploadingFile.TargetResourceID, &uploadingFile.TargetStorageID, storageKeyUnavailable, "", uploadingFile.TotalSize, uid, sumStr, uploadingFile.Tag)
	if err != nil {
		log.Error("failed to create file in db: ", err)
		_ = os.Remove(resultFilePath)
		return nil, model.NewInternalServerError("failed to finish uploading file. please re-upload")
	}

	go func() {
		defer func() {
			_ = os.Remove(resultFilePath)
		}()
		err := dao.AddStorageUsage(uploadingFile.TargetStorageID, uploadingFile.TotalSize)
		if err != nil {
			log.Error("failed to add storage usage: ", err)
			_ = dao.DeleteFile(dbFile.UUID)
			return
		}
		storageKey, err := iStorage.Upload(resultFilePath, uploadingFile.Filename)
		if err != nil {
			_ = dao.AddStorageUsage(uploadingFile.TargetStorageID, -uploadingFile.TotalSize)
			log.Error("failed to upload file to storage: ", err)
			_ = dao.DeleteFile(dbFile.UUID)
		} else {
			err = dao.SetFileStorageKey(dbFile.UUID, storageKey)
			if err != nil {
				_ = dao.AddStorageUsage(uploadingFile.TargetStorageID, -uploadingFile.TotalSize)
				_ = iStorage.Delete(storageKey)
				_ = dao.DeleteFile(dbFile.UUID)
				log.Error("failed to set file storage key: ", err)
			}
		}
	}()

	return dbFile.ToView(), nil
}

func CancelUploadingFile(uid uint, fid uint) error {
	uploadingFile, err := dao.GetUploadingFile(fid)
	if err != nil {
		log.Error("failed to get uploading file: ", err)
		return model.NewNotFoundError("file not found")
	}
	if uploadingFile.UserID != uid {
		return model.NewUnAuthorizedError("user cannot cancel uploading file")
	}

	if err := dao.DeleteUploadingFile(fid); err != nil {
		log.Error("failed to delete uploading file: ", err)
		return model.NewInternalServerError("failed to delete uploading file")
	}

	go func() {
		// Wait for 1 second to ensure there is no block being uploading
		time.Sleep(time.Second)
		if err := os.RemoveAll(uploadingFile.TempPath); err != nil {
			log.Error("failed to remove temp dir: ", err)
		}
	}()

	updateUploadingSize(-uploadingFile.TotalSize)

	return nil
}

func CreateRedirectFile(c ctx2.Context, filename string, description string, resourceID uint, redirectUrl string, fileSize int64, md5 string, tag string) (*model.FileView, error) {
	u, err := url.Parse(redirectUrl)
	if err != nil {
		return nil, model.NewRequestError("URL is not valid")
	}
	for _, domain := range bannedRedirectDomains {
		if u.Host == domain {
			return nil, model.NewRequestError(fmt.Sprintf("Domain '%s' is not allowed", domain))
		}
	}

	canUpload := c.UserPermission() >= model.PermissionUploader
	if !canUpload && !config.AllowNormalUserUpload() {
		return nil, model.NewUnAuthorizedError("user cannot upload file")
	}

	uid := c.MustUserID()
	file, err := dao.CreateFile(filename, description, resourceID, nil, "", redirectUrl, fileSize, uid, md5, tag)
	if err != nil {
		log.Error("failed to create file in db: ", err)
		return nil, model.NewInternalServerError("failed to create file in db")
	}

	if fileSize == 0 {
		// asynchronously fetch the file size
		go func() {
			size, err := utils.FetchRedirectFileSize(redirectUrl)
			if err != nil {
				log.Error("failed to fetch file size: ", err)
				return
			}
			_, err = dao.UpdateFile(file.UUID, "", "", "", size)
			if err != nil {
				log.Error("failed to update file size: ", err)
				return
			}
		}()
	}

	return file.ToView(), nil
}

func DeleteFile(c ctx2.Context, fid string) error {
	file, err := dao.GetFile(fid)
	if err != nil {
		log.Error("failed to get file: ", err)
		return model.NewNotFoundError("file not found")
	}

	uid := c.MustUserID()
	isAdmin := c.UserPermission() == model.PermissionAdmin

	if !isAdmin && file.UserID != uid {
		return model.NewUnAuthorizedError("user cannot delete file")
	}

	if file.StorageID != nil {
		iStorage := storage.NewStorage(file.Storage)
		if iStorage != nil {
			if err := iStorage.Delete(file.StorageKey); err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
				log.Error("failed to delete file from storage: ", err)
				return model.NewInternalServerError("failed to delete file from storage")
			}
			_ = dao.AddStorageUsage(*file.StorageID, -file.Size)
		}
	}

	if err := dao.DeleteFile(fid); err != nil {
		log.Error("failed to delete file from db: ", err)
		return model.NewInternalServerError("failed to delete file from db")
	}

	return nil
}

func UpdateFile(c ctx2.Context, fid string, filename string, description string, tag string, size int64) (*model.FileView, error) {
	file, err := dao.GetFile(fid)
	if err != nil {
		log.Error("failed to get file: ", err)
		return nil, model.NewNotFoundError("file not found")
	}

	uid := c.MustUserID()
	isAdmin := c.UserPermission() == model.PermissionAdmin

	if !isAdmin && file.UserID != uid {
		return nil, model.NewUnAuthorizedError("user cannot update file")
	}

	file, err = dao.UpdateFile(fid, filename, description, tag, size)
	if err != nil {
		log.Error("failed to update file in db: ", err)
		return nil, model.NewInternalServerError("failed to update file in db")
	}

	if file.RedirectUrl != "" && file.Size == 0 {
		// asynchronously fetch the file size
		go func() {
			size, err := utils.FetchRedirectFileSize(file.RedirectUrl)
			if err != nil {
				log.Error("failed to fetch file size: ", err)
				return
			}
			_, err = dao.UpdateFile(file.UUID, "", "", "", size)
			if err != nil {
				log.Error("failed to update file size: ", err)
				return
			}
		}()
	}

	return file.ToView(), nil
}

func GetFile(fid string) (*model.FileView, error) {
	file, err := dao.GetFile(fid)
	if err != nil {
		log.Error("failed to get file: ", err)
		return nil, model.NewNotFoundError("file not found")
	}

	return file.ToView(), nil
}

// DownloadFile handles the file download request. Return a presigned URL or a direct file path.
func DownloadFile(fid string, verified, isRealUser bool) (string, string, error) {
	file, err := dao.GetFile(fid)
	if err != nil {
		log.Error("failed to get file: ", err)
		return "", "", model.NewNotFoundError("file not found")
	}
	if file.StorageKey == storageKeyUnavailable {
		return "", "", model.NewRequestError("file is not available, please try again later")
	}

	if !verified && requiresDownloadVerification(file) {
		log.Info("cf token verification failed")
		return "", "", model.NewRequestError("cf token verification failed")
	}

	if file.StorageID == nil {
		if file.RedirectUrl != "" {
			err := dao.AddResourceDownloadCount(file.ResourceID)
			if err != nil {
				log.Errorf("failed to add resource download count: %v", err)
			}
			return file.RedirectUrl, file.Filename, nil
		}
		return "", "", model.NewRequestError("file is not available")
	}

	iStorage := storage.NewStorage(file.Storage)
	if iStorage == nil {
		log.Error("failed to find storage: ", err)
		return "", "", model.NewInternalServerError("failed to find storage")
	}

	if file.StorageKey == "" {
		return "", "", model.NewRequestError("file is not available, please try again later")
	}

	path, err := iStorage.Download(file.StorageKey, file.Filename)
	if err != nil {
		log.Error("failed to download file from storage: ", err)
		return "", "", model.NewInternalServerError("failed to download file from storage")
	}

	if isRealUser {
		err = dao.AddResourceDownloadCount(file.ResourceID)
		if err != nil {
			log.Errorf("failed to add resource download count: %v", err)
		}
	}

	return path, file.Filename, nil
}

func testFileUrl(url string) (int64, error) {
	client := http.Client{Timeout: 10 * time.Second}

	// Try HEAD request first, fallback to GET
	for _, method := range []string{"HEAD", "GET"} {
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			return 0, model.NewRequestError("failed to create HTTP request")
		}

		resp, err := client.Do(req)
		if err != nil {
			if method == "GET" {
				return 0, model.NewRequestError("failed to send HTTP request")
			}
			continue // Try GET if HEAD fails
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if method == "GET" {
				return 0, model.NewRequestError("URL is not accessible, status code: " + resp.Status)
			}
			continue // Try GET if HEAD fails
		}

		contentLengthStr := resp.Header.Get("Content-Length")
		if contentLengthStr == "" {
			if method == "GET" {
				return 0, model.NewRequestError("URL does not provide content length")
			}
			continue // Try GET if HEAD doesn't provide Content-Length
		}

		contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64)
		if err != nil || contentLength <= 0 {
			if method == "GET" {
				return 0, model.NewRequestError("Content-Length is not valid")
			}
			continue // Try GET if HEAD has invalid Content-Length
		}

		return contentLength, nil
	}

	return 0, model.NewRequestError("failed to get valid content length")
}

func shouldUseBTDownload(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "magnet:") {
		return true
	}
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}

	parsed, err := url.Parse(trimmed)
	if err == nil {
		if strings.HasSuffix(strings.ToLower(parsed.Path), ".torrent") {
			return true
		}
	}

	client := http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("HEAD", trimmed, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if strings.Contains(contentType, "application/x-bittorrent") {
		return true
	}

	contentDisposition := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Disposition")))
	return strings.Contains(contentDisposition, ".torrent")
}

// downloadFile return nil if the download is successful or the context is cancelled
func downloadFile(ctx context.Context, url string, path string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		_ = os.Remove(path) // Remove the file if it already exists
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", model.NewRequestError("failed to create HTTP request")
	}
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		// Check if the error is due to context cancellation
		if ctx.Err() != nil {
			return "", nil
		}
		return "", model.NewRequestError("failed to send HTTP request")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", model.NewRequestError("URL is not accessible, status code: " + resp.Status)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return "", model.NewInternalServerError("failed to open file for writing")
	}
	defer file.Close()
	writer := bufio.NewWriter(file)

	h := md5.New()

	buf := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return "", nil
		default:
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := writer.Write(buf[:n]); writeErr != nil {
					return "", model.NewInternalServerError("failed to write to file")
				}
				h.Write(buf[:n])
			}
			if readErr != nil {
				if readErr == io.EOF {
					if err := writer.Flush(); err != nil {
						return "", model.NewInternalServerError("failed to flush writer")
					}
					md5Sum := hex.EncodeToString(h.Sum(nil))
					return md5Sum, nil // Download completed successfully
				}
				if ctx.Err() != nil {
					return "", nil // Context cancelled, return nil
				}
				return "", model.NewInternalServerError("failed to read response body")
			}
		}
	}
}

func CreateServerDownloadTask(c ctx2.Context, url, filename, description string, resourceID, storageID uint, tag string) (*model.FileView, error) {
	if c.UserPermission() < model.PermissionUploader {
		return nil, model.NewUnAuthorizedError("user cannot upload file")
	}

	useBT := shouldUseBTDownload(url)
	contentLength := int64(0)
	if !useBT {
		var err error
		contentLength, err = testFileUrl(url)
		if err != nil {
			log.Error("failed to test file URL: ", err)
			return nil, model.NewRequestError("failed to test file URL: " + err.Error())
		}
	}

	if contentLength+getUploadingSize() > config.MaxUploadingSize() {
		log.Info("A new downloading file is rejected due to max uploading size limit")
		return nil, model.NewRequestError("server is busy, please try again later")
	}

	uid := c.MustUserID()
	file, err := dao.CreateFile(filename, description, resourceID, &storageID, storageKeyUnavailable, "", 0, uid, "", tag)
	if err != nil {
		log.Error("failed to create file in db: ", err)
		return nil, model.NewInternalServerError("failed to create file in db")
	}

	updateUploadingSize(contentLength)

	downloadTask := task.NewServerDownloadTask(file.ID, file.UUID, url, filename, storageID, contentLength, useBT)
	task.RegisterTask(downloadTask)

	go func() {
		if err := downloadTask.Run(); err != nil {
			log.Error("server download task failed: ", downloadTask.ID(), ", err: ", err)
		}
	}()

	return file.ToView(), nil
}

func CreateFileMigrationTask(c ctx2.Context, fileID string, targetStorageID uint) (task.Task, error) {
	if fileID == "" {
		return nil, model.NewRequestError("file_id is required")
	}

	file, err := dao.GetFile(fileID)
	if err != nil {
		log.Error("failed to get file: ", err)
		return nil, model.NewNotFoundError("file not found")
	}

	if file.StorageID == nil || file.RedirectUrl != "" {
		return nil, model.NewRequestError("file is not stored in storage")
	}
	if file.StorageKey == "" || file.StorageKey == storageKeyUnavailable {
		return nil, model.NewRequestError("file is not available, please try again later")
	}

	uid := c.MustUserID()
	isAdmin := c.UserPermission() == model.PermissionAdmin
	if !isAdmin && file.UserID != uid {
		return nil, model.NewUnAuthorizedError("user cannot migrate file")
	}

	if *file.StorageID == targetStorageID {
		return nil, model.NewRequestError("source and target storage are the same")
	}

	targetStorage, err := dao.GetStorage(targetStorageID)
	if err != nil {
		log.Error("failed to get target storage: ", err)
		return nil, model.NewRequestError("target storage not found")
	}
	if storage.NewStorage(targetStorage) == nil {
		return nil, model.NewRequestError("target storage configuration is invalid")
	}

	migrationTask := task.NewFileMigrationTask(file.ID, file.UUID, file.Filename, *file.StorageID, targetStorageID, file.Size)
	if err := task.TryRegisterFileMigration(migrationTask); err != nil {
		migrationTask.Stop()
		if errors.Is(err, task.ErrMigrationAlreadyRunning) {
			return nil, model.NewRequestError("file migration is already running")
		}
		return nil, model.NewInternalServerError("failed to start file migration")
	}

	go func() {
		if err := migrationTask.Run(); err != nil {
			log.Error("file migration task failed: ", migrationTask.ID(), ", err: ", err)
		}
	}()

	return migrationTask, nil
}

func ListUserFiles(username string, page int) ([]*model.FileView, int, error) {
	user, err := dao.GetUserByUsername(username)
	if err != nil {
		log.Error("failed to get user by username: ", err)
		return nil, 0, model.NewNotFoundError("user not found")
	}
	uid := user.ID
	files, total, err := dao.ListUserFiles(uid, page, pageSize)
	if err != nil {
		log.Error("failed to list user files: ", err)
		return nil, 0, model.NewInternalServerError("failed to list user files")
	}

	fileViews := make([]*model.FileView, len(files))
	for i, file := range files {
		fileViews[i] = file.ToViewWithResource()
	}

	totalPages := (total + pageSize - 1) / pageSize

	return fileViews, int(totalPages), nil
}
