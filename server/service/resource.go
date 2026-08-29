package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"nysoure/server/cache"
	"nysoure/server/config"
	"nysoure/server/ctx"
	"nysoure/server/dao"
	"nysoure/server/model"
	"nysoure/server/search"
	"nysoure/server/utils"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/log"

	"gorm.io/gorm"
)

const (
	maxSearchQueryLength = 100
	maxSearchTagCount    = 20
)

type ResourceSearchParams struct {
	Keyword     string
	Tags        []string
	ReleaseFrom *time.Time
	ReleaseTo   *time.Time
	Page        int
}

type RelationParam struct {
	ToID        uint   `json:"to_id"`
	Description string `json:"description"`
}

type ResourceParams struct {
	Title             string            `json:"title" binding:"required"`
	AlternativeTitles []string          `json:"alternative_titles"`
	Links             []model.Link      `json:"links"`
	ReleaseDate       string            `json:"release_date"`
	SkipUpdateTime    bool              `json:"skip_update_time"`
	Tags              []uint            `json:"tags"`
	Article           string            `json:"article"`
	Images            []uint            `json:"images"`
	CoverID           *uint             `json:"cover_id"`
	Gallery           []uint            `json:"gallery"`
	GalleryNsfw       []uint            `json:"gallery_nsfw"`
	Characters        []CharacterParams `json:"characters"`
	Relations         []RelationParam   `json:"relations"`
}

type CharacterParams struct {
	Name  string   `json:"name" binding:"required"`
	Alias []string `json:"alias"`
	CV    string   `json:"cv"`
	Role  string   `json:"role"`
	Image uint     `json:"image"`
}

var allowedResourceUpdateFields = map[string]struct{}{
	"title":              {},
	"alternative_titles": {},
	"links":              {},
	"release_date":       {},
	"tags":               {},
	"article":            {},
	"images":             {},
	"cover_id":           {},
	"gallery":            {},
	"gallery_nsfw":       {},
	"characters":         {},
	"relations":          {},
}

func normalizeResourceUpdateFields(updateFields []string) (map[string]struct{}, error) {
	if len(updateFields) == 0 {
		return nil, nil
	}

	fieldSet := make(map[string]struct{}, len(updateFields))
	for _, field := range updateFields {
		normalized := strings.TrimSpace(field)
		if normalized == "" {
			continue
		}
		if _, ok := allowedResourceUpdateFields[normalized]; !ok {
			return nil, model.NewRequestError("Invalid update field: " + normalized)
		}
		fieldSet[normalized] = struct{}{}
	}

	if len(fieldSet) == 0 {
		return nil, model.NewRequestError("update_fields cannot be empty")
	}

	return fieldSet, nil
}

func shouldUpdateResourceField(fieldSet map[string]struct{}, field string) bool {
	if fieldSet == nil {
		return true
	}
	_, ok := fieldSet[field]
	return ok
}

func imageIDsFromModels(images []model.Image) []uint {
	ids := make([]uint, len(images))
	for i, image := range images {
		ids[i] = image.ID
	}
	return ids
}

