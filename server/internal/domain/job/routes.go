package job

import (
	"log/slog"

	baseapp "hestia/server/internal/app"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo Repository
	if deps != nil {
		repo = NewMySQLRepository(deps.DB)
	}
	RegisterUserRoutesWithService(group, NewService(repo), loggerFromDeps(deps))
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, _ *slog.Logger) {
	handler := NewHandler(service)
	group.GET("/:public_id", handler.GetForUser)
}

func loggerFromDeps(deps *baseapp.Deps) *slog.Logger {
	if deps == nil {
		return nil
	}
	return deps.Logger
}
