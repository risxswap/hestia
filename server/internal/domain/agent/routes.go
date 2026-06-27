package agent

import (
	baseapp "hestia/server/internal/app"
	"hestia/server/internal/domain/wardrobe"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var wardrobeService WardrobeAdviceService
	if deps != nil {
		wardrobeService = wardrobe.NewService(wardrobe.NewMySQLRepository(deps.DB))
	}
	RegisterUserRoutesWithService(group, NewServiceWithWardrobe(wardrobeService))
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service) {
	handler := NewHandler(service)
	group.POST("/stream", handler.Stream)
}
