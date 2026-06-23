package onboarding

import (
	"context"

	businesslock "hestia/server/internal/common/lock"
	"hestia/server/internal/domain/asset"
	"hestia/server/internal/domain/generator"
	"hestia/server/internal/domain/imageroute"
	"hestia/server/internal/domain/job"
	"hestia/server/internal/domain/profile"
	"hestia/server/internal/domain/report"
	"hestia/server/internal/domain/wardrobe"

	"github.com/jmoiron/sqlx"
)

type MySQLTransactor struct {
	db        *sqlx.DB
	generator generator.ReportGenerator
}

func NewMySQLTransactor(db *sqlx.DB, generator generator.ReportGenerator) *MySQLTransactor {
	return &MySQLTransactor{db: db, generator: generator}
}

func (t *MySQLTransactor) WithinTx(ctx context.Context, fn func(ctx context.Context, deps SubmitDependencies) error) error {
	tx, err := t.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := fn(ctx, NewMySQLSubmitDependencies(tx, t.generator)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func NewMySQLSubmitDependencies(ext sqlx.ExtContext, generator generator.ReportGenerator) SubmitDependencies {
	return SubmitDependencies{
		Drafts:      NewMySQLDraftRepositoryWithExt(ext),
		Profiles:    profile.NewService(profile.NewMySQLRepositoryWithExt(ext)),
		Assets:      asset.NewService(asset.NewMySQLRepositoryWithExt(ext)),
		Wardrobe:    wardrobe.NewService(wardrobe.NewMySQLRepositoryWithExt(ext)),
		Jobs:        job.NewService(job.NewMySQLRepositoryWithExt(ext)),
		Reports:     report.NewService(report.NewMySQLRepositoryWithExt(ext)),
		ImageRoutes: imageroute.NewService(imageroute.NewMySQLRepositoryWithExt(ext)),
		Generator:   generator,
		Locker:      businesslock.NoopLocker{},
	}
}
