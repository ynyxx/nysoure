package dao

import (
	"errors"
	"nysoure/server/model"

	"gorm.io/gorm"
)

func CreateTagTemplate(uid uint, name, content string) (model.TagTemplate, error) {
	t := model.TagTemplate{
		UserID:  uid,
		Name:    name,
		Content: content,
	}
	if err := db.Create(&t).Error; err != nil {
		return model.TagTemplate{}, err
	}
	return t, nil
}

func UpdateTagTemplate(id uint, name, content string) error {
	result := db.Model(&model.TagTemplate{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name":    name,
		"content": content,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return model.NewNotFoundError("Tag template not found")
	}
	return nil
}

func DeleteTagTemplate(id uint) error {
	result := db.Delete(&model.TagTemplate{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return model.NewNotFoundError("Tag template not found")
	}
	return nil
}

func GetTagTemplateByID(id uint) (model.TagTemplate, error) {
	var t model.TagTemplate
	if err := db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.TagTemplate{}, model.NewNotFoundError("Tag template not found")
		}
		return model.TagTemplate{}, err
	}
	return t, nil
}

func GetTagTemplateByUserAndName(uid uint, name string) (model.TagTemplate, error) {
	var t model.TagTemplate
	if err := db.Where("user_id = ? AND name = ?", uid, name).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.TagTemplate{}, model.NewNotFoundError("Tag template not found")
		}
		return model.TagTemplate{}, err
	}
	return t, nil
}

func ListTagTemplatesByUser(uid uint) ([]model.TagTemplate, error) {
	var templates []model.TagTemplate
	if err := db.Where("user_id = ?", uid).Order("updated_at DESC").Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

func CountTagTemplatesByUser(uid uint) (int64, error) {
	var count int64
	if err := db.Model(&model.TagTemplate{}).Where("user_id = ?", uid).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
