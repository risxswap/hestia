package migration_test

import (
	"context"
	"errors"
	"testing"

	"hestia/server/internal/infra/config"
	"hestia/server/internal/infra/migration"
)

func TestRunOnStartupCallsRunnerUp(t *testing.T) {
	runner := &fakeRunner{}
	cfg := &config.Config{DatabaseDSN: "mysql://example"}

	if err := migration.RunOnStartup(context.Background(), cfg, runner); err != nil {
		t.Fatalf("run startup migration: %v", err)
	}

	if runner.calls != 1 {
		t.Fatalf("expected runner to be called once, got %d", runner.calls)
	}
	if runner.lastConfig != cfg {
		t.Fatal("expected runner to receive config")
	}
}

func TestRunOnStartupReturnsRunnerError(t *testing.T) {
	expected := errors.New("migration failed")
	runner := &fakeRunner{err: expected}

	err := migration.RunOnStartup(context.Background(), &config.Config{}, runner)
	if !errors.Is(err, expected) {
		t.Fatalf("expected runner error, got %v", err)
	}
}

type fakeRunner struct {
	calls      int
	lastConfig *config.Config
	err        error
}

func (f *fakeRunner) Up(_ context.Context, cfg *config.Config) error {
	f.calls++
	f.lastConfig = cfg
	return f.err
}
