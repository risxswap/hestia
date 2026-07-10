package agent

import (
	"log/slog"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/domain/clothes"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var clothesService ClothesAdviceService
	var repo Repository
	var logger *slog.Logger
	if deps != nil && deps.DB != nil {
		clothesService = clothes.NewService(clothes.NewMySQLRepository(deps.DB))
		repo = NewMySQLRepository(deps.DB)
	}
	if deps != nil {
		logger = deps.Logger
	}
	RegisterUserRoutesWithService(group, NewServiceWithDependencies(repo, clothesService), logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, logger ...*slog.Logger) {
	handler := NewHandler(service, optionalLogger(logger))
	group.POST("/chat", handler.Chat)
}

func RegisterAdviceDraftRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var service *Service
	var logger *slog.Logger
	if deps != nil && deps.DB != nil {
		service = NewServiceWithRepository(NewMySQLRepository(deps.DB))
	} else {
		service = NewService()
	}
	if deps != nil {
		logger = deps.Logger
	}
	RegisterAdviceDraftRoutesWithService(group, service, logger)
}

func RegisterAdviceDraftRoutesWithService(group *gin.RouterGroup, service *Service, logger ...*slog.Logger) {
	handler := NewHandler(service, optionalLogger(logger))
	group.GET("/current", handler.CurrentDraft)
	group.POST("/:public_id/confirm", handler.ConfirmDraft)
	group.POST("/:public_id/discard", handler.DiscardDraft)
}

func optionalLogger(loggers []*slog.Logger) *slog.Logger {
	if len(loggers) == 0 {
		return nil
	}
	return loggers[0]
}
