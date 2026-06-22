package mysql

import (
	"hestia/server/internal/infra/config"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func Open(cfg *config.Config) (*sqlx.DB, error) {
	if cfg == nil || cfg.DatabaseDSN == "" {
		return nil, nil
	}
	return sqlx.Open("mysql", cfg.DatabaseDSN)
}
