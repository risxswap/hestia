package agent

import (
	baseapp "hestia/server/internal/app"
	"hestia/server/internal/domain/clothes"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var clothesService ClothesAdviceService
	if deps != nil {
		clothesService = clothes.NewService(clothes.NewMySQLRepository(deps.DB))
	}
	RegisterUserRoutesWithService(group, NewServiceWithClothes(clothesService))
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service) {
	handler := NewHandler(service)
	group.POST("/stream", handler.Stream)
}
