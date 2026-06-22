package imageroute

import (
	"log/slog"

	baseapp "hestia/server/internal/app"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo FeedbackRepository
	var logger *slog.Logger
	if deps != nil {
		repo = NewMySQLRepository(deps.DB)
		logger = deps.Logger
	}
	RegisterUserRoutesWithService(group, NewFeedbackService(repo), logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *FeedbackService, logger *slog.Logger) {
	handler := NewHandler(service, logger)
	group.POST("/:public_id/feedback", handler.ApplyFeedback)
}
