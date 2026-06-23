package onboarding

import (
	"log/slog"

	baseapp "hestia/server/internal/app"
	businesslock "hestia/server/internal/common/lock"
	"hestia/server/internal/domain/generator"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.GET("", handler.Get)
	group.PUT("", handler.Save)
	group.POST("/submit", handler.Submit)
}

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo DraftRepository
	var logger *slog.Logger
	service := NewService(repo)
	if deps != nil {
		repo = NewMySQLDraftRepository(deps.DB)
		logger = deps.Logger
		reportGenerator := generator.NewRuleReportGenerator()
		submitDeps := NewMySQLSubmitDependencies(deps.DB, reportGenerator)
		submitDeps.Transactor = NewMySQLTransactor(deps.DB, reportGenerator)
		submitDeps.Locker = businesslock.NewRedisLocker(deps.Redis)
		service = NewSubmitService(repo, submitDeps)
	}
	RegisterRoutes(group, NewHandler(service, logger))
}