func filterResourceImageRefs(ids []uint, allowed []uint) []uint {
	filtered := make([]uint, 0, len(ids))
	for _, id := range ids {
		if slices.Contains(allowed, id) {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

func normalizeResourceCoverID(coverID *uint, imageIDs []uint, strict bool) (*uint, error) {
	if coverID == nil || *coverID == 0 {
		return nil, nil
	}
	if !slices.Contains(imageIDs, *coverID) {
		if strict {
			return nil, model.NewRequestError("Cover ID must be one of the resource images")
		}
		return nil, nil
	}
	return coverID, nil
}

func CreateResource(c ctx.Context, params *ResourceParams) (uint, error) {
	if c.UserPermission() < model.PermissionUploader {
		return 0, model.NewUnAuthorizedError("You have not permission to upload resources")
	}
	uid := c.MustUserID()

	images := make([]model.Image, len(params.Images))
	for i, id := range params.Images {
		images[i] = model.Image{
			Model: gorm.Model{
				ID: id,
			},
		}
	}
	tags := make([]model.Tag, len(params.Tags))
	for i, id := range params.Tags {
		tags[i] = model.Tag{
			Model: gorm.Model{
				ID: id,
			},
		}
	}
	gallery := make([]uint, 0, len(params.Gallery))
	for _, id := range params.Gallery {
		if slices.Contains(params.Images, id) {
			gallery = append(gallery, id)
		}
	}
	nsfw := make([]uint, 0, len(params.GalleryNsfw))
	for _, id := range params.GalleryNsfw {
		if slices.Contains(gallery, id) {
			nsfw = append(nsfw, id)
		}
	}
	characters := make([]model.Character, len(params.Characters))
	for i, c := range params.Characters {
		role := c.Role
		if role == "" {
			role = "primary"
		}
		var imageID *uint
		if c.Image != 0 {
			imageID = &c.Image
		}
		characters[i] = model.Character{
			Name:    c.Name,
			Alias:   c.Alias,
			CV:      c.CV,
			Role:    role,
			ImageID: imageID,
		}
	}
	var date *time.Time
	if params.ReleaseDate != "" {
		parsedDate, err := time.Parse("2006-01-02", params.ReleaseDate)
		if err != nil {
			return 0, model.NewRequestError("Invalid release date format, expected YYYY-MM-DD")
		}
		date = &parsedDate
	}
	// Validate CoverID if provided
	var coverID *uint
	if params.CoverID != nil && *params.CoverID != 0 {
		if !slices.Contains(params.Images, *params.CoverID) {
			return 0, model.NewRequestError("Cover ID must be one of the resource images")
		}
		coverID = params.CoverID
	}
	r := model.Resource{
		Title:             params.Title,
		AlternativeTitles: params.AlternativeTitles,
		Article:           params.Article,
		Links:             params.Links,
		ReleaseDate:       date,
		Images:            images,
		CoverID:           coverID,
		Tags:              tags,
		UserID:            uid,
		Gallery:           gallery,
		GalleryNsfw:       nsfw,
		Characters:        characters,
	}
	var err error
	if r, err = dao.CreateResource(r); err != nil {
		return 0, err
	}
	relations := make([]model.Relation, 0, len(params.Relations))
	for _, rel := range params.Relations {
		if rel.ToID == 0 || rel.ToID == r.ID {
			continue
		}
		relations = append(relations, model.Relation{
			FromID:      int64(r.ID),
			ToID:        int64(rel.ToID),
			Description: rel.Description,
		})
	}
	if err := dao.ReplaceRelations(r.ID, relations); err != nil {
		log.Error("ReplaceRelations error: ", err)
	}
	err = updateCachedTagList()
	if err != nil {
		log.Error("Error updating cached tag list:", err)
	}
	err = dao.AddNewResourceActivity(uid, r.ID)
	if err != nil {
		log.Error("AddNewResourceActivity error: ", err)
	}
	if err := search.AddResourceToIndex(r); err != nil {
		log.Error("AddResourceToIndex error: ", err)
	}
	return r.ID, nil
}

func findRelatedResources(r model.Resource, host string) []model.ResourceView {
	lines := strings.Split(r.Article, "\n")
	var relatedResources []model.ResourceView
	for _, line := range lines {
		r := parseResourceIfPresent(line, host)
		if r != nil {
			relatedResources = append(relatedResources, *r)
		}
	}
	return relatedResources
}

func parseResourceIfPresent(line string, host string) *model.ResourceView {
	if len(line) < 4 {
		return nil
	}
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, ")") {
		return nil
	}
	if !strings.Contains(line, "](") {
		return nil
	}
	splites := strings.Split(line, "(")
	if len(splites) != 2 {
		return nil
	}
	u := strings.TrimSuffix(splites[1], ")")
	u = strings.TrimSpace(u)
	parsed, err := url.Parse(u)
	if err != nil {
		return nil
	}
	if parsed.IsAbs() && parsed.Hostname() != host {
		return nil
	}
	path := parsed.Path
	if !strings.HasPrefix(path, "/resources/") {
		return nil
	}
	idStr := strings.TrimPrefix(path, "/resources/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil
	}
	r, err := dao.GetResourceByID(uint(id))
	if err != nil {
		return nil
	}
	v := r.ToView()
	return &v
}

func GetResource(id uint, c ctx.Context) (*model.ResourceDetailView, error) {
	r, err := dao.GetResourceByID(id)
	if err != nil {
		return nil, err
	}
	v := r.ToDetailView()
	if c.Host() != "" {
		related := findRelatedResources(r, c.Host())
		v.Related = related
	}
	rawRelations, relErr := dao.GetRelations(id)
	if relErr != nil {
		log.Error("GetRelations error: ", relErr)
	} else {
		relationViews := make([]model.RelationView, 0, len(rawRelations))
		for _, rel := range rawRelations {
			related, relErr := dao.GetResourceByID(uint(rel.ToID))
			if relErr != nil {
				continue
			}
			relationViews = append(relationViews, model.RelationView{
				Resource:    related.ToView(),
				Description: rel.Description,
			})
		}
		v.Relations = relationViews
	}
	fillRatings(&v)

	removeNsfw := true
	if c.LoggedIn() {
		createdAt := c.UserCreatedAt()
		if createdAt.Before(time.Now().Add(-72 * time.Hour)) {
			removeNsfw = false
		}
	}
	if removeNsfw {
		removeNsfwImages(&v)
	}

	return &v, nil
}

func AddResourceView(id uint, c ctx.Context) error {
	if !c.IsRealUser() {
		return nil
	}
	if err := dao.AddResourceViewCount(id); err != nil {
		log.Error("AddResourceViewCount error: ", err)
		return model.NewInternalServerError("Failed to add resource view")
	}
	return nil
}

func GetResourceList(page int, sort model.RSort) ([]model.ResourceView, int, error) {
	resources, totalPages, err := dao.GetResourceList(page, pageSize, sort)
	if err != nil {
		return nil, 0, err
	}
	var views []model.ResourceView
	for _, r := range resources {
		views = append(views, r.ToView())
	}
	return views, totalPages, nil
}

func GetAllResourcesStats(c ctx.Context, page int, sort string) ([]model.ResourceStatsView, int, error) {
	if c.UserPermission() != model.PermissionAdmin {
		return nil, 0, model.NewUnAuthorizedError("You do not have permission to access this resource")
	}
	if page < 1 {
		page = 1
	}
	return dao.GetAllResourcesStats(page, 500, sort)
}

// splitQuery splits the input query string into keywords, treating quoted substrings (single or double quotes)
// as single keywords and supporting escape characters for quotes. Spaces outside quotes are used as separators.
func splitQuery(query string) []string {
	var keywords []string

	query = strings.TrimSpace(query)
	if query == "" {
		return keywords
	}

	l, r := 0, 0
	inQuote := false
	quoteChar := byte(0)

	for r < len(query) {
		if (query[r] == '"' || query[r] == '\'') && (r == 0 || query[r-1] != '\\') {
			if !inQuote {
				inQuote = true
				quoteChar = query[r]
				l = r + 1
			} else if query[r] == quoteChar {
				if r > l {
					keywords = append(keywords, strings.TrimSpace(query[l:r]))
				}
				inQuote = false
				r++
				l = r
				continue
			}
		} else if !inQuote && query[r] == ' ' {
			if r > l {
				keywords = append(keywords, strings.TrimSpace(query[l:r]))
			}
			for r < len(query) && query[r] == ' ' {
				r++
			}
			l = r
			continue
		}

		r++
	}

	if l < len(query) {
		keywords = append(keywords, strings.TrimSpace(query[l:r]))
	}

	return keywords
}

func searchWithKeyword(keyword string) ([]uint, error) {
	resources := make([]uint, 0)

	if len([]rune(keyword)) <= maxTagLength {
		exists, err := dao.ExistsTag(keyword)
		if err != nil {
			return nil, err
		}
		if exists {
			t, err := dao.GetTagByName(keyword)
			if err != nil {
				return nil, err
			}
			res, err := dao.GetResourcesIdWithTag(t.ID)
			if err != nil {
				return nil, err
			}
			resources = append(resources, res...)
		}
	}

	searchResult, err := search.SearchResource(keyword)
	if err != nil {
		return nil, err
	}

	resources = append(resources, searchResult...)

	return resources, nil
}

func SearchResource(query string, page int) ([]model.ResourceView, int, error) {
	return SearchResources(ResourceSearchParams{
		Keyword: query,
		Page:    page,
	})
}

func SearchResources(params ResourceSearchParams) ([]model.ResourceView, int, error) {
	keyword := strings.TrimSpace(params.Keyword)
	tags := make([]string, 0, len(params.Tags))
	for _, tag := range params.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	tags = utils.RemoveDuplicate(tags)

	if len([]rune(keyword)) > maxSearchQueryLength {
		return nil, 0, model.NewRequestError("Search query is too long")
	}
	if len(tags) > maxSearchTagCount {
		return nil, 0, model.NewRequestError("Too many tags")
	}
	if params.ReleaseFrom != nil && params.ReleaseTo != nil && params.ReleaseFrom.After(*params.ReleaseTo) {
		return nil, 0, model.NewRequestError("Invalid date range")
	}

	hasKeyword := keyword != ""
	hasTags := len(tags) > 0
	hasDate := params.ReleaseFrom != nil || params.ReleaseTo != nil
	if !hasKeyword && !hasTags && !hasDate {
		return nil, 0, model.NewRequestError("At least one search condition is required")
	}

	page := params.Page
	if page < 1 {
		page = 1
	}

	var ids []uint
	hasIDs := false

	if hasKeyword {
		keywordIDs, err := searchResourceIDsByKeyword(keyword)
		if err != nil {
			return nil, 0, err
		}
		ids = keywordIDs
		hasIDs = true
		if len(ids) == 0 {
			return []model.ResourceView{}, 0, nil
		}
	}

	for _, tagName := range tags {
		t, err := dao.GetTagByName(tagName)
		if err != nil {
			if model.IsNotFoundError(err) {
				return []model.ResourceView{}, 0, nil
			}
			return nil, 0, err
		}
		if hasIDs {
			ids, err = dao.FilterResourceIDsByTag(ids, t.ID)
		} else {
			ids, err = dao.GetAllResourceIDsWithTag(t.ID)
			hasIDs = true
		}
		if err != nil {
			return nil, 0, err
		}
		if len(ids) == 0 {
			return []model.ResourceView{}, 0, nil
		}
	}

	if hasDate {
		var err error
		if hasIDs {
			ids, err = dao.FilterResourceIDsByReleaseDate(ids, params.ReleaseFrom, params.ReleaseTo)
		} else {
			ids, err = dao.GetResourceIDsByReleaseDate(params.ReleaseFrom, params.ReleaseTo)
			hasIDs = true
		}
		if err != nil {
			return nil, 0, err
		}
		if len(ids) == 0 {
			return []model.ResourceView{}, 0, nil
		}
	}

	if !hasIDs {
		return []model.ResourceView{}, 0, nil
	}

	total := len(ids)
	totalPages := (total + pageSize - 1) / pageSize
	start := (page - 1) * pageSize
	if start >= total {
		return []model.ResourceView{}, totalPages, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	resourcesPage, err := dao.BatchGetResources(ids[start:end])
	if err != nil {
		return nil, 0, err
	}
	views := make([]model.ResourceView, 0, len(resourcesPage))
	for _, r := range resourcesPage {
		views = append(views, r.ToView())
	}
	return views, totalPages, nil
}

func searchResourceIDsByKeyword(query string) ([]uint, error) {
	resources := make([]uint, 0)

	checkTag := func(tag string) error {
		if len([]rune(tag)) > maxTagLength {
			return nil
		}
		exists, err := dao.ExistsTag(tag)
		if err != nil {
			return err
		}
		if exists {
			t, err := dao.GetTagByName(tag)
			if err != nil {
				return err
			}
			res, err := dao.GetResourcesIdWithTag(t.ID)
			if err != nil {
				return err
			}
			resources = append(resources, res...)
		}
		return nil
	}

	if err := checkTag(query); err != nil {
		return nil, err
	}

	trimmed := utils.RemoveSpaces(query)
	if trimmed != query {
		if err := checkTag(trimmed); err != nil {
			return nil, err
		}
	}

	keywords := splitQuery(query)
	var temp []uint
	haveTag := false
	for _, keyword := range keywords {
		if len([]rune(keyword)) <= maxTagLength {
			exists, err := dao.ExistsTag(keyword)
			if err != nil {
				return nil, err
			}
			if exists {
				haveTag = true
			}
		}
	}
	if haveTag {
		first := true
		for _, keyword := range keywords {
			if keyword == "" {
				continue
			}
			if utils.OnlyPunctuation(keyword) {
				continue
			}

			res, err := searchWithKeyword(keyword)
			if err != nil {
				return nil, err
			}
			if len(res) == 0 && search.IsStopWord(keyword) {
				continue
			}
			if first {
				temp = utils.RemoveDuplicate(res)
				first = false
			} else {
				temp = utils.IntersectPreserveOrder(temp, res)
			}
		}
	} else {
		res, err := searchWithKeyword(query)
		if err != nil {
			return nil, err
		}
		temp = res
	}
	resources = append(resources, temp...)
	return utils.RemoveDuplicate(resources), nil
}

func DeleteResource(c ctx.Context, id uint) error {
	uid := c.MustUserID()
	isAdmin := c.UserPermission() == model.PermissionAdmin
	if !isAdmin {
		r, err := dao.GetResourceByID(id)
		if err != nil {
			return err
		}
		if r.UserID != uid {
			return model.NewUnAuthorizedError("You have not permission to delete this resource")
		}
	}
	r, err := GetResource(id, c)
	if err != nil {
		return err
	}
	if len(r.Files) > 0 {
		return model.NewRequestError("This resource has files, please delete them first")
	}
	if err := dao.DeleteResource(id); err != nil {
		return err
	}
	err = updateCachedTagList()
	if err != nil {
		log.Error("Error updating cached tag list:", err)
	}
	if err := search.RemoveResourceFromIndex(id); err != nil {
		log.Error("RemoveResourceFromIndex error: ", err)
	}
	return nil
}

func GetResourcesWithTag(tag string, page int, sort model.RSort) ([]model.ResourceView, int, error) {
	t, err := dao.GetTagByName(tag)
	if err != nil {
		return nil, 0, err
	}
	tagID := t.ID
	resources, totalPages, err := dao.GetResourceByTag(tagID, page, pageSize, sort)
	if err != nil {
		return nil, 0, err
	}
	var views []model.ResourceView
	for _, r := range resources {
		views = append(views, r.ToView())
	}
	return views, totalPages, nil
}

func GetResourcesWithUser(username string, page int) ([]model.ResourceView, int, error) {
	resources, totalPages, err := dao.GetResourcesByUsername(username, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	var views []model.ResourceView
	for _, r := range resources {
		views = append(views, r.ToView())
	}
	return views, totalPages, nil
}

func UpdateResource(c ctx.Context, rid uint, params *ResourceParams, updateFields []string) error {
	uid := c.MustUserID()
	canUpload := c.UserPermission() >= model.PermissionUploader
	r, err := dao.GetResourceByID(rid)
	if err != nil {
		return err
	}
	if r.UserID != uid && !canUpload {
		return model.NewUnAuthorizedError("You have not permission to edit this resource")
	}
	fieldSet, err := normalizeResourceUpdateFields(updateFields)
	if err != nil {
		return err
	}
	updateTitle := shouldUpdateResourceField(fieldSet, "title")
	updateAlternativeTitles := shouldUpdateResourceField(fieldSet, "alternative_titles")
	updateLinks := shouldUpdateResourceField(fieldSet, "links")
	updateReleaseDate := shouldUpdateResourceField(fieldSet, "release_date")
	updateTags := shouldUpdateResourceField(fieldSet, "tags")
	updateArticle := shouldUpdateResourceField(fieldSet, "article")
	updateImages := shouldUpdateResourceField(fieldSet, "images")
	updateCover := shouldUpdateResourceField(fieldSet, "cover_id")
	updateGallery := shouldUpdateResourceField(fieldSet, "gallery")
	updateGalleryNsfw := shouldUpdateResourceField(fieldSet, "gallery_nsfw")
	updateCharacters := shouldUpdateResourceField(fieldSet, "characters")
	updateRelations := shouldUpdateResourceField(fieldSet, "relations")

	effectiveImageIDs := imageIDsFromModels(r.Images)
	if updateImages {
		effectiveImageIDs = append([]uint(nil), params.Images...)
	}

	effectiveGallerySource := r.Gallery
	if updateGallery {
		effectiveGallerySource = params.Gallery
	}
	gallery := filterResourceImageRefs(effectiveGallerySource, effectiveImageIDs)

	effectiveGalleryNsfwSource := r.GalleryNsfw
	if updateGalleryNsfw {
		effectiveGalleryNsfwSource = params.GalleryNsfw
	}
	nsfw := filterResourceImageRefs(effectiveGalleryNsfwSource, gallery)

	coverSource := r.CoverID
	if updateCover {
		coverSource = params.CoverID
	}
	coverID, err := normalizeResourceCoverID(coverSource, effectiveImageIDs, updateCover || fieldSet == nil)
	if err != nil {
		return err
	}

	characters := r.Characters
	if updateCharacters {
		characters = make([]model.Character, len(params.Characters))
		for i, c := range params.Characters {
			role := c.Role
			if role == "" {
				role = "primary"
			}
			var imageID *uint
			if c.Image != 0 {
				imageID = &c.Image
			}
			characters[i] = model.Character{
				Name:    c.Name,
				Alias:   c.Alias,
				CV:      c.CV,
				Role:    role,
				ImageID: imageID,
			}
		}
	}

	date := r.ReleaseDate
	if updateReleaseDate {
		date = nil
		if params.ReleaseDate != "" {
			parsedDate, err := time.Parse("2006-01-02", params.ReleaseDate)
			if err != nil {
				return model.NewRequestError("Invalid release date format, expected YYYY-MM-DD")
			}
			date = &parsedDate
		}
	}

	if updateTitle {
		r.Title = params.Title
	}
	if updateAlternativeTitles {
		r.AlternativeTitles = params.AlternativeTitles
	}
	if updateArticle {
		r.Article = params.Article
	}
	if updateLinks {
		r.Links = params.Links
	}
	if updateReleaseDate {
		r.ReleaseDate = date
	}
	if updateCover || updateImages {
		r.CoverID = coverID
	}
	if updateGallery || updateImages {
		r.Gallery = gallery
	}
	if updateGalleryNsfw || updateGallery || updateImages {
		r.GalleryNsfw = nsfw
	}
	if updateCharacters {
		r.Characters = characters
	}

	if updateImages {
		images := make([]model.Image, len(params.Images))
		for i, id := range params.Images {
			images[i] = model.Image{
				Model: gorm.Model{
					ID: id,
				},
			}
		}
		r.Images = images
	}
	if updateTags {
		tags := make([]model.Tag, len(params.Tags))
		for i, id := range params.Tags {
			tags[i] = model.Tag{
				Model: gorm.Model{
					ID: id,
				},
			}
		}
		r.Tags = tags
	}
	if err := dao.UpdateResource(r, params.SkipUpdateTime); err != nil {
		log.Error("UpdateResource error: ", err)
		return model.NewInternalServerError("Failed to update resource")
	}
	if updateRelations {
		relations := make([]model.Relation, 0, len(params.Relations))
		for _, rel := range params.Relations {
			if rel.ToID == 0 || rel.ToID == rid {
				continue
			}
			relations = append(relations, model.Relation{
				FromID:      int64(rid),
				ToID:        int64(rel.ToID),
				Description: rel.Description,
			})
		}
		if err := dao.ReplaceRelations(rid, relations); err != nil {
			log.Error("ReplaceRelations error: ", err)
		}
	}
	err = updateCachedTagList()
	if err != nil {
		log.Error("Error updating cached tag list:", err)
	}
	err = dao.AddUpdateResourceActivity(uid, r.ID)
	if err != nil {
		log.Error("AddUpdateResourceActivity error: ", err)
	}
	if err := search.AddResourceToIndex(r); err != nil {
		log.Error("AddResourceToIndex error: ", err)
	}
	return nil
}

func RandomResource(host string) (*model.ResourceDetailView, error) {
	r, err := dao.RandomResource()
	if err != nil {
		return nil, err
	}
	v := r.ToDetailView()
	if host != "" {
		related := findRelatedResources(r, host)
		v.Related = related
	}
	fillRatings(&v)
	return &v, nil
}

var lastSuccessCover uint

func RandomCover() (uint, error) {
	for retries := 0; retries < 5; retries++ {
		v, err := dao.RandomResource()
		if err != nil {
			return 0, err
		}
		if len(v.Gallery) > 0 {
			galleryWithoutNsfw := make([]uint, 0, len(v.Gallery))
			for _, id := range v.Gallery {
				if !slices.Contains(v.GalleryNsfw, id) {
					galleryWithoutNsfw = append(galleryWithoutNsfw, id)
				}
			}
			if len(galleryWithoutNsfw) > 0 {
				lastSuccessCover = galleryWithoutNsfw[rand.Intn(len(galleryWithoutNsfw))]
				return lastSuccessCover, nil
			}
		}
		if len(v.Images) > 0 {
			lastSuccessCover = v.Images[0].ID
			return v.Images[0].ID, nil
		}
	}
	if lastSuccessCover == 0 {
		return 0, model.NewNotFoundError("No cover found")
	}
	return lastSuccessCover, nil
}

func GetPinnedResources() ([]model.ResourceView, error) {
	ids := config.PinnedResources()
	var views []model.ResourceView
	for _, id := range ids {
		r, err := dao.GetResourceByID(id)
		if err != nil {
			continue
		}
		views = append(views, r.ToView())
	}
	return views, nil
}

// downloadAndCreateImage 下载图片并使用 CreateImage 保存
func downloadAndCreateImage(imageURL string) (uint, error) {
	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 下载图片
	resp, err := client.Get(imageURL)
	if err != nil {
		return 0, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to download image: HTTP %d", resp.StatusCode)
	}

	// 读取图片数据
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read image data: %w", err)
	}

	// 限制图片大小，防止内存溢出
	if len(imageData) > 8*1024*1024 { // 8MB 限制
		return 0, fmt.Errorf("image too large")
	}

	// 创建一个临时的fake context用于内部调用
	fakeCtx := ctx.NewFakeContext(1, model.PermissionUploader, time.Time{})
	imageID, err := CreateImage(fakeCtx, "127.0.0.1", imageData)
	if err != nil {
		return 0, fmt.Errorf("failed to create image: %w", err)
	}

	return imageID, nil
}

// UpdateCharacterImage 更新角色的图片ID
func UpdateCharacterImage(c ctx.Context, resourceID, characterID, imageID uint) error {
	// 检查资源是否存在并且用户有权限修改
	resource, err := dao.GetResourceByID(resourceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.NewNotFoundError("Resource not found")
		}
		return err
	}

	uid := c.MustUserID()
	isAdmin := c.UserPermission() == model.PermissionAdmin

	// 检查用户是否有权限修改这个资源
	if resource.UserID != uid && !isAdmin {
		return model.NewUnAuthorizedError("You don't have permission to modify this resource")
	}

	// 更新角色图片
	err = dao.UpdateCharacterImage(characterID, imageID)
	if err != nil {
		return err
	}

	return nil
}

// GetLowResolutionCharacters 获取低清晰度的角色图片
func GetLowResolutionCharacters(page int, pageSize int, maxWidth, maxHeight int) ([]model.LowResCharacterView, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50 // 默认每页50个角色
	}
	if pageSize > 1000 {
		pageSize = 1000 // 限制最大页面大小
	}

	offset := (page - 1) * pageSize

	// 获取角色列表
	characters, err := dao.GetLowResolutionCharacters(maxWidth, maxHeight, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	// 获取总数
	totalCount, err := dao.GetLowResolutionCharactersCount(maxWidth, maxHeight)
	if err != nil {
		return nil, 0, err
	}

	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))

	return characters, totalPages, nil
}

