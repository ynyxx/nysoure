package api

import (
	"encoding/json"
	"net/url"
	"nysoure/server/ctx"
	"nysoure/server/dao"
	"nysoure/server/model"
	"nysoure/server/service"
	"nysoure/server/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/log"

	"github.com/gofiber/fiber/v3"
)

type updateResourceRequest struct {
	service.ResourceParams
	UpdateFields []string `json:"update_fields"`
}

func updateSiteMapAndRss(baseURL string) {
	resources, err := dao.GetAllResources()
	if err != nil {
		log.Error("Error getting resources: ", err)
	}
	utils.GenerateSiteMap(baseURL, resources)
	utils.GenerateRss(baseURL, resources)
}

func handleCreateResource(c fiber.Ctx) error {
	var params service.ResourceParams
	body := c.Body()
	err := json.Unmarshal(body, &params)
	if err != nil {
		return model.NewRequestError("Invalid request body")
	}
	context := ctx.NewContext(c)
	id, err := service.CreateResource(context, &params)
	if err != nil {
		return err
	}
	updateSiteMapAndRss(c.BaseURL())
	return c.Status(fiber.StatusOK).JSON(model.Response[uint]{
		Success: true,
		Data:    id,
		Message: "Resource created successfully",
	})
}

func handleGetResource(c fiber.Ctx) error {
	idStr := c.Params("id")
	if idStr == "" {
		return model.NewRequestError("Resource ID is required")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return model.NewRequestError("Invalid resource ID")
	}
	resource, err := service.GetResource(uint(id), ctx.NewContext(c))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[model.ResourceDetailView]{
		Success: true,
		Data:    *resource,
		Message: "Resource retrieved successfully",
	})
}

func handleAddResourceView(c fiber.Ctx) error {
	idStr := c.Params("id")
	if idStr == "" {
		return model.NewRequestError("Resource ID is required")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return model.NewRequestError("Invalid resource ID")
	}
	err = service.AddResourceView(uint(id), ctx.NewContext(c))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[any]{
		Success: true,
		Data:    nil,
		Message: "Resource view added successfully",
	})
}

func handleDeleteResource(c fiber.Ctx) error {
	idStr := c.Params("id")
	if idStr == "" {
		return model.NewRequestError("Resource ID is required")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return model.NewRequestError("Invalid resource ID")
	}
	context := ctx.NewContext(c)
	err = service.DeleteResource(context, uint(id))
	if err != nil {
		return err
	}
	updateSiteMapAndRss(c.BaseURL())
	return c.Status(fiber.StatusOK).JSON(model.Response[any]{
		Success: true,
		Data:    nil,
		Message: "Resource deleted successfully",
	})
}

func handleListResources(c fiber.Ctx) error {
	pageStr := c.Query("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return model.NewRequestError("Invalid page number")
	}
	sortStr := c.Query("sort")
	if sortStr == "" {
		sortStr = "0"
	}
	sortInt, err := strconv.Atoi(sortStr)
	if err != nil {
		return model.NewRequestError("Invalid sort parameter")
	}
	if sortInt < 0 || sortInt > 7 {
		return model.NewRequestError("Sort parameter out of range")
	}
	sort := model.RSort(sortInt)
	resources, maxPage, err := service.GetResourceList(page, sort)
	if err != nil {
		return err
	}
	if resources == nil {
		resources = []model.ResourceView{}
	}
	return c.Status(fiber.StatusOK).JSON(model.PageResponse[model.ResourceView]{
		Success:    true,
		Data:       resources,
		TotalPages: maxPage,
		Message:    "Resources retrieved successfully",
	})
}

func handleListAllResourcesForAdmin(c fiber.Ctx) error {
	pageStr := c.Query("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return model.NewRequestError("Invalid page number")
	}
	sort := c.Query("sort", "")
	if sort != "" && sort != "views_asc" && sort != "views_desc" && sort != "downloads_asc" && sort != "downloads_desc" {
		return model.NewRequestError("Invalid sort parameter")
	}
	resources, maxPage, err := service.GetAllResourcesStats(ctx.NewContext(c), page, sort)
	if err != nil {
		return err
	}
	if resources == nil {
		resources = []model.ResourceStatsView{}
	}
	return c.Status(fiber.StatusOK).JSON(model.PageResponse[model.ResourceStatsView]{
		Success:    true,
		Data:       resources,
		TotalPages: maxPage,
		Message:    "Resources retrieved successfully",
	})
}

