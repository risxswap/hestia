package asset

import (
	"log/slog"

	baseapp "hestia/server/internal/app"

	"github.com/gin-gonic/gin"
)

func RegisterFileRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo Repository
	var logger *slog.Logger
	var service *Service
	if deps != nil {
		repo = NewMySQLRepository(deps.DB)
		logger = deps.Logger
		service = NewServiceFromConfig(repo, deps.Config)
	} else {
		service = NewService(repo)
	}
	RegisterFileRoutesWithService(group, service, logger)
}

func RegisterFileRoutesWithService(group *gin.RouterGroup, service *Service, logger *slog.Logger) {
	handler := NewHandler(service, logger)
	group.POST("/upload-token", handler.UploadToken)
	group.POST("/confirm", handler.Confirm)
}
