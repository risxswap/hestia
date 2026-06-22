package main

import (
	"net/http"

	baseapp "hestia/server/internal/app"
	userapp "hestia/server/internal/app/user"
	"hestia/server/internal/infra/config"
	"hestia/server/internal/infra/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New()
	router := userapp.NewRouter(&baseapp.Deps{Config: cfg, Logger: log})
	log.Info("user-server listening", "port", cfg.UserPort)
	if err := http.ListenAndServe(":"+cfg.UserPort, router); err != nil {
		log.Error("user-server stopped", "error", err)
		panic(err)
	}
}
