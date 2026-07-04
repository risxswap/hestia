package wardrobe

import (
	"context"
	"log/slog"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/domain/asset"
	"hestia/server/internal/infra/llm"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo Repository
	var logger *slog.Logger
	service := NewService(repo)
	if deps != nil {
		repo = NewMySQLRepository(deps.DB)
		logger = deps.Logger
		service = NewService(repo)
		service.SetImageURLSigner(asset.NewServiceFromConfig(asset.NewMySQLRepository(deps.DB), deps.Config))
		if deps.LLM != nil {
			llmRepo := llm.NewMySQLConfigRepository(deps.DB)
			service.SetImageRecognizer(NewLLMImageRecognizer(llm.NewService(llm.NewConfigResolver(llmRepo), deps.LLM)))
		}
		if logger != nil {
			service.SetImageURLSignErrorHandler(func(_ context.Context, objectKey string, err error) {
				logger.Warn("wardrobe primary image url signing failed", "object_key", objectKey, "error", err)
			})
		}
	}
	RegisterUserRoutesWithService(group, service, logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, logger *slog.Logger) {
	handler := NewHandler(service, logger)
	group.GET("/options", handler.GetOptions)
	group.GET("/items", handler.ListItems)
	group.POST("/items", handler.CreateItem)
	group.POST("/items/recognize", handler.RecognizeItemImage)
	group.PATCH("/items/:public_id", handler.UpdateItem)
	group.DELETE("/items/:public_id", handler.DeleteItem)
}
