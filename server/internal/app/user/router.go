package user

import (
	baseapp "hestia/server/internal/app"
	"hestia/server/internal/common/auth"
	"hestia/server/internal/common/response"
	"hestia/server/internal/domain/account"
	"hestia/server/internal/domain/agent"
	"hestia/server/internal/domain/asset"
	"hestia/server/internal/domain/clothes"
	"hestia/server/internal/domain/collection"
	"hestia/server/internal/domain/hair"
	"hestia/server/internal/domain/imageroute"
	"hestia/server/internal/domain/job"
	"hestia/server/internal/domain/makeup"
	"hestia/server/internal/domain/onboarding"
	"hestia/server/internal/domain/profile"
	"hestia/server/internal/domain/report"

	"github.com/gin-gonic/gin"
)

func NewRouter(deps *baseapp.Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	api := router.Group("/api/user")
	api.GET("/health", health)
	account.RegisterUserRoutes(api, deps)
	protected := api.Group("")
	if deps == nil {
		protected.Use(auth.RequireUserSession(auth.NewRedisSessionStore(nil)))
	} else {
		protected.Use(auth.RequireUserSession(auth.NewRedisSessionStore(deps.Redis)))
	}
	registerFileRoutes(protected.Group("/files"), deps)
	agent.RegisterUserRoutes(protected.Group("/agent"), deps)
	onboarding.RegisterUserRoutes(protected.Group("/onboarding"), deps)
	profile.RegisterUserRoutes(protected.Group("/profile"), deps)
	job.RegisterUserRoutes(protected.Group("/jobs"), deps)
	report.RegisterUserRoutes(protected.Group("/reports"), deps)
	imageroute.RegisterUserRoutes(protected.Group("/image-routes"), deps)
	collection.RegisterUserRoutes(protected.Group("/collection"), deps)
	clothes.RegisterUserRoutes(protected.Group("/clothes"), deps)
	hair.RegisterUserRoutes(protected.Group("/hair"), deps)
	makeup.RegisterUserRoutes(protected.Group("/makeup"), deps)
	return router
}

func registerFileRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	if deps == nil {
		asset.RegisterFileRoutes(group, deps)
		return
	}
	repo := asset.NewMySQLRepository(deps.DB)
	baseFileService := asset.NewServiceFromConfig(repo, deps.Config)
	options := baseFileService.Options()
	service := asset.NewServiceWithOptions(repo, options)
	asset.RegisterFileRoutesWithService(group, service, deps.Logger)
}

func health(c *gin.Context) {
	response.OK(c, gin.H{
		"status":  "ok",
		"surface": "user",
	})
}
