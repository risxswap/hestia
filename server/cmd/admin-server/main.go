package main

import (
	"context"
	"log/slog"
	"net/http"

	baseapp "hestia/server/internal/app"
	adminapp "hestia/server/internal/app/admin"
	"hestia/server/internal/infra/config"
	"hestia/server/internal/infra/logger"
	"hestia/server/internal/infra/migration"

	"github.com/gin-gonic/gin"
)

func main() {
	if err := runAdminServer(); err != nil {
		panic(err)
	}
}

func runAdminServer() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New()
	router, err := prepareAdminServer(context.Background(), cfg, log, migration.NoopRunner{})
	if err != nil {
		return err
	}
	log.Info("admin-server listening", "port", cfg.AdminPort)
	if err := http.ListenAndServe(":"+cfg.AdminPort, router); err != nil {
		log.Error("admin-server stopped", "error", err)
		return err
	}
	return nil
}

func prepareAdminServer(ctx context.Context, cfg *config.Config, log *slog.Logger, runner migration.Runner) (*gin.Engine, error) {
	if err := migration.RunOnStartup(ctx, cfg, runner); err != nil {
		return nil, err
	}
	return adminapp.NewRouter(&baseapp.Deps{Config: cfg, Logger: log}), nil
}
