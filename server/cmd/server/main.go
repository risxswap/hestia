package main

import (
	"context"
	"log/slog"
	"net/http"

	baseapp "hestia/server/internal/app"
	userapp "hestia/server/internal/app/user"
	"hestia/server/internal/infra/config"
	"hestia/server/internal/infra/llm"
	"hestia/server/internal/infra/logger"
	"hestia/server/internal/infra/migration"
	mysqlinfra "hestia/server/internal/infra/mysql"

	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New()
	if err := migration.RunOnStartup(context.Background(), cfg, newServerMigrationRunner()); err != nil {
		log.Error("server migration failed", "error", err)
		panic(err)
	}
	deps, cleanup, err := newDeps(cfg, log)
	if err != nil {
		log.Error("server deps init failed", "error", err)
		panic(err)
	}
	defer cleanup()
	router := userapp.NewRouter(deps)
	log.Info("server listening", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Error("server stopped", "error", err)
		panic(err)
	}
}

func newServerMigrationRunner() migration.Runner {
	return migration.NewMySQLRunner()
}

func newDeps(cfg *config.Config, log *slog.Logger) (*baseapp.Deps, func(), error) {
	db, err := mysqlinfra.Open(cfg)
	if err != nil {
		return nil, nil, err
	}
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
	}
	cleanup := func() {
		if redisClient != nil {
			_ = redisClient.Close()
		}
		if db != nil {
			_ = db.Close()
		}
	}
	var llmClient llm.Client
	if db != nil {
		llmClient = llm.NewEinoClient()
	}
	return &baseapp.Deps{Config: cfg, DB: db, Redis: redisClient, LLM: llmClient, Logger: log}, cleanup, nil
}
