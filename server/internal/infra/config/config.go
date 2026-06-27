package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	AppEnv                     string `env:"APP_ENV" envDefault:"development"`
	Port                       string `env:"SERVER_PORT" envDefault:"8080"`
	DatabaseDSN                string `env:"DATABASE_DSN"`
	RedisAddr                  string `env:"REDIS_ADDR"`
	RedisPassword              string `env:"REDIS_PASSWORD"`
	RedisDB                    int    `env:"REDIS_DB"`
	LLMProvider                string `env:"LLM_PROVIDER"`
	QiniuAccessKey             string `env:"QINIU_ACCESS_KEY"`
	QiniuSecretKey             string `env:"QINIU_SECRET_KEY"`
	QiniuBucket                string `env:"QINIU_BUCKET"`
	QiniuUploadHost            string `env:"QINIU_UPLOAD_HOST"`
	QiniuPrivateDomain         string `env:"QINIU_PRIVATE_DOMAIN"`
	QiniuUploadTokenTTLSeconds int    `env:"QINIU_UPLOAD_TOKEN_TTL_SECONDS"`
	QiniuDownloadURLTTLSeconds int    `env:"QINIU_DOWNLOAD_URL_TTL_SECONDS"`
}

func Load() (*Config, error) {
	_ = godotenv.Load(".env", "server/.env")

	cfg := Config{
		AppEnv:                     "development",
		Port:                       "8080",
		QiniuUploadTokenTTLSeconds: 3600,
		QiniuDownloadURLTTLSeconds: 900,
	}
	if err := applyTOMLConfig(&cfg); err != nil {
		return nil, err
	}
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	if cfg.Port == "" {
		return nil, errors.New("SERVER_PORT cannot be empty")
	}
	return &cfg, nil
}

type fileConfig struct {
	MySQL mysqlConfig `toml:"mysql"`
	Redis redisConfig `toml:"redis"`
	Qiniu qiniuConfig `toml:"qiniu"`
}

type mysqlConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Database string `toml:"database"`
}

type redisConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Password string `toml:"password"`
	Database int    `toml:"database"`
}

type qiniuConfig struct {
	AccessKey             string `toml:"access_key"`
	SecretKey             string `toml:"secret_key"`
	Bucket                string `toml:"bucket"`
	UploadHost            string `toml:"upload_host"`
	PrivateDomain         string `toml:"private_domain"`
	UploadTokenTTLSeconds int    `toml:"upload_token_ttl_seconds"`
	DownloadURLTTLSeconds int    `toml:"download_url_ttl_seconds"`
}

func applyTOMLConfig(cfg *Config) error {
	path, ok, err := findConfigFile()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file %s: %w", path, err)
	}

	var fc fileConfig
	if err := toml.Unmarshal(raw, &fc); err != nil {
		return fmt.Errorf("parse config file %s: %w", path, err)
	}

	if fc.MySQL.Host != "" {
		if fc.MySQL.Port == 0 {
			fc.MySQL.Port = 3306
		}
		cfg.DatabaseDSN = fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
			fc.MySQL.User,
			fc.MySQL.Password,
			fc.MySQL.Host,
			fc.MySQL.Port,
			fc.MySQL.Database,
		)
	}
	if fc.Redis.Host != "" {
		if fc.Redis.Port == 0 {
			fc.Redis.Port = 6379
		}
		cfg.RedisAddr = fmt.Sprintf("%s:%d", fc.Redis.Host, fc.Redis.Port)
		cfg.RedisPassword = fc.Redis.Password
		cfg.RedisDB = fc.Redis.Database
	}
	if fc.Qiniu.AccessKey != "" {
		cfg.QiniuAccessKey = fc.Qiniu.AccessKey
	}
	if fc.Qiniu.SecretKey != "" {
		cfg.QiniuSecretKey = fc.Qiniu.SecretKey
	}
	if fc.Qiniu.Bucket != "" {
		cfg.QiniuBucket = fc.Qiniu.Bucket
	}
	if fc.Qiniu.UploadHost != "" {
		cfg.QiniuUploadHost = fc.Qiniu.UploadHost
	}
	if fc.Qiniu.PrivateDomain != "" {
		cfg.QiniuPrivateDomain = fc.Qiniu.PrivateDomain
	}
	if fc.Qiniu.UploadTokenTTLSeconds > 0 {
		cfg.QiniuUploadTokenTTLSeconds = fc.Qiniu.UploadTokenTTLSeconds
	}
	if fc.Qiniu.DownloadURLTTLSeconds > 0 {
		cfg.QiniuDownloadURLTTLSeconds = fc.Qiniu.DownloadURLTTLSeconds
	}

	return nil
}

func findConfigFile() (string, bool, error) {
	if path := os.Getenv("CONFIG_FILE"); path != "" {
		if _, err := os.Stat(path); err != nil {
			return "", false, fmt.Errorf("stat config file %s: %w", path, err)
		}
		return path, true, nil
	}

	for _, path := range []string{"config.toml", "server/config.toml"} {
		if _, err := os.Stat(path); err == nil {
			return path, true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", false, fmt.Errorf("stat config file %s: %w", path, err)
		}
	}

	return "", false, nil
}
