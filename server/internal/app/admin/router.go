package admin

import (
	baseapp "hestia/server/internal/app"
	"hestia/server/internal/common/response"
	"hestia/server/internal/domain/job"

	"github.com/gin-gonic/gin"
)

func NewRouter(deps *baseapp.Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	api := router.Group("/api/admin")
	api.GET("/health", health)
	job.RegisterAdminRoutes(api.Group("/jobs"), deps)
	return router
}

func health(c *gin.Context) {
	response.OK(c, gin.H{
		"status":  "ok",
		"surface": "admin",
	})
}