// GetLowResolutionResourceImages 获取低清晰度的资源图片
func GetLowResolutionResourceImages(page int, pageSize int, maxWidth, maxHeight int) ([]model.LowResResourceImageView, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50 // 默认每页50个图片
	}
	if pageSize > 1000 {
		pageSize = 1000 // 限制最大页面大小
	}

	offset := (page - 1) * pageSize

	// 获取资源图片列表
	images, err := dao.GetLowResolutionResourceImages(maxWidth, maxHeight, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	// 获取总数
	totalCount, err := dao.GetLowResolutionResourceImagesCount(maxWidth, maxHeight)
	if err != nil {
		return nil, 0, err
	}

	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))

	return images, totalPages, nil
}

// UpdateResourceImage 更新资源图片
func UpdateResourceImage(c ctx.Context, resourceID, oldImageID, newImageID uint) error {
	// 首先检查用户权限 - 确保用户是资源的所有者或管理员
	resource, err := dao.GetResourceByID(resourceID)
	if err != nil {
		return err
	}

	uid := c.MustUserID()
	isAdmin := c.UserPermission() == model.PermissionAdmin

	if resource.UserID != uid && !isAdmin {
		return model.NewUnAuthorizedError("You don't have permission to update this resource")
	}

	// 更新资源图片
	return dao.UpdateResourceImage(resourceID, oldImageID, newImageID)
}

