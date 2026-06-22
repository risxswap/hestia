package main

import (
	"net/http"
	"os"

	baseapp "hestia/server/internal/app"
	adminapp "hestia/server/internal/app/admin"
	"hestia/server/internal/infra/config"
	"hestia/server/internal/infra/logger"
)

func main() {
	if err := runAdminCommand(os.Args[1:], os.Stdout); err != nil {
		panic(err)
	}
}

func runAdminServer() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New()
	router := adminapp.NewRouter(&baseapp.Deps{Config: cfg, Logger: log})
	log.Info("admin-server listening", "port", cfg.AdminPort)
	if err := http.ListenAndServe(":"+cfg.AdminPort, router); err != nil {
		log.Error("admin-server stopped", "error", err)
		return err
	}
	return nil
}
