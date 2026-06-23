package imageroute

import (
	"log/slog"

	baseapp "hestia/server/internal/app"
	businesslock "hestia/server/internal/common/lock"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo FeedbackRepository
	var logger *slog.Logger
	var locker businesslock.Locker = businesslock.NoopLocker{}
	if deps != nil {
		repo = NewMySQLRepository(deps.DB)
		logger = deps.Logger
		locker = businesslock.NewRedisLocker(deps.Redis)
	}
	RegisterUserRoutesWithService(group, NewFeedbackServiceWithLocker(repo, locker), logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *FeedbackService, logger *slog.Logger) {
	handler := NewHandler(service, logger)
	group.POST("/:public_id/feedback", handler.ApplyFeedback)
}