func getSteamRating(steamID string) (int, error) {
	client := http.Client{}
	url := fmt.Sprintf("https://store.steampowered.com/appreviews/%s?json=1&language=all&purchase_type=all&cursor=*&num_per_page=0", steamID)
	resp, err := client.Get(url)
	if err != nil {
		return 0, model.NewInternalServerError("Failed to get Steam rating")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, model.NewInternalServerError("Failed to get Steam rating")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, model.NewInternalServerError("Failed to read Steam rating")
	}
	var steamResp struct {
		QuerySummary struct {
			TotalReviews  int `json:"total_reviews"`
			TotalPositive int `json:"total_positive"`
		} `json:"query_summary"`
	}
	if err := json.Unmarshal(body, &steamResp); err != nil {
		return 0, model.NewInternalServerError("Failed to parse Steam rating")
	}
	rating := int(math.Round(float64(steamResp.QuerySummary.TotalPositive) / float64(steamResp.QuerySummary.TotalReviews) * 100))
	return rating, nil
}

func getSteamRatingWithCache(steamID string) (int, error) {
	cacheKey := fmt.Sprintf("steam_rating_%s", steamID)
	ratingStr, err := cache.Get(cacheKey)
	if err != nil && !errors.Is(err, cache.ErrNotFound) {
		return 0, err
	} else if errors.Is(err, cache.ErrNotFound) {
		rating, err := getSteamRating(steamID)
		if err != nil {
			return 0, err
		}
		err = cache.Set(cacheKey, strconv.Itoa(rating), 24*time.Hour)
		if err != nil {
			log.Error("Failed to set Steam rating cache: ", err)
		}
		return rating, nil
	}
	rating, err := strconv.Atoi(ratingStr)
	if err != nil {
		return 0, model.NewInternalServerError("Failed to parse Steam rating")
	}
	return rating, nil
}

