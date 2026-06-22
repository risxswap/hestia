package app

import (
	"log/slog"

	"hestia/server/internal/infra/config"
	"hestia/server/internal/infra/llm"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	Config *config.Config
	DB     *sqlx.DB
	Redis  *redis.Client
	Queue  *asynq.Client
	LLM    llm.Client
	Logger *slog.Logger
}
