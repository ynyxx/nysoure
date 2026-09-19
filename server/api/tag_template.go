package api

import (
	"nysoure/server/ctx"
	"nysoure/server/model"
	"nysoure/server/service"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

type tagTemplateRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type applyTagTemplateRequest struct {
	Params map[string]string `json:"params"`
}

func handleListTagTemplates(c fiber.Ctx) error {
	templates, err := service.ListTagTemplates(ctx.NewContext(c))
	if err != nil {
		return err
	}
	if templates == nil {
		templates = []model.TagTemplateView{}
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[*[]model.TagTemplateView]{
		Success: true,
		Data:    &templates,
		Message: "Tag templates retrieved successfully",
	})
}

func handleCreateTagTemplate(c fiber.Ctx) error {
	var req tagTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return model.NewRequestError("Invalid request format")
	}
	t, err := service.CreateTagTemplate(ctx.NewContext(c), req.Name, req.Content)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[model.TagTemplateView]{
		Success: true,
		Data:    *t,
		Message: "Tag template created successfully",
	})
}

func handleUpdateTagTemplate(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return model.NewRequestError("Invalid template ID")
	}
	var req tagTemplateRequest
	if err := c.Bind().JSON(&req); err != nil {
		return model.NewRequestError("Invalid request format")
	}
	t, err := service.UpdateTagTemplate(ctx.NewContext(c), uint(id), req.Name, req.Content)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[model.TagTemplateView]{
		Success: true,
		Data:    *t,
		Message: "Tag template updated successfully",
	})
}

func handleDeleteTagTemplate(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return model.NewRequestError("Invalid template ID")
	}
	if err := service.DeleteTagTemplate(ctx.NewContext(c), uint(id)); err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[any]{
		Success: true,
		Message: "Tag template deleted successfully",
	})
}

func handleApplyTagTemplate(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return model.NewRequestError("Invalid template ID")
	}
	var req applyTagTemplateRequest
	if len(c.Body()) > 0 {
		if err := c.Bind().JSON(&req); err != nil {
			return model.NewRequestError("Invalid request format")
		}
	}
	tags, err := service.ApplyTagTemplate(ctx.NewContext(c), uint(id), req.Params)
	if err != nil {
		return err
	}
	if tags == nil {
		tags = []model.TagView{}
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[*[]model.TagView]{
		Success: true,
		Data:    &tags,
		Message: "Tag template applied successfully",
	})
}

func AddTagTemplateRoutes(api fiber.Router) {
	g := api.Group("/tag-template")
	g.Get("/", handleListTagTemplates)
	g.Post("/", handleCreateTagTemplate)
	g.Put("/:id", handleUpdateTagTemplate)
	g.Delete("/:id", handleDeleteTagTemplate)
	g.Post("/:id/apply", handleApplyTagTemplate)
}
