package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type Resource struct {
	gorm.Model
	Title             string
	AlternativeTitles []string `gorm:"serializer:json"`
	Links             []Link   `gorm:"serializer:json"`
	ReleaseDate       *time.Time
	Article           string
	Images            []Image `gorm:"many2many:resource_images;"`
	CoverID           *uint
	Tags              []Tag  `gorm:"many2many:resource_tags;"`
	Files             []File `gorm:"foreignKey:ResourceID"`
	UserID            uint
	User              User
	Views             uint
	Downloads         uint
	Comments          uint
	ModifiedTime      time.Time
	Gallery           []uint      `gorm:"serializer:json"`
	GalleryNsfw       []uint      `gorm:"serializer:json"`
	Characters        []Character `gorm:"foreignKey:ResourceID"`
}

type Link struct {
	URL   string `json:"url"`
	Label string `json:"label"`
}

type ResourceView struct {
	ID          uint       `json:"id"`
	Title       string     `json:"title"`
	Subtitle    string     `json:"subtitle,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ReleaseDate *time.Time `json:"release_date,omitempty"`
	Tags        []TagView  `json:"tags"`
	Image       *ImageView `json:"image"`
	Author      UserView   `json:"author"`
}

type ResourceStatsView struct {
	ID        uint   `json:"id"`
	Title     string `json:"title"`
	Views     uint   `json:"views"`
	Downloads uint   `json:"downloads"`
	FileCount int    `json:"file_count"`
}

type ResourceDetailView struct {
	ID                uint            `json:"id"`
	Title             string          `json:"title"`
	AlternativeTitles []string        `json:"alternativeTitles"`
	Links             []Link          `json:"links"`
	Article           string          `json:"article"`
	CreatedAt         time.Time       `json:"createdAt"`
	ReleaseDate       *time.Time      `json:"releaseDate,omitempty"`
	Tags              []TagView       `json:"tags"`
	Images            []ImageView     `json:"images"`
	CoverID           *uint           `json:"coverId,omitempty"`
	Files             []FileView      `json:"files"`
	Author            UserView        `json:"author"`
	Views             uint            `json:"views"`
	Downloads         uint            `json:"downloads"`
	Comments          uint            `json:"comments"`
	Related           []ResourceView  `json:"related"`
	Relations         []RelationView  `json:"relations"`
	Gallery           []uint          `json:"gallery"`
	GalleryNsfw       []uint          `json:"galleryNsfw"`
	Characters        []CharacterView `json:"characters"`
	Ratings           map[string]int  `json:"ratings"`
}

type LowResResourceImageView struct {
	ResourceID  uint   `json:"resource_id"`
	Title       string `json:"title"`
	ImageID     uint   `json:"image_id"`
	ImageWidth  int    `json:"image_width"`
	ImageHeight int    `json:"image_height"`
}

func (r *Resource) ToView() ResourceView {
	tags := make([]TagView, len(r.Tags))
	for i, tag := range r.Tags {
		tags[i] = *tag.ToView()
	}

	var image *ImageView
	if r.CoverID != nil {
		// Use the cover image if specified
		for _, img := range r.Images {
			if img.ID == *r.CoverID {
				v := img.ToView()
				image = &v
				break
			}
		}
	}
	// If no cover is set or cover image not found, use the first image
	if image == nil && len(r.Images) > 0 {
		v := r.Images[0].ToView()
		image = &v
	}

	return ResourceView{
		ID:          r.ID,
		Title:       r.Title,
		Subtitle:    firstNonEmpty(r.AlternativeTitles),
		CreatedAt:   r.CreatedAt,
		ReleaseDate: r.ReleaseDate,
		Tags:        tags,
		Image:       image,
		Author:      r.User.ToView(),
	}
}

func firstNonEmpty(ss []string) string {
	for _, s := range ss {
		if t := strings.TrimSpace(s); t != "" {
			return t
		}
	}
	return ""
}

func (r *Resource) ToDetailView() ResourceDetailView {
	images := make([]ImageView, len(r.Images))
	for i, image := range r.Images {
		images[i] = image.ToView()
	}
	tags := make([]TagView, len(r.Tags))
	for i, tag := range r.Tags {
		tags[i] = *tag.ToView()
	}

	files := make([]FileView, len(r.Files))
	for i, file := range r.Files {
		files[i] = *file.ToView()
	}
	characters := make([]CharacterView, len(r.Characters))
	for i, character := range r.Characters {
		characters[i] = *character.ToView()
	}
	return ResourceDetailView{
		ID:                r.ID,
		Title:             r.Title,
		AlternativeTitles: r.AlternativeTitles,
		Links:             r.Links,
		Article:           r.Article,
		CreatedAt:         r.CreatedAt,
		ReleaseDate:       r.ReleaseDate,
		Tags:              tags,
		Images:            images,
		CoverID:           r.CoverID,
		Files:             files,
		Author:            r.User.ToView(),
		Views:             r.Views,
		Downloads:         r.Downloads,
		Comments:          r.Comments,
		Gallery:           r.Gallery,
		GalleryNsfw:       r.GalleryNsfw,
		Characters:        characters,
	}
}
