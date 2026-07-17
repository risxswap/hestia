package agent

import (
	"context"
	"log/slog"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/domain/clothes"
	"hestia/server/internal/domain/memory"
	"hestia/server/internal/domain/profile"
	"hestia/server/internal/infra/llm"
	serverlogger "hestia/server/internal/infra/logger"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var clothesService ClothesAdviceService
	var profileService ProfileContextService
	var memoryService MemoryContextService
	var repo Repository
	var logger *slog.Logger
	if deps != nil && deps.DB != nil {
		clothesService = clothes.NewService(clothes.NewMySQLRepository(deps.DB))
		profileService = profile.NewService(profile.NewMySQLRepository(deps.DB))
		memoryService = memory.NewService(memory.NewMySQLRepository(deps.DB))
		repo = NewMySQLRepository(deps.DB)
	}
	if deps != nil {
		logger = deps.Logger
	}
	service := NewServiceWithDependencies(repo, clothesService)
	service.SetLogger(logger)
	if deps != nil && deps.DB != nil && deps.LLM != nil {
		llmService := llm.NewService(llm.NewConfigResolver(llm.NewMySQLConfigRepository(deps.DB)), deps.LLM)
		llmService.SetLogger(logger)
		if chatModel, resolvedUsage, err := llmService.NewToolCallingChatModelWithUsage(context.Background(), llm.Request{
			UsageKey:     "agent_chat",
			RequiredCaps: []string{"text"},
		}); err == nil {
			if tools, err := NewAdviceToolsWithDependencies(repo, AdviceToolDependencies{
				Clothes: clothesService,
				Profile: profileService,
				Memory:  memoryService,
			}); err == nil {
				toolsConfig := adk.ToolsConfig{
					ToolsNodeConfig: compose.ToolsNodeConfig{
						Tools:               tools,
						ExecuteSequentially: true,
					},
				}
				metadata := AdviceRunMetadata{
					UsageKey:      resolvedUsage.Usage.Key,
					ProviderCode:  resolvedUsage.Provider.Code,
					ModelCode:     resolvedUsage.Model.ModelCode,
					PromptVersion: resolvedUsage.Usage.PromptVersion,
					MaxIterations: einoADKMaxIterations,
				}
				if runner, err := NewEinoADKChatModelAdviceRunnerWithMetadata(context.Background(), chatModel, toolsConfig, metadata); err == nil {
					service.SetAdviceRunner(runner)
				} else {
					logAgentSetupError(logger, "runner_init", err)
				}
			} else {
				logAgentSetupError(logger, "tools_init", err)
			}
		} else {
			logAgentSetupError(logger, "model_init", err)
		}
	}
	RegisterUserRoutesWithService(group, service, logger)
}

func logAgentSetupError(logger *slog.Logger, stage string, err error) {
	if err == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	logger.Error("agent setup failed", "error_stage", stage, "error", serverlogger.ErrorSummary(err))
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, logger ...*slog.Logger) {
	handler := NewHandler(service, optionalLogger(logger))
	group.POST("/chat", handler.Chat)
	group.GET("/messages", handler.Messages)
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
	group.GET("/:public_id/versions", handler.DraftVersions)
	group.POST("/:public_id/confirm", handler.ConfirmDraft)
	group.POST("/:public_id/discard", handler.DiscardDraft)
}

func optionalLogger(loggers []*slog.Logger) *slog.Logger {
	if len(loggers) == 0 {
		return nil
	}
	return loggers[0]
}
