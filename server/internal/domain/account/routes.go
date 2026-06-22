package account

import (
	"time"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/common/auth"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.POST("/dev-login", handler.DevLogin)
}

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo UserRepository
	var sessions auth.SessionWriter
	if deps != nil {
		repo = NewMySQLUserRepository(deps.DB)
		sessions = auth.NewRedisSessionStore(deps.Redis)
	}
	RegisterRoutes(group, NewHandler(NewService(repo, sessions, 30*24*time.Hour)))
}
