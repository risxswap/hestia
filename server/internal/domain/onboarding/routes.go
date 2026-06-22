package onboarding

import (
	"log/slog"

	baseapp "hestia/server/internal/app"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.GET("", handler.Get)
	group.PUT("", handler.Save)
}

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo DraftRepository
	var logger *slog.Logger
	if deps != nil {
		repo = NewMySQLDraftRepository(deps.DB)
		logger = deps.Logger
	}
	RegisterRoutes(group, NewHandler(NewService(repo), logger))
}
