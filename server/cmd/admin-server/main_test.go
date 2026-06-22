package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"hestia/server/internal/infra/config"
)

func TestPrepareAdminServerRunsMigrationBeforeReturningRouter(t *testing.T) {
	runner := &startupMigrationRunner{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	router, err := prepareAdminServer(context.Background(), &config.Config{AdminPort: "8081"}, logger, runner)
	if err != nil {
		t.Fatalf("prepare admin server: %v", err)
	}
	if router == nil {
		t.Fatal("expected router")
	}
	if runner.calls != 1 {
		t.Fatalf("expected migration runner to be called once, got %d", runner.calls)
	}
}

type startupMigrationRunner struct {
	calls int
}

func (s *startupMigrationRunner) Up(context.Context, *config.Config) error {
	s.calls++
	return nil
}
