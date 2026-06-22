package onboarding

import (
	"log/slog"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/domain/asset"
	"hestia/server/internal/domain/generator"
	"hestia/server/internal/domain/imageroute"
	"hestia/server/internal/domain/job"
	"hestia/server/internal/domain/profile"
	"hestia/server/internal/domain/report"
	"hestia/server/internal/domain/wardrobe"

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
		service = NewSubmitService(repo, SubmitDependencies{
			Profiles:    profile.NewService(profile.NewMySQLRepository(deps.DB)),
			Assets:      asset.NewService(asset.NewMySQLRepository(deps.DB)),
			Wardrobe:    wardrobe.NewService(wardrobe.NewMySQLRepository(deps.DB)),
			Jobs:        job.NewService(job.NewMySQLRepository(deps.DB)),
			Reports:     report.NewService(report.NewMySQLRepository(deps.DB)),
			ImageRoutes: imageroute.NewService(imageroute.NewMySQLRepository(deps.DB)),
			Generator:   generator.NewRuleReportGenerator(),
		})
	}
	RegisterRoutes(group, NewHandler(service, logger))
}
