package hair

import (
	baseapp "hestia/server/internal/app"
	"hestia/server/internal/common/response"

	"github.com/gin-gonic/gin"
)

type ListResponse struct {
	Items   []any `json:"items"`
	Enabled bool  `json:"enabled"`
}

func RegisterUserRoutes(group *gin.RouterGroup, _ *baseapp.Deps) {
	group.GET("", func(c *gin.Context) {
		response.OK(c, ListResponse{Items: []any{}, Enabled: true})
	})
}
