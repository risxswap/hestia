package agent

import (
	baseapp "hestia/server/internal/app"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, _ *baseapp.Deps) {
	handler := NewHandler(NewService())
	group.POST("/stream", handler.Stream)
}
