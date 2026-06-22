package config_test

import (
	"os"
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
