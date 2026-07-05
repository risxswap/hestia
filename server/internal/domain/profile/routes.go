package profile

import (
	"log/slog"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/domain/asset"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo Repository
	var logger *slog.Logger
	if deps != nil {
		repo = NewMySQLRepository(deps.DB)
		logger = deps.Logger
	}
	service := NewService(repo)
	if deps != nil {
		service.SetImageURLSigner(asset.NewServiceFromConfig(asset.NewMySQLRepository(deps.DB), deps.Config))
	}
	RegisterUserRoutesWithService(group, service, logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, logger *slog.Logger) {
	handler := NewHandler(service, logger)
	group.GET("/summary", handler.Summary)
	group.PATCH("", handler.UpdateProfile)
	group.PATCH("/preferences", handler.UpdatePreferences)
	group.POST("/photos", handler.CreateProfilePhoto)
	group.PATCH("/photos/:public_id", handler.UpdateProfilePhoto)
	group.DELETE("/photos/:public_id", handler.DeleteProfilePhoto)
}
