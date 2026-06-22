package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"hestia/server/internal/infra/config"
)

func TestLoadUsesDefaultPorts(t *testing.T) {
	unsetenv(t, "USER_SERVER_PORT")
	unsetenv(t, "ADMIN_SERVER_PORT")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.UserPort != "8080" {
		t.Fatalf("expected user port 8080, got %q", cfg.UserPort)
	}
	if cfg.AdminPort != "8081" {
		t.Fatalf("expected admin port 8081, got %q", cfg.AdminPort)
	}
}

func TestLoadUsesEnvironmentOverride(t *testing.T) {
	t.Setenv("USER_SERVER_PORT", "18080")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.UserPort != "18080" {
		t.Fatalf("expected user port 18080, got %q", cfg.UserPort)
	}
}

func TestLoadUsesConfigTOML(t *testing.T) {
	unsetenv(t, "DATABASE_DSN")
	unsetenv(t, "REDIS_ADDR")
	unsetenv(t, "REDIS_PASSWORD")
	unsetenv(t, "REDIS_DB")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(configPath, []byte(`
[mysql]
host = "127.0.0.1"
port = 3307
user = "app"
password = "secret"
database = "hestia_test"

[redis]
host = "127.0.0.2"
port = 6380
password = "redis-secret"
database = 3
`), 0o600)
	if err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	expectedDSN := "app:secret@tcp(127.0.0.1:3307)/hestia_test?charset=utf8mb4&parseTime=true&loc=Local"
	if cfg.DatabaseDSN != expectedDSN {
		t.Fatalf("expected database dsn %q, got %q", expectedDSN, cfg.DatabaseDSN)
	}
	if cfg.RedisAddr != "127.0.0.2:6380" {
		t.Fatalf("expected redis addr 127.0.0.2:6380, got %q", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "redis-secret" {
		t.Fatalf("expected redis password from config file")
	}
	if cfg.RedisDB != 3 {
		t.Fatalf("expected redis db 3, got %d", cfg.RedisDB)
	}
}

func unsetenv(t *testing.T, key string) {
	t.Helper()
	previous, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, previous)
			return
		}
		_ = os.Unsetenv(key)
	})
}
