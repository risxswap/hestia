package agent

import (
	"context"
	"log/slog"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/domain/clothes"
	"hestia/server/internal/infra/llm"

	"github.com/cloudwego/eino/adk"
	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var clothesService ClothesAdviceService
	var repo Repository
	var logger *slog.Logger
	if deps != nil && deps.DB != nil {
		clothesService = clothes.NewService(clothes.NewMySQLRepository(deps.DB))
		repo = NewMySQLRepository(deps.DB)
	}
	if deps != nil {
		logger = deps.Logger
	}
	service := NewServiceWithDependencies(repo, clothesService)
	if deps != nil && deps.DB != nil && deps.LLM != nil {
		llmService := llm.NewService(llm.NewConfigResolver(llm.NewMySQLConfigRepository(deps.DB)), deps.LLM)
		llmService.SetLogger(logger)
		if chatModel, err := llmService.NewToolCallingChatModel(context.Background(), llm.Request{
			UsageKey:     "agent_chat",
			RequiredCaps: []string{"text"},
		}); err == nil {
			if runner, err := NewEinoADKChatModelAdviceRunner(context.Background(), chatModel, adk.ToolsConfig{}); err == nil {
				service.SetAdviceRunner(runner)
			}
		}
	}
	RegisterUserRoutesWithService(group, service, logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, logger ...*slog.Logger) {
	handler := NewHandler(service, optionalLogger(logger))
	group.POST("/chat", handler.Chat)
}

func RegisterAdviceDraftRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var service *Service
	var logger *slog.Logger
	if deps != nil && deps.DB != nil {
		service = NewServiceWithRepository(NewMySQLRepository(deps.DB))
	} else {
		service = NewService()
	}
	if deps != nil {
		logger = deps.Logger
	}
	RegisterAdviceDraftRoutesWithService(group, service, logger)
}

func RegisterAdviceDraftRoutesWithService(group *gin.RouterGroup, service *Service, logger ...*slog.Logger) {
	handler := NewHandler(service, optionalLogger(logger))
	group.GET("/current", handler.CurrentDraft)
	group.POST("/:public_id/confirm", handler.ConfirmDraft)
	group.POST("/:public_id/discard", handler.DiscardDraft)
}

func optionalLogger(loggers []*slog.Logger) *slog.Logger {
	if len(loggers) == 0 {
		return nil
	}
	return loggers[0]
}
