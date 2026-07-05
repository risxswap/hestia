package collection

import (
	"context"
	"log/slog"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/domain/asset"
	"hestia/server/internal/domain/clothes"
	"hestia/server/internal/domain/hair"
	"hestia/server/internal/domain/makeup"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var logger *slog.Logger
	var clothesLister ClothesLister
	var hairLister HairLister
	var makeupLister MakeupLister
	if deps != nil {
		logger = deps.Logger
		clothesService := clothes.NewService(clothes.NewMySQLRepository(deps.DB))
		clothesService.SetImageURLSigner(asset.NewServiceFromConfig(asset.NewMySQLRepository(deps.DB), deps.Config))
		if logger != nil {
			clothesService.SetImageURLSignErrorHandler(func(_ context.Context, objectKey string, err error) {
				logger.Warn("collection clothes image url signing failed", "object_key", objectKey, "error", err)
			})
		}
		clothesLister = clothesService
		hairLister = hair.NewService(hair.NewMySQLRepository(deps.DB))
		makeupLister = makeup.NewService(makeup.NewMySQLRepository(deps.DB))
	}
	RegisterUserRoutesWithService(group, NewServiceWithDomains(clothesLister, hairLister, makeupLister), logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, logger ...*slog.Logger) {
	var log *slog.Logger
	if len(logger) > 0 {
		log = logger[0]
	}
	handler := NewHandler(service, log)
	group.GET("", handler.Summary)
}
