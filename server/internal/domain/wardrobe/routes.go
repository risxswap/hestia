package wardrobe

import (
	"log/slog"

	baseapp "hestia/server/internal/app"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo Repository
	var logger *slog.Logger
	if deps != nil {
		repo = NewMySQLRepository(deps.DB)
		logger = deps.Logger
	}
	RegisterUserRoutesWithService(group, NewService(repo), logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, logger *slog.Logger) {
	handler := NewHandler(service, logger)
	group.GET("/items", handler.ListItems)
	group.POST("/items", handler.CreateItem)
	group.PATCH("/items/:public_id", handler.UpdateItem)
	group.DELETE("/items/:public_id", handler.DeleteItem)
}