func handleListResourcesWithTag(c fiber.Ctx) error {
	tag := c.Params("tag")
	if tag == "" {
		return model.NewRequestError("Tag is required")
	}
	tag, err := url.PathUnescape(tag)
	if err != nil {
		return model.NewRequestError("Invalid tag")
	}
	pageStr := c.Query("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return model.NewRequestError("Invalid page number")
	}
	sortStr := c.Query("sort")
	if sortStr == "" {
		sortStr = "1"
	}
	sortInt, err := strconv.Atoi(sortStr)
	if err != nil {
		return model.NewRequestError("Invalid sort parameter")
	}
	if sortInt < 0 || sortInt > 7 {
		return model.NewRequestError("Sort parameter out of range")
	}
	sort := model.RSort(sortInt)
	resources, totalPages, err := service.GetResourcesWithTag(tag, page, sort)
	if err != nil {
		return err
	}
	if resources == nil {
		resources = []model.ResourceView{}
	}
	return c.Status(fiber.StatusOK).JSON(model.PageResponse[model.ResourceView]{
		Success:    true,
		Data:       resources,
		TotalPages: totalPages,
		Message:    "Resources retrieved successfully",
	})
}

func parseSearchDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.UTC)
	if err != nil {
		return nil, model.NewRequestError("Invalid date format, expected YYYY-MM-DD")
	}
	return &parsed, nil
}

func parseSearchTags(value string) []string {
	var tags []string
	for _, tag := range strings.Split(value, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return utils.RemoveDuplicate(tags)
}

func handleSearchResources(c fiber.Ctx) error {
	query := strings.TrimSpace(c.Query("keyword"))
	pageStr := c.Query("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return model.NewRequestError("Invalid page number")
	}
	releaseFrom, err := parseSearchDate(c.Query("release_from"))
	if err != nil {
		return err
	}
	releaseTo, err := parseSearchDate(c.Query("release_to"))
	if err != nil {
		return err
	}
	resources, totalPages, err := service.SearchResources(service.ResourceSearchParams{
		Keyword:     query,
		Tags:        parseSearchTags(c.Query("tags")),
		ReleaseFrom: releaseFrom,
		ReleaseTo:   releaseTo,
		Page:        page,
	})
	if err != nil {
		return err
	}
	if resources == nil {
		resources = []model.ResourceView{}
	}
	return c.Status(fiber.StatusOK).JSON(model.PageResponse[model.ResourceView]{
		Success:    true,
		Data:       resources,
		TotalPages: totalPages,
		Message:    "Resources retrieved successfully",
	})
}

func handleGetResourcesWithUser(c fiber.Ctx) error {
	username := c.Params("username")
	if username == "" {
		return model.NewRequestError("Username is required")
	}
	username, err := url.PathUnescape(username)
	if err != nil {
		return model.NewRequestError("Invalid username")
	}
	pageStr := c.Query("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return model.NewRequestError("Invalid page number")
	}
	resources, totalPages, err := service.GetResourcesWithUser(username, page)
	if err != nil {
		return err
	}
	if resources == nil {
		resources = []model.ResourceView{}
	}
	return c.Status(fiber.StatusOK).JSON(model.PageResponse[model.ResourceView]{
		Success:    true,
		Data:       resources,
		TotalPages: totalPages,
		Message:    "Resources retrieved successfully",
	})
}

func handleUpdateResource(c fiber.Ctx) error {
	idStr := c.Params("id")
	if idStr == "" {
		return model.NewRequestError("Resource ID is required")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return model.NewRequestError("Invalid resource ID")
	}
	var req updateResourceRequest
	body := c.Body()
	err = json.Unmarshal(body, &req)
	if err != nil {
		return model.NewRequestError("Invalid request body")
	}
	context := ctx.NewContext(c)
	err = service.UpdateResource(context, uint(id), &req.ResourceParams, req.UpdateFields)
	if err != nil {
		return err
	}
	updateSiteMapAndRss(c.BaseURL())
	return c.Status(fiber.StatusOK).JSON(model.Response[any]{
		Success: true,
		Data:    nil,
		Message: "Resource updated successfully",
	})
}

func handleGetRandomResource(c fiber.Ctx) error {
	host := c.Hostname()
	resource, err := service.RandomResource(host)
	if err != nil {
		return err
	}
	if resource == nil {
		return model.NewNotFoundError("No resources found")
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[model.ResourceDetailView]{
		Success: true,
		Data:    *resource,
		Message: "Random resource retrieved successfully",
	})
}

func handleGetPinnedResources(c fiber.Ctx) error {
	views, err := service.GetPinnedResources()
	if err != nil {
		return err
	}
	if views == nil {
		views = []model.ResourceView{}
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[[]model.ResourceView]{
		Success: true,
		Data:    views,
		Message: "Pinned resources retrieved successfully",
	})
}

func handleGetInfoFromVndb(c fiber.Ctx) error {
	vnID := c.Query("vnid")
	if vnID == "" {
		return model.NewRequestError("VNDB ID is required")
	}
	context := ctx.NewContext(c)
	characters, releaseDate, err := service.GetInfoFromVndb(vnID, context)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[map[string]any]{
		Success: true,
		Data: map[string]any{
			"characters":   characters,
			"release_date": releaseDate,
		},
		Message: "Characters retrieved successfully",
	})
}

