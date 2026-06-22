package report

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
	group.GET("/latest", handler.Latest)
	group.GET("/:public_id", handler.Get)
}
