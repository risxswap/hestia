package config

import (
	"errors"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv      string `env:"APP_ENV" envDefault:"development"`
	UserPort    string `env:"USER_SERVER_PORT" envDefault:"8080"`
	AdminPort   string `env:"ADMIN_SERVER_PORT" envDefault:"8081"`
	DatabaseDSN string `env:"DATABASE_DSN"`
	RedisAddr   string `env:"REDIS_ADDR"`
	LLMProvider string `env:"LLM_PROVIDER"`
}

func Load() (*Config, error) {
	_ = godotenv.Load(".env", "server/.env")

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	if cfg.UserPort == "" {
		return nil, errors.New("USER_SERVER_PORT cannot be empty")
	}
	if cfg.AdminPort == "" {
		return nil, errors.New("ADMIN_SERVER_PORT cannot be empty")
	}
	return &cfg, nil
}
