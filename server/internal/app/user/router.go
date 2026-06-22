package user

import (
	baseapp "hestia/server/internal/app"
	"hestia/server/internal/common/response"
	"hestia/server/internal/domain/agent"

	"github.com/gin-gonic/gin"
)

func NewRouter(deps *baseapp.Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	api := router.Group("/api/user")
	api.GET("/health", health)
	agent.RegisterUserRoutes(api.Group("/agent"), deps)
	return router
}

func health(c *gin.Context) {
	response.OK(c, gin.H{
		"status":  "ok",
		"surface": "user",
	})
}
