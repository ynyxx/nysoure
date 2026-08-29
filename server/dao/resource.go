package dao

import (
	"errors"
	"fmt"
	"math/rand"
	"nysoure/server/model"
	"nysoure/server/utils"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3/log"

	"gorm.io/gorm"
)

func CreateResource(r model.Resource) (model.Resource, error) {
	err := db.Transaction(func(tx *gorm.DB) error {
		r.ModifiedTime = time.Now()
		characters := r.Characters
		r.Characters = nil
		err := tx.Create(&r).Error
		if err != nil {
			return err
		}
		for _, c := range characters {
			c.ResourceID = r.ID
			// If ImageID is 0, set it to nil to avoid foreign key constraint error
			if c.ImageID != nil && *c.ImageID == 0 {
				c.ImageID = nil
			}
			if err := tx.Create(&c).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.User{}).Where("id = ?", r.UserID).Update("resources_count", gorm.Expr("resources_count + ?", 1)).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return model.Resource{}, err
	}
	return r, nil
}

func GetResourceByID(id uint) (model.Resource, error) {
	// Retrieve a resource by its ID from the database
	var r model.Resource
	if err := db.Preload("User").
		Preload("Images").
		Preload("Tags", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "type", "alias_of")
		}).
		Preload("Files", func(db *gorm.DB) *gorm.DB {
			return db.Order("files.updated_at ASC")
		}).
		Preload("Files.User").
		Preload("Files.Storage").
		Preload("Characters").
		First(&r, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Resource{}, model.NewNotFoundError("Resource not found")
		}
		return model.Resource{}, err
	}
	for i, tag := range r.Tags {
		if tag.AliasOf != nil {
			t, err := GetTagByID(*tag.AliasOf)
			if err != nil {
				return model.Resource{}, err
			} else {
				r.Tags[i].Type = t.Type
			}
		}
	}
	return r, nil
}

func resourceOrderAndWhere(sort model.RSort) (string, string) {
	order := ""
	where := ""
	switch sort {
	case model.RSortTimeAsc:
		order = "modified_time ASC, id ASC"
	case model.RSortTimeDesc:
		order = "modified_time DESC, id DESC"
	case model.RSortViewsAsc:
		order = "views ASC, id ASC"
	case model.RSortViewsDesc:
		order = "views DESC, id DESC"
	case model.RSortDownloadsAsc:
		order = "downloads ASC, id ASC"
	case model.RSortDownloadsDesc:
		order = "downloads DESC, id DESC"
	case model.RSortReleaseDateAsc:
		order = "release_date ASC, id ASC"
		where = "release_date is not null"
	case model.RSortReleaseDateDesc:
		order = "release_date DESC, id DESC"
		where = "release_date is not null"
	default:
		order = "modified_time DESC, id DESC" // Default sort order
	}
	return order, where
}

