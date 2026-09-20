package api

import (
	"nysoure/server/model"
	"nysoure/server/service"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func handleGetMoyuPatch(c fiber.Ctx) error {
	vndbID := strings.TrimSpace(c.Query("vndb_id"))
	if vndbID == "" {
		return model.NewRequestError("vndb_id is required")
	}
	patch, err := service.GetMoyuPatchByVNDB(vndbID)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(model.Response[*service.MoyuPatch]{
		Success: true,
		Data:    patch,
		Message: "ok",
	})
}

func AddMoyuRoutes(api fiber.Router) {
	g := api.Group("/moyu")
	g.Get("/patch", handleGetMoyuPatch)
}
