package main

import (
	"io"
	"log/slog"
	"testing"

	"hestia/server/internal/infra/config"
	"hestia/server/internal/infra/migration"
)

func TestNewDepsIncludesDBWhenDatabaseDSNIsConfigured(t *testing.T) {
	deps, cleanup, err := newDeps(&config.Config{
		DatabaseDSN: "user:pass@tcp(localhost:3306)/hestia?parseTime=true",
		RedisAddr:   "localhost:6379",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if cleanup != nil {
		defer cleanup()
	}

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if deps.DB == nil {
		t.Fatalf("expected db dependency")
	}
	if deps.Redis == nil {
		t.Fatalf("expected redis dependency")
	}
}

func TestNewDepsAllowsEmptyDatabaseDSN(t *testing.T) {
	deps, cleanup, err := newDeps(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if cleanup != nil {
		defer cleanup()
	}

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if deps.DB != nil {
		t.Fatalf("expected nil db, got %#v", deps.DB)
	}
}

func TestNewServerMigrationRunnerUsesMySQLRunner(t *testing.T) {
	if _, ok := newServerMigrationRunner().(migration.MySQLRunner); !ok {
		t.Fatalf("expected server startup to use MySQL migration runner")
	}
}
