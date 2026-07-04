package collection

import (
	"context"
	"log/slog"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/domain/asset"
	"hestia/server/internal/domain/wardrobe"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var logger *slog.Logger
	var wardrobeLister WardrobeLister
	if deps != nil {
		logger = deps.Logger
		wardrobeService := wardrobe.NewService(wardrobe.NewMySQLRepository(deps.DB))
		wardrobeService.SetImageURLSigner(asset.NewServiceFromConfig(asset.NewMySQLRepository(deps.DB), deps.Config))
		if logger != nil {
			wardrobeService.SetImageURLSignErrorHandler(func(_ context.Context, objectKey string, err error) {
				logger.Warn("collection wardrobe image url signing failed", "object_key", objectKey, "error", err)
			})
		}
		wardrobeLister = wardrobeService
	}
	RegisterUserRoutesWithService(group, NewService(wardrobeLister), logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, logger ...*slog.Logger) {
	var log *slog.Logger
	if len(logger) > 0 {
		log = logger[0]
	}
	handler := NewHandler(service, log)
	group.GET("", handler.Summary)
}