func GetResourceList(page, pageSize int, sort model.RSort) ([]model.Resource, int, error) {
	// Retrieve a list of resources with pagination
	var resources []model.Resource
	var total int64

	order, where := resourceOrderAndWhere(sort)

	countQuery := db.Model(&model.Resource{})
	if where != "" {
		countQuery = countQuery.Where(where)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := db.Offset((page - 1) * pageSize).Limit(pageSize).Preload("User").Preload("Images").Preload("Tags").Order(order)
	if where != "" {
		query = query.Where(where)
	}

	if err := query.Find(&resources).Error; err != nil {
		return nil, 0, err
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	return resources, int(totalPages), nil
}

func UpdateResource(r model.Resource, skipUpdateTime bool) error {
	// Update a resource in the database
	return db.Transaction(func(tx *gorm.DB) error {
		images := r.Images
		tags := r.Tags
		characters := r.Characters
		r.Characters = nil
		r.Images = nil
		r.Tags = nil
		r.Files = nil
		if !skipUpdateTime {
			r.ModifiedTime = time.Now()
		}
		oldCharacters := []model.Character{}
		if err := tx.Model(&model.Character{}).Where("resource_id = ?", r.ID).Find(&oldCharacters).Error; err != nil {
			return err
		}
		if err := tx.Save(&r).Error; err != nil {
			return err
		}
		if err := tx.Model(&r).Association("Images").Replace(images); err != nil {
			return err
		}
		if err := tx.Model(&r).Association("Tags").Replace(tags); err != nil {
			return err
		}
		for _, c := range oldCharacters {
			shouldDelete := true
			for _, nc := range characters {
				if c.Equal(&nc) {
					shouldDelete = false
					break
				}
			}
			if shouldDelete {
				if err := tx.Delete(&c).Error; err != nil {
					return err
				}
			}
		}
		for _, c := range characters {
			shouldAdd := true
			for _, oc := range oldCharacters {
				if c.Equal(&oc) {
					shouldAdd = false
					break
				}
			}
			if shouldAdd {
				c.ID = 0
				c.ResourceID = r.ID
				// If ImageID is 0, set it to nil to avoid foreign key constraint error
				if c.ImageID != nil && *c.ImageID == 0 {
					c.ImageID = nil
				}
				if err := tx.Create(&c).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func DeleteResource(id uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var r model.Resource
		if err := tx.Where("id = ?", id).First(&r).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.NewNotFoundError("Resource not found")
			}
			return err
		}
		if err := tx.Model(&model.File{}).Where("resource_id = ?", id).Delete(&model.File{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", r.UserID).Update("resources_count", gorm.Expr("resources_count - ?", 1)).Error; err != nil {
			return err
		}
		if err := tx.Delete(&r).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Activity{}).Where("type = ? AND ref_id = ?", model.ActivityTypeNewResource, id).Delete(&model.Activity{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Activity{}).Where("type = ? AND ref_id = ?", model.ActivityTypeUpdateResource, id).Delete(&model.Activity{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func GetResourceByTag(tagID uint, page int, pageSize int, sort model.RSort) ([]model.Resource, int, error) {
	tag, err := GetTagByID(tagID)
	if err != nil {
		return nil, 0, err
	}

	if tag.AliasOf != nil {
		tag, err = GetTagByID(*tag.AliasOf)
		if err != nil {
			return nil, 0, err
		}
	}

	var tagIds []uint
	tagIds = append(tagIds, tag.ID)
	for _, alias := range tag.Aliases {
		tagIds = append(tagIds, alias.ID)
	}

	var resources []model.Resource
	var total int64
	order, where := resourceOrderAndWhere(sort)

	subQuery := db.Table("resource_tags").
		Select("resource_id").
		Where("tag_id IN ?", tagIds).
		Group("resource_id")

	countQuery := db.Model(&model.Resource{}).Where("id IN (?)", subQuery)
	if where != "" {
		countQuery = countQuery.Where(where)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := db.Where("id IN (?)", subQuery).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Preload("User").
		Preload("Images").
		Preload("Tags").
		Preload("Files").
		Order(order)
	if where != "" {
		query = query.Where(where)
	}
	if err := query.Find(&resources).Error; err != nil {
		return nil, 0, err
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	return resources, int(totalPages), nil
}

// CountResourcesByTag counts the number of resources associated with a specific tag.
func CountResourcesByTag(tagID uint) (int64, error) {
	tag, err := GetTagByID(tagID)
	if err != nil {
		return 0, err
	}
	if tag.AliasOf != nil {
		tag, err = GetTagByID(*tag.AliasOf)
		if err != nil {
			return 0, err
		}
	}
	var tagIds []uint
	tagIds = append(tagIds, tag.ID)
	for _, alias := range tag.Aliases {
		tagIds = append(tagIds, alias.ID)
	}
	var count int64
	subQuery := db.Table("resource_tags").
		Select("resource_id").
		Where("tag_id IN ?", tagIds).
		Group("resource_id")
	if err := db.Model(&model.Resource{}).
		Where("id IN (?)", subQuery).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func ExistsResource(id uint) (bool, error) {
	var r model.Resource
	if err := db.Model(&model.Resource{}).Where("id = ?", id).First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func GetResourcesByUsername(username string, page, pageSize int) ([]model.Resource, int, error) {
	var user model.User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, model.NewNotFoundError("User not found")
		}
		return nil, 0, err
	}
	var resources []model.Resource
	var total int64

	if err := db.Model(&model.Resource{}).Where("user_id = ?", user.ID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Model(&model.Resource{}).Where("user_id = ?", user.ID).Offset((page - 1) * pageSize).Limit(pageSize).Preload("User").Preload("Images").Preload("Tags").Order("created_at DESC, id DESC").Find(&resources).Error; err != nil {
		return nil, 0, err
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	return resources, int(totalPages), nil
}

// GetAllResources retrieves all resources from the database without all related data.
// It is used to generate a sitemap and rss feed.
func GetAllResources() ([]model.Resource, error) {
	var resources []model.Resource
	if err := db.Order("id DESC").Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

func GetAllResourcesStats(page, pageSize int, sort string) ([]model.ResourceStatsView, int, error) {
	var total int64
	if err := db.Model(&model.Resource{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderBy := "resources.id DESC"
	switch sort {
	case "views_asc":
		orderBy = "resources.views ASC, resources.id ASC"
	case "views_desc":
		orderBy = "resources.views DESC, resources.id DESC"
	case "downloads_asc":
		orderBy = "resources.downloads ASC, resources.id ASC"
	case "downloads_desc":
		orderBy = "resources.downloads DESC, resources.id DESC"
	}

	var stats []model.ResourceStatsView
	offset := (page - 1) * pageSize
	if err := db.Table("resources").
		Select("resources.id, resources.title, resources.views, resources.downloads, COUNT(files.id) AS file_count").
		Joins("LEFT JOIN files ON files.resource_id = resources.id").
		Group("resources.id, resources.title").
		Order(orderBy).
		Offset(offset).
		Limit(pageSize).
		Scan(&stats).Error; err != nil {
		return nil, 0, err
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)
	return stats, int(totalPages), nil
}

type CachedResourceStats struct {
	id        uint
	views     atomic.Int64
	downloads atomic.Int64
}

var (
	cachedResourcesStats = make(map[uint]*CachedResourceStats)
	cacheMutex           = sync.RWMutex{}
)

func init() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			cacheMutex.Lock()
			if len(cachedResourcesStats) == 0 {
				cacheMutex.Unlock()
				continue
			}

			err := db.Transaction(func(tx *gorm.DB) error {
				for id, stats := range cachedResourcesStats {
					var count int64
					if err := tx.Model(&model.Resource{}).Where("id = ?", id).Count(&count).Error; err != nil {
						if errors.Is(err, gorm.ErrRecordNotFound) {
							log.Warnf("Resource with ID %d not found, skipping stats update", id)
							continue
						}
						return err
					}
					if count == 0 {
						continue
					}

					if views := stats.views.Swap(0); views > 0 {
						if err := tx.Model(&model.Resource{}).Where("id = ?", id).Update("views", gorm.Expr("views + ?", views)).Error; err != nil {
							return err
						}
					}
					if downloads := stats.downloads.Swap(0); downloads > 0 {
						if err := tx.Model(&model.Resource{}).Where("id = ?", id).Update("downloads", gorm.Expr("downloads + ?", downloads)).Error; err != nil {
							return err
						}
					}
				}
				return nil
			})
			if err != nil {
				log.Error("Failed to update resource stats cache: ", err)
			}
			clear(cachedResourcesStats)
			cacheMutex.Unlock()
		}
	}()
}

func AddResourceViewCount(id uint) error {
	cacheMutex.RLock()
	stats, exists := cachedResourcesStats[id]
	cacheMutex.RUnlock()

	if !exists {
		cacheMutex.Lock()
		stats, exists = cachedResourcesStats[id]
		if !exists {
			stats = &CachedResourceStats{id: id}
			cachedResourcesStats[id] = stats
		}
		cacheMutex.Unlock()
	}

	stats.views.Add(1)
	return nil
}

func AddResourceDownloadCount(id uint) error {
	cacheMutex.RLock()
	stats, exists := cachedResourcesStats[id]
	cacheMutex.RUnlock()

	if !exists {
		cacheMutex.Lock()
		stats, exists = cachedResourcesStats[id]
		if !exists {
			stats = &CachedResourceStats{id: id}
			cachedResourcesStats[id] = stats
		}
		cacheMutex.Unlock()
	}

	stats.downloads.Add(1)
	return nil
}

func RandomResource() (model.Resource, error) {
	var maxID int64
	var tmpMaxID *int64
	if err := db.Model(&model.Resource{}).Select("MAX(id)").Scan(&tmpMaxID).Error; err != nil {
		return model.Resource{}, err
	}
	if tmpMaxID == nil {
		return model.Resource{}, model.NewRequestError("No resources found")
	}
	maxID = *tmpMaxID
	for {
		randomID := uint(1)
		if maxID > 1 {
			randomID = uint(1 + rand.Int63n(maxID-1))
		}
		var resource model.Resource
		if err := db.
			Preload("User").
			Preload("Images").
			Preload("Tags").
			Preload("Files").
			Where("id = ?", randomID).
			First(&resource).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue // Try again if the resource does not exist
			}
			return model.Resource{}, err // Return error if any other issue occurs
		}
		return resource, nil // Return the found resource
	}
}

func tagIDsIncludingAliases(tagID uint) ([]uint, error) {
	tag, err := GetTagByID(tagID)
	if err != nil {
		return nil, err
	}
	if tag.AliasOf != nil {
		tag, err = GetTagByID(*tag.AliasOf)
		if err != nil {
			return nil, err
		}
	}
	tagIDs := make([]uint, 0, 1+len(tag.Aliases))
	tagIDs = append(tagIDs, tag.ID)
	for _, alias := range tag.Aliases {
		tagIDs = append(tagIDs, alias.ID)
	}
	return tagIDs, nil
}

func GetResourcesIdWithTag(tagID uint) ([]uint, error) {
	return getResourceIDsWithTag(tagID, 10000)
}

func GetAllResourceIDsWithTag(tagID uint) ([]uint, error) {
	return getResourceIDsWithTag(tagID, 0)
}

func getResourceIDsWithTag(tagID uint, limit int) ([]uint, error) {
	tagIDs, err := tagIDsIncludingAliases(tagID)
	if err != nil {
		return nil, err
	}
	var result []model.Resource
	subQuery := db.Table("resource_tags").
		Select("resource_id").
		Where("tag_id IN ?", tagIDs).
		Group("resource_id")
	query := db.Model(&model.Resource{}).
		Where("id IN (?)", subQuery).
		Order("created_at DESC").
		Select("id", "created_at")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&result).Error; err != nil {
		return nil, err
	}

	ids := make([]uint, len(result))
	for i, r := range result {
		ids[i] = r.ID
	}
	return ids, nil
}

func FilterResourceIDsByTag(ids []uint, tagID uint) ([]uint, error) {
	if len(ids) == 0 {
		return []uint{}, nil
	}
	tagIDs, err := tagIDsIncludingAliases(tagID)
	if err != nil {
		return nil, err
	}
	var matched []uint
	if err := db.Table("resource_tags").
		Where("tag_id IN ? AND resource_id IN ?", tagIDs, ids).
		Distinct("resource_id").
		Pluck("resource_id", &matched).Error; err != nil {
		return nil, err
	}
	return utils.IntersectPreserveOrder(ids, matched), nil
}

func applyReleaseDateFilter(query *gorm.DB, from, to *time.Time) *gorm.DB {
	query = query.Where("release_date IS NOT NULL")
	if from != nil {
		query = query.Where("release_date >= ?", *from)
	}
	if to != nil {
		query = query.Where("release_date < ?", to.AddDate(0, 0, 1))
	}
	return query
}

func FilterResourceIDsByReleaseDate(ids []uint, from, to *time.Time) ([]uint, error) {
	if len(ids) == 0 {
		return []uint{}, nil
	}
	query := applyReleaseDateFilter(db.Model(&model.Resource{}).Where("id IN ?", ids), from, to)
	var matched []uint
	if err := query.Pluck("id", &matched).Error; err != nil {
		return nil, err
	}
	return utils.IntersectPreserveOrder(ids, matched), nil
}

func GetResourceIDsByReleaseDate(from, to *time.Time) ([]uint, error) {
	query := applyReleaseDateFilter(db.Model(&model.Resource{}), from, to).
		Order("release_date DESC, id DESC")
	var ids []uint
	if err := query.Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []uint{}
	}
	return ids, nil
}

func BatchGetResources(ids []uint) ([]model.Resource, error) {
	var resources []model.Resource

	for _, id := range ids {
		var r model.Resource
		if err := db.
			Preload("User").
			Preload("Images").
			Preload("Tags").
			First(&r, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		for i, tag := range r.Tags {
			if tag.AliasOf != nil {
				t, err := GetTagByID(*tag.AliasOf)
				if err != nil {
					return nil, err
				} else {
					r.Tags[i].Type = t.Type
				}
			}
		}
		resources = append(resources, r)
	}

	return resources, nil
}

func CountResources() (int64, error) {
	var count int64
	if err := db.Model(&model.Resource{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// UpdateCharacterImage 更新角色的图片ID
func UpdateCharacterImage(characterID, imageID uint) error {
	var updateValue interface{}
	if imageID == 0 {
		updateValue = nil
	} else {
		updateValue = imageID
	}

	result := db.Model(&model.Character{}).Where("id = ?", characterID).Update("image_id", updateValue)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return model.NewNotFoundError("Character not found")
	}
	return nil
}

// GetLowResolutionCharacters 获取低清晰度的角色图片
// maxWidth和maxHeight定义了低清晰度的阈值
func GetLowResolutionCharacters(maxWidth, maxHeight int, limit int, offset int) ([]model.LowResCharacterView, error) {
	var results []model.LowResCharacterView

	query := `
		SELECT 
			c.id as character_id,
			c.resource_id as resource_id,
			c.name as name,
			i.id as image_id,
			i.width as image_width,
			i.height as image_height
		FROM characters c
		INNER JOIN images i ON c.image_id = i.id
		WHERE (i.width <= ? OR i.height <= ?)
		AND c.image_id IS NOT NULL
		ORDER BY c.id
		LIMIT ? OFFSET ?
	`

	err := db.Raw(query, maxWidth, maxHeight, limit, offset).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	return results, nil
}

// GetLowResolutionCharactersCount 获取低清晰度角色的总数
func GetLowResolutionCharactersCount(maxWidth, maxHeight int) (int64, error) {
	var count int64

	query := `
		SELECT COUNT(*)
		FROM characters c
		INNER JOIN images i ON c.image_id = i.id
		WHERE (i.width <= ? OR i.height <= ?)
		AND c.image_id IS NOT NULL
	`

	err := db.Raw(query, maxWidth, maxHeight).Scan(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetLowResolutionResourceImages 获取低清晰度的资源图片
// maxWidth和maxHeight定义了低清晰度的阈值
func GetLowResolutionResourceImages(maxWidth, maxHeight int, limit int, offset int) ([]model.LowResResourceImageView, error) {
	var results []model.LowResResourceImageView

	query := `
		SELECT DISTINCT
			r.id as resource_id,
			r.title as title,
			i.id as image_id,
			i.width as image_width,
			i.height as image_height
		FROM resources r
		INNER JOIN resource_images ri ON r.id = ri.resource_id
		INNER JOIN images i ON ri.image_id = i.id
		WHERE (i.width <= ? OR i.height <= ?)
		ORDER BY r.id, i.id
		LIMIT ? OFFSET ?
	`

	err := db.Raw(query, maxWidth, maxHeight, limit, offset).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	return results, nil
}

// GetLowResolutionResourceImagesCount 获取低清晰度资源图片的总数
func GetLowResolutionResourceImagesCount(maxWidth, maxHeight int) (int64, error) {
	var count int64

	query := `
		SELECT COUNT(DISTINCT ri.resource_id, ri.image_id)
		FROM resources r
		INNER JOIN resource_images ri ON r.id = ri.resource_id
		INNER JOIN images i ON ri.image_id = i.id
		WHERE (i.width <= ? OR i.height <= ?)
	`

	err := db.Raw(query, maxWidth, maxHeight).Scan(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

// UpdateResourceImage 更新资源中特定的图片ID
func UpdateResourceImage(resourceID, oldImageID, newImageID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 首先检查关联是否存在
		var exists bool
		err := tx.Raw("SELECT EXISTS(SELECT 1 FROM resource_images WHERE resource_id = ? AND image_id = ?)",
			resourceID, oldImageID).Scan(&exists).Error
		if err != nil {
			return err
		}

		if !exists {
			return fmt.Errorf("resource %d does not have image %d", resourceID, oldImageID)
		}

		// 更新resource_images表中的image_id
		result := tx.Exec("UPDATE resource_images SET image_id = ? WHERE resource_id = ? AND image_id = ?",
			newImageID, resourceID, oldImageID)
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return fmt.Errorf("no resource image association updated")
		}

		// 更新资源描述中的图片引用
		var resource model.Resource
		if err := tx.Select("article").Where("id = ?", resourceID).First(&resource).Error; err != nil {
			return err
		}

		// 替换描述中的图片引用
		oldImageRef := fmt.Sprintf("/api/image/%d)", oldImageID)
		newImageRef := fmt.Sprintf("/api/image/%d)", newImageID)

		// 使用字符串替换更新文章内容
		updatedArticle := strings.ReplaceAll(resource.Article, oldImageRef, newImageRef)

		// 如果文章内容有变化，更新数据库
		if updatedArticle != resource.Article {
			if err := tx.Model(&model.Resource{}).Where("id = ?", resourceID).Update("article", updatedArticle).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func GetResourceOwnerID(resourceID uint) (uint, error) {
	var uid uint
	if err := db.Model(&model.Resource{}).Select("user_id").Where("id = ?", resourceID).First(&uid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, model.NewNotFoundError("Resource not found")
		}
		return 0, err
	}
	return uid, nil
}

func UpdateResourceReleaseDate(resourceID uint, releaseDate time.Time) error {
	result := db.Model(&model.Resource{}).Where("id = ?", resourceID).Update("release_date", releaseDate)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return model.NewNotFoundError("Resource not found")
	}
	return nil
}

// CountResourcesByUserID counts the number of resources created by a user
func CountResourcesByUserID(userID uint) (int64, error) {
	var count int64
	if err := db.Model(&model.Resource{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetResourceByVNID retrieves a resource by its VNDB ID.
// **Low performance**: Only used for admin to quickly find the resource when linking with VNDB.
func GetResourceByVNID(vnID string) (model.Resource, error) {
	vndbLink := fmt.Sprintf("https://vndb.org/%s", vnID)
	var r model.Resource
	if err := db.Preload("User").
		Preload("Images").
		Preload("Tags", func(db *gorm.DB) *gorm.DB {
			return db.Order("name ASC")
		}).
		Where("links LIKE ?", "%"+vndbLink+"%").
		First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Resource{}, model.NewNotFoundError("Resource not found")
		}
		return model.Resource{}, err
	}
	return r, nil
}
