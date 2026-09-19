package model

import "time"

type TagTemplate struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	UserID    uint   `gorm:"not null;uniqueIndex:idx_tag_template_user_name"`
	User      User   `gorm:"foreignKey:UserID;references:ID"`
	Name      string `gorm:"not null;uniqueIndex:idx_tag_template_user_name"`
	Content   string `gorm:"not null;type:text"`
}

type TagTemplateView struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Params    []string  `json:"params"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
