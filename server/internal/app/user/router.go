package user

import (
	baseapp "hestia/server/internal/app"
	"hestia/server/internal/common/auth"
	"hestia/server/internal/common/response"
	"hestia/server/internal/domain/account"
	"hestia/server/internal/domain/agent"
	"hestia/server/internal/domain/imageroute"
	"hestia/server/internal/domain/job"
	"hestia/server/internal/domain/onboarding"
	"hestia/server/internal/domain/report"

	"github.com/gin-gonic/gin"
)

func NewRouter(deps *baseapp.Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	api := router.Group("/api/user")
	api.GET("/health", health)
	account.RegisterUserRoutes(api, deps)
	agent.RegisterUserRoutes(api.Group("/agent"), deps)
	protected := api.Group("")
	if deps == nil {
		protected.Use(auth.RequireUserSession(auth.NewRedisSessionStore(nil)))
	} else {
		protected.Use(auth.RequireUserSession(auth.NewRedisSessionStore(deps.Redis)))
	}
	onboarding.RegisterUserRoutes(protected.Group("/onboarding"), deps)
	job.RegisterUserRoutes(protected.Group("/jobs"), deps)
	report.RegisterUserRoutes(protected.Group("/reports"), deps)
	imageroute.RegisterUserRoutes(protected.Group("/image-routes"), deps)
	return router
}

func health(c *gin.Context) {
	response.OK(c, gin.H{
		"status":  "ok",
		"surface": "user",
	})
}
