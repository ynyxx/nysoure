package dao

import (
	"errors"
	"nysoure/server/config"
	"nysoure/server/model"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	fileSizeCache struct {
		sync.RWMutex
		value      int64
		expireTime time.Time
	}
	fileSizeCacheTTL = 1 * time.Hour
)

// invalidateFileSizeCache 清除文件大小缓存
func invalidateFileSizeCache() {
	fileSizeCache.Lock()
	fileSizeCache.expireTime = time.Time{}
	fileSizeCache.Unlock()
}

func CreateUploadingFile(filename string, description string, fileSize int64, blockSize int64, tempPath string, resourceID, storageID, userID uint, tag string) (*model.UploadingFile, error) {
	blocksCount := (fileSize + blockSize - 1) / blockSize
	uf := &model.UploadingFile{
		Filename:         filename,
		Description:      description,
		TotalSize:        fileSize,
		BlockSize:        blockSize,
		TempPath:         tempPath,
		Blocks:           make([]bool, blocksCount),
		TargetResourceID: resourceID,
		TargetStorageID:  storageID,
		UserID:           userID,
		Tag:              tag,
	}
	if err := db.Create(uf).Error; err != nil {
		return nil, err
	}
	return uf, nil
}

func GetUploadingFile(id uint) (*model.UploadingFile, error) {
	uf := &model.UploadingFile{}
	if err := db.Where("id = ?", id).First(uf).Error; err != nil {
		return nil, err
	}
	return uf, nil
}

func UpdateUploadingBlock(id uint, blockIndex int) error {
	return db.Transaction(func(tx *gorm.DB) error {
		uf := &model.UploadingFile{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(uf).Error; err != nil {
			return err
		}

		if blockIndex < 0 || blockIndex >= uf.BlocksCount() {
			return nil
		}

		uf.Blocks[blockIndex] = true

		return tx.Save(uf).Error
	})
}

func DeleteUploadingFile(id uint) error {
	uf := &model.UploadingFile{}
	if err := db.Where("id = ?", id).First(uf).Error; err != nil {
		return err
	}
	if err := db.Delete(uf).Error; err != nil {
		return err
	}
	return nil
}

func GetUploadingFilesOlderThan(time time.Time) ([]model.UploadingFile, error) {
	var files []model.UploadingFile
	if err := db.Where("updated_at < ?", time).Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

func CreateFile(filename string, description string, resourceID uint, storageID *uint, storageKey string, redirectUrl string, size int64, userID uint, hash string, tag string) (*model.File, error) {
	if storageID == nil && redirectUrl == "" {
		return nil, errors.New("storageID and redirectUrl cannot be both empty")
	}

	f := &model.File{
		UUID:        uuid.NewString(),
		Filename:    filename,
		Description: description,
		ResourceID:  resourceID,
		StorageID:   storageID,
		RedirectUrl: redirectUrl,
		StorageKey:  storageKey,
		Size:        size,
		UserID:      userID,
		Hash:        hash,
		Tag:         tag,
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(f).Error; err != nil {
			return err
		}
		err := tx.Model(&model.User{}).Where("id = ?", userID).
			UpdateColumn("files_count", gorm.Expr("files_count + ?", 1)).Error
		if err != nil {
			return err
		}
		if config.UpdateModifiedTimeAfterNewFileUpload() {
			err = tx.Model(&model.Resource{}).Where("id = ?", resourceID).
				UpdateColumn("modified_time", time.Now()).Error
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	_ = AddNewFileActivity(userID, f.ID)

	// 清除文件大小缓存
	invalidateFileSizeCache()

	return f, nil
}

func GetFile(id string) (*model.File, error) {
	f := &model.File{}
	if err := db.Preload("Storage").Where("uuid = ?", id).First(f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NewNotFoundError("file not found")
		}
		return nil, err
	}
	return f, nil
}

func GetFileByID(id uint) (*model.File, error) {
	f := &model.File{}
	if err := db.Preload("Storage").Where("id = ?", id).First(f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NewNotFoundError("file not found")
		}
		return nil, err
	}
	return f, nil
}

func DeleteFile(id string) error {
	f := &model.File{}
	if err := db.Where("uuid = ?", id).First(f).Error; err != nil {
		return err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(f).Error; err != nil {
			return err
		}
		if err := tx.
			Model(&model.User{}).
			Where("id = ?", f.UserID).
			UpdateColumn("files_count", gorm.Expr("files_count - ?", 1)).
			Error; err != nil {
			return err
		}
		if err := tx.
			Model(&model.Activity{}).
			Where("type = ? AND ref_id = ?", model.ActivityTypeNewFile, f.ID).
			Delete(&model.Activity{}).
			Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// 清除文件大小缓存
	invalidateFileSizeCache()

	return nil
}

func UpdateFile(id string, filename string, description string, tag string, size int64) (*model.File, error) {
	f := &model.File{}
	if err := db.Where("uuid = ?", id).First(f).Error; err != nil {
		return nil, err
	}
	if filename != "" {
		f.Filename = filename
	}
	if description != "" {
		f.Description = description
	}
	if tag != "" {
		f.Tag = tag
	}
	if size > 0 {
		f.Size = size
	}
	if err := db.Save(f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.NewNotFoundError("file not found")
		}
		return nil, err
	}

	// 如果修改了文件大小，清除缓存
	if size > 0 {
		invalidateFileSizeCache()
	}

	return f, nil
}

func SetFileStorageKey(id string, storageKey string) error {
	f := &model.File{}
	if err := db.Where("uuid = ?", id).First(f).Error; err != nil {
		return err
	}
	f.StorageKey = storageKey
	if err := db.Save(f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.NewNotFoundError("file not found")
		}
		return err
	}
	return nil
}

func SetFileStorageKeyAndSize(id string, storageKey string, size int64, hash string) error {
	f := &model.File{}
	if err := db.Where("uuid = ?", id).First(f).Error; err != nil {
		return err
	}
	f.StorageKey = storageKey
	f.Size = size
	f.Hash = hash
	if err := db.Save(f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.NewNotFoundError("file not found")
		}
		return err
	}

	// 清除文件大小缓存
	invalidateFileSizeCache()

	return nil
}