func handleGetResourceByVNID(c fiber.Ctx) error {
	context := ctx.NewContext(c)
	if context.UserPermission() != model.PermissionAdmin {
		return model.NewUnAuthorizedError("Admin permission required")
	}
	vnID := c.Query("vnid")
	if vnID == "" {
		return model.NewRequestError("VNDB ID is required")
	}
	resource, err := dao.GetResourceByVNID(vnID)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[model.Resource]{
		Success: true,
		Data:    resource,
		Message: "Resource retrieved successfully",
	})
}

func handleGetResourcePrefillFromVndb(c fiber.Ctx) error {
	vnID := c.Query("vnid")
	if vnID == "" {
		return model.NewRequestError("VNDB ID is required")
	}

	sections := service.ParsePrefillSections(c.Query("sections"))
	prefill, err := service.GetResourceFormPrefillFromVNDB(vnID, ctx.NewContext(c), sections)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(model.Response[service.ResourceFormPrefill]{
		Success: true,
		Data:    *prefill,
		Message: "VNDB resource params retrieved successfully",
	})
}

func handleUpdateCharacterImage(c fiber.Ctx) error {
	resourceIdStr := c.Params("resourceId")
	characterIdStr := c.Params("characterId")
	if resourceIdStr == "" || characterIdStr == "" {
		return model.NewRequestError("Resource ID and Character ID are required")
	}
	resourceId, err := strconv.Atoi(resourceIdStr)
	if err != nil {
		return model.NewRequestError("Invalid resource ID")
	}
	characterId, err := strconv.Atoi(characterIdStr)
	if err != nil {
		return model.NewRequestError("Invalid character ID")
	}

	var params struct {
		ImageID uint `json:"image_id"`
	}
	body := c.Body()
	err = json.Unmarshal(body, &params)
	if err != nil {
		return model.NewRequestError("Invalid request body")
	}

	context := ctx.NewContext(c)
	err = service.UpdateCharacterImage(context, uint(resourceId), uint(characterId), params.ImageID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(model.Response[any]{
		Success: true,
		Data:    nil,
		Message: "Character image updated successfully",
	})
}

func handleGetLowResolutionCharacters(c fiber.Ctx) error {
	pageStr := c.Query("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return model.NewRequestError("Invalid page number")
	}

	// 支持自定义页面大小，默认50，最大1000
	pageSizeStr := c.Query("page_size")
	if pageSizeStr == "" {
		pageSizeStr = "50"
	}
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil {
		return model.NewRequestError("Invalid page_size parameter")
	}
	if pageSize > 1000 {
		pageSize = 1000 // 限制最大页面大小
	}
	if pageSize < 1 {
		pageSize = 1
	}

	maxWidthStr := c.Query("max_width")
	if maxWidthStr == "" {
		maxWidthStr = "800" // 默认最大宽度800px
	}
	maxWidth, err := strconv.Atoi(maxWidthStr)
	if err != nil {
		return model.NewRequestError("Invalid max_width parameter")
	}

	maxHeightStr := c.Query("max_height")
	if maxHeightStr == "" {
		maxHeightStr = "800" // 默认最大高度800px
	}
	maxHeight, err := strconv.Atoi(maxHeightStr)
	if err != nil {
		return model.NewRequestError("Invalid max_height parameter")
	}

	characters, totalPages, err := service.GetLowResolutionCharacters(page, pageSize, maxWidth, maxHeight)
	if err != nil {
		return err
	}

	if characters == nil {
		characters = []model.LowResCharacterView{}
	}

	return c.Status(fiber.StatusOK).JSON(model.PageResponse[model.LowResCharacterView]{
		Success:    true,
		Data:       characters,
		TotalPages: totalPages,
		Message:    "Low resolution characters retrieved successfully",
	})
}

func handleGetLowResolutionResourceImages(c fiber.Ctx) error {
	pageStr := c.Query("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return model.NewRequestError("Invalid page number")
	}

	// 支持自定义页面大小，默认50，最大1000
	pageSizeStr := c.Query("page_size")
	if pageSizeStr == "" {
		pageSizeStr = "50"
	}
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil {
		return model.NewRequestError("Invalid page_size parameter")
	}
	if pageSize > 1000 {
		pageSize = 1000 // 限制最大页面大小
	}
	if pageSize < 1 {
		pageSize = 1
	}

	maxWidthStr := c.Query("max_width")
	if maxWidthStr == "" {
		maxWidthStr = "800" // 默认最大宽度800px
	}
	maxWidth, err := strconv.Atoi(maxWidthStr)
	if err != nil {
		return model.NewRequestError("Invalid max_width parameter")
	}

	maxHeightStr := c.Query("max_height")
	if maxHeightStr == "" {
		maxHeightStr = "800" // 默认最大高度800px
	}
	maxHeight, err := strconv.Atoi(maxHeightStr)
	if err != nil {
		return model.NewRequestError("Invalid max_height parameter")
	}

	images, totalPages, err := service.GetLowResolutionResourceImages(page, pageSize, maxWidth, maxHeight)
	if err != nil {
		return err
	}

	if images == nil {
		images = []model.LowResResourceImageView{}
	}

	return c.Status(fiber.StatusOK).JSON(model.PageResponse[model.LowResResourceImageView]{
		Success:    true,
		Data:       images,
		TotalPages: totalPages,
		Message:    "Low resolution resource images retrieved successfully",
	})
}

func handleUpdateResourceImage(c fiber.Ctx) error {
	resourceIdStr := c.Params("resourceId")
	oldImageIdStr := c.Params("oldImageId")
	if resourceIdStr == "" || oldImageIdStr == "" {
		return model.NewRequestError("Resource ID and Old Image ID are required")
	}
	resourceId, err := strconv.Atoi(resourceIdStr)
	if err != nil {
		return model.NewRequestError("Invalid resource ID")
	}
	oldImageId, err := strconv.Atoi(oldImageIdStr)
	if err != nil {
		return model.NewRequestError("Invalid old image ID")
	}

	var params struct {
		NewImageID uint `json:"new_image_id"`
	}
	body := c.Body()
	err = json.Unmarshal(body, &params)
	if err != nil {
		return model.NewRequestError("Invalid request body")
	}

	context := ctx.NewContext(c)
	err = service.UpdateResourceImage(context, uint(resourceId), uint(oldImageId), params.NewImageID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(model.Response[any]{
		Success: true,
		Data:    nil,
		Message: "Resource image updated successfully",
	})
}

func AddResourceRoutes(api fiber.Router) {
	resource := api.Group("/resource")
	{
		resource.Post("/", handleCreateResource)
		resource.Get("/search", handleSearchResources)
		resource.Get("/", handleListResources)
		resource.Get("/random", handleGetRandomResource)
		resource.Get("/pinned", handleGetPinnedResources)
		resource.Get("/vndb/info", handleGetInfoFromVndb)
		resource.Get("/vndb/prefill", handleGetResourcePrefillFromVndb)
		resource.Get("/vndb/find", handleGetResourceByVNID)
		resource.Get("/characters/low-resolution", handleGetLowResolutionCharacters)
		resource.Get("/images/low-resolution", handleGetLowResolutionResourceImages)
		resource.Get("/admin/all", handleListAllResourcesForAdmin)
		resource.Get("/:id", handleGetResource)
		resource.Post("/:id/view", handleAddResourceView)
		resource.Delete("/:id", handleDeleteResource)
		resource.Get("/tag/:tag", handleListResourcesWithTag)
		resource.Get("/user/:username", handleGetResourcesWithUser)
		resource.Post("/:id", handleUpdateResource)
		resource.Put("/:resourceId/character/:characterId/image", handleUpdateCharacterImage)
		resource.Put("/:resourceId/image/:oldImageId", handleUpdateResourceImage)
	}
}
