package account

import (
	"log/slog"
	"strings"
	"time"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/common/auth"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.POST("/dev-login", handler.DevLogin)
}

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	if !devLoginAllowed(deps) {
		return
	}
	var repo UserRepository
	var sessions auth.SessionWriter
	var logger *slog.Logger
	if deps != nil {
		repo = NewMySQLUserRepository(deps.DB)
		sessions = auth.NewRedisSessionStore(deps.Redis)
		logger = deps.Logger
	}
	RegisterRoutes(group, NewHandler(NewService(repo, sessions, 30*24*time.Hour), logger))
}

func devLoginAllowed(deps *baseapp.Deps) bool {
	if deps == nil || deps.Config == nil {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(deps.Config.AppEnv)) {
	case "", "development", "test", "local":
		return true
	default:
		return false
	}
}