func fillRatings(resource *model.ResourceDetailView) {
	ratings := make(map[string]int)
	for _, link := range resource.Links {
		if link.Label == "" {
			continue
		}
		if vnID, ok := strings.CutPrefix(link.URL, "https://vndb.org/v"); ok {
			vnID = strings.TrimSpace(vnID)
			rating, err := getVNDBRatingWithCache(vnID)
			if err == nil {
				ratings[link.Label] = rating
			} else {
				log.Error("Failed to get VNDB rating: ", err)
			}
		} else if steamID, ok := strings.CutPrefix(link.URL, "https://store.steampowered.com/app/"); ok {
			steamID = strings.TrimSpace(steamID)
			if strings.Contains(steamID, "/") {
				steamID = strings.Split(steamID, "/")[0]
			}
			rating, err := getSteamRatingWithCache(steamID)
			if err == nil {
				ratings[link.Label] = rating
			} else {
				log.Error("Failed to get Steam rating: ", err)
			}
		}
	}
	resource.Ratings = ratings
}

func removeNsfwImages(r *model.ResourceDetailView) {
	if len(r.GalleryNsfw) == 0 || len(r.Gallery) < len(r.GalleryNsfw) || len(r.Images) < len(r.GalleryNsfw) {
		return
	}
	nsfwImageIDs := make(map[uint]struct{}, len(r.GalleryNsfw))
	for _, id := range r.GalleryNsfw {
		nsfwImageIDs[id] = struct{}{}
	}
	newGalleryIDs := make([]uint, 0, len(r.Gallery)-len(r.GalleryNsfw))
	for _, id := range r.Gallery {
		if _, ok := nsfwImageIDs[id]; ok {
			continue
		}
		newGalleryIDs = append(newGalleryIDs, id)
	}
	newImages := make([]model.ImageView, 0, len(r.Images)-len(r.GalleryNsfw))
	for _, i := range r.Images {
		if _, ok := nsfwImageIDs[i.ID]; ok {
			continue
		}
		newImages = append(newImages, i)
	}
	r.Images = newImages
	r.Gallery = newGalleryIDs
	r.GalleryNsfw = []uint{}
}
