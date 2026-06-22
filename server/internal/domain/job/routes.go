package job

import (
	baseapp "hestia/server/internal/app"

	"github.com/gin-gonic/gin"
)

func RegisterAdminRoutes(group *gin.RouterGroup, _ *baseapp.Deps) {
	handler := NewHandler(NewService())
	group.GET("", handler.List)
}
