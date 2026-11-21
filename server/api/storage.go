package api

import (
	"nysoure/server/ctx"
	"nysoure/server/model"
	"nysoure/server/service"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

func handleCreateS3Storage(c fiber.Ctx) error {
	var params service.CreateS3StorageParams
	if err := c.Bind().JSON(&params); err != nil {
		return model.NewRequestError("Invalid request body")
	}

	if params.Name == "" || params.EndPoint == "" || params.AccessKeyID == "" ||
		params.SecretAccessKey == "" || params.BucketName == "" {
		return model.NewRequestError("All fields are required")
	}

	if params.MaxSizeInMB <= 0 {
		return model.NewRequestError("Max size must be greater than 0")
	}

	context := ctx.NewContext(c)
	err := service.CreateS3Storage(context, params)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(model.Response[any]{
		Success: true,
		Message: "S3 storage created successfully",
	})
}

func handleCreateLocalStorage(c fiber.Ctx) error {
	var params service.CreateLocalStorageParams
	if err := c.Bind().JSON(&params); err != nil {
		return model.NewRequestError("Invalid request body")
	}

	if params.Name == "" || params.Path == "" {
		return model.NewRequestError("All fields are required")
	}

	if params.MaxSizeInMB <= 0 {
		return model.NewRequestError("Max size must be greater than 0")
	}

	context := ctx.NewContext(c)
	err := service.CreateLocalStorage(context, params)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(model.Response[any]{
		Success: true,
		Message: "Local storage created successfully",
	})
}

func handleCreateFTPStorage(c fiber.Ctx) error {
	var params service.CreateFTPStorageParams
	if err := c.Bind().JSON(&params); err != nil {
		return model.NewRequestError("Invalid request body")
	}

	if params.Name == "" || params.Host == "" || params.Username == "" ||
		params.Password == "" || params.Domain == "" {
		return model.NewRequestError("All fields are required")
	}

	if params.MaxSizeInMB <= 0 {
		return model.NewRequestError("Max size must be greater than 0")
	}

	context := ctx.NewContext(c)
	err := service.CreateFTPStorage(context, params)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(model.Response[any]{
		Success: true,
		Message: "FTP storage created successfully",
	})
}

func handleCreateGoogleDriveStorage(c fiber.Ctx) error {
	var params service.CreateGoogleDriveStorageParams
	if err := c.Bind().JSON(&params); err != nil {
		return model.NewRequestError("Invalid request body")
	}

	if params.Name == "" || params.ClientID == "" || params.ClientSecret == "" || params.RefreshToken == "" {
		return model.NewRequestError("All fields are required")
	}

	if params.MaxSizeInMB <= 0 {
		return model.NewRequestError("Max size must be greater than 0")
	}

	context := ctx.NewContext(c)
	err := service.CreateGoogleDriveStorage(context, params)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(model.Response[any]{
		Success: true,
		Message: "Google Drive storage created successfully",
	})
}

func handleListStorages(c fiber.Ctx) error {
	storages, err := service.ListStorages()
	if err != nil {
		return err
	}

	if storages == nil {
		storages = []model.StorageView{}
	}

	return c.Status(fiber.StatusOK).JSON(model.Response[*[]model.StorageView]{
		Success: true,
		Data:    &storages,
		Message: "Storages retrieved successfully",
	})
}

func handleDeleteStorage(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return model.NewRequestError("Invalid storage ID")
	}

	context := ctx.NewContext(c)
	err = service.DeleteStorage(context, uint(id))
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(model.Response[any]{
		Success: true,
		Message: "Storage deleted successfully",
	})
}

func handleSetDefaultStorage(c fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return model.NewRequestError("Invalid storage ID")
	}

	context := ctx.NewContext(c)
	err = service.SetDefaultStorage(context, uint(id))
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(model.Response[any]{
		Success: true,
		Message: "Default storage set successfully",
	})
}

func AddStorageRoutes(r fiber.Router) {
	s := r.Group("storage")
	s.Post("/s3", handleCreateS3Storage)
	s.Post("/local", handleCreateLocalStorage)
	s.Post("/ftp", handleCreateFTPStorage)
	s.Post("/gdrive", handleCreateGoogleDriveStorage)
	s.Get("/", handleListStorages)
	s.Delete("/:id", handleDeleteStorage)
	s.Put("/:id/default", handleSetDefaultStorage)
}
