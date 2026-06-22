package main

import (
	"log/slog"
	"net/http"

	baseapp "hestia/server/internal/app"
	userapp "hestia/server/internal/app/user"
	"hestia/server/internal/infra/config"
	"hestia/server/internal/infra/logger"
	mysqlinfra "hestia/server/internal/infra/mysql"

	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New()
	deps, cleanup, err := newDeps(cfg, log)
	if err != nil {
		log.Error("user-server deps init failed", "error", err)
		panic(err)
	}
	defer cleanup()
	router := userapp.NewRouter(deps)
	log.Info("user-server listening", "port", cfg.UserPort)
	if err := http.ListenAndServe(":"+cfg.UserPort, router); err != nil {
		log.Error("user-server stopped", "error", err)
		panic(err)
	}
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
	return &baseapp.Deps{Config: cfg, DB: db, Redis: redisClient, Logger: log}, cleanup, nil
}
