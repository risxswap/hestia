package migration

import (
	"context"

	"hestia/server/internal/infra/config"
)

type Runner interface {
	Up(ctx context.Context, cfg *config.Config) error
}

type NoopRunner struct{}

func (NoopRunner) Up(context.Context, *config.Config) error {
	return nil
}

func RunOnStartup(ctx context.Context, cfg *config.Config, runner Runner) error {
	return runner.Up(ctx, cfg)
}
