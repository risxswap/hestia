package memory

import (
	"log/slog"

	baseapp "hestia/server/internal/app"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var service *Service
	var logger *slog.Logger
	if deps != nil {
		service = NewService(NewMySQLRepository(deps.DB))
		logger = deps.Logger
	} else {
		service = NewService(nil)
	}
	RegisterUserRoutesWithService(group, service, logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, logger *slog.Logger) {
	handler := NewHandler(service, logger)
	group.GET("", handler.ListItems)
	group.PATCH("/:public_id", handler.UpdateItem)
	group.DELETE("/:public_id", handler.DeleteItem)
}
