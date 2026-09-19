package main

import (
	"log"
	"nysoure/server/api"
	"nysoure/server/dao"
	"nysoure/server/middleware"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/logger"
	recovermw "github.com/gofiber/fiber/v3/middleware/recover"
	prom "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	dao.InitDB()

	app := fiber.New(fiber.Config{
		BodyLimit:   8 * 1024 * 1024,
		TrustProxy:  true,
		ProxyHeader: "X-Real-IP",
		TrustProxyConfig: fiber.TrustProxyConfig{
			Private: true,
		},
	})

	app.Use(logger.New(logger.Config{
		Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
	}))

	app.Use(middleware.UnsupportedRegionMiddleware)

	app.Use(middleware.ErrorHandler)
	app.Use(recovermw.New())

	app.Use(middleware.RealUserMiddleware)

	app.Use(middleware.JwtMiddleware)

	app.Use(middleware.StaticContentMiddleware)

	app.Use(middleware.StatMiddleware)

	app.Use(middleware.GlobalDevMiddleware())

	app.Get("/metrics", adaptor.HTTPHandler(prom.Handler()))

	api.AddRootImageRoutes(app)

	apiG := app.Group("/api")
	apiG.Use(middleware.PrivateMiddleware)
	{
		api.AddUserRoutes(apiG)
		api.AddTagRoutes(apiG)
		api.AddImageRoutes(apiG)
		api.AddResourceRoutes(apiG)
		api.AddStorageRoutes(apiG)
		api.AddFileRoutes(apiG)
		api.AddCommentRoutes(apiG)
		api.AddConfigRoutes(apiG)
		api.AddActivityRoutes(apiG)
		api.AddCollectionRoutes(apiG)
		api.AddProxyRoutes(apiG)
		api.AddMoyuRoutes(apiG)
		api.AddDevAPI(apiG)
	}

	log.Fatal(app.Listen(":3000"))
}