func UpdateFileStorage(id string, storageID uint, storageKey string) error {
	f := &model.File{}
	if err := db.Where("uuid = ?", id).First(f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.NewNotFoundError("file not found")
		}
		return err
	}
	f.StorageID = &storageID
	f.StorageKey = storageKey
	if err := db.Save(f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.NewNotFoundError("file not found")
		}
		return err
	}
	return nil
}

// UpdateFileStorageIfCurrent updates the file only while it still points at
// expectedStorageID and expectedKey. ok is false when another writer already
// changed the file.
func UpdateFileStorageIfCurrent(id string, expectedStorageID uint, expectedKey string, storageID uint, storageKey string) (bool, error) {
	result := db.Model(&model.File{}).
		Where("uuid = ? AND storage_id = ? AND storage_key = ?", id, expectedStorageID, expectedKey).
		Updates(map[string]any{
			"storage_id":  storageID,
			"storage_key": storageKey,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func ListUserFiles(userID uint, page, pageSize int) ([]*model.File, int64, error) {
	var files []*model.File
	var count int64

	if err := db.Model(&model.File{}).
		Preload("Resource").
		Where("user_id = ?", userID).
		Count(&count).
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&files).Error; err != nil {
		return nil, 0, err
	}
	return files, count, nil
}

func CountFiles() (int64, error) {
	var count int64
	if err := db.Model(&model.File{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountFilesByUserID counts the number of files uploaded by a user
func CountFilesByUserID(userID uint) (int64, error) {
	var count int64
	if err := db.Model(&model.File{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func SumFileSize() (int64, error) {
	// 先尝试读取缓存
	fileSizeCache.RLock()
	if time.Now().Before(fileSizeCache.expireTime) {
		cachedValue := fileSizeCache.value
		fileSizeCache.RUnlock()
		return cachedValue, nil
	}
	fileSizeCache.RUnlock()

	// 缓存已过期或不存在，查询数据库
	var size int64
	var tmpSize *int64
	if err := db.Model(&model.File{}).Select("SUM(size)").Scan(&tmpSize).Error; err != nil {
		return 0, err
	}
	if tmpSize != nil {
		size = *tmpSize
	}

	// 更新缓存
	fileSizeCache.Lock()
	fileSizeCache.value = size
	fileSizeCache.expireTime = time.Now().Add(fileSizeCacheTTL)
	fileSizeCache.Unlock()

	return size, nil
}
