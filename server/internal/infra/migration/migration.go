package migration

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	"hestia/server/internal/infra/config"
	mysqlinfra "hestia/server/internal/infra/mysql"
)

//go:embed mysql/*.sql
var mysqlSchemaFS embed.FS

type Runner interface {
	Up(ctx context.Context, cfg *config.Config) error
}

type SQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type NoopRunner struct{}

func (NoopRunner) Up(context.Context, *config.Config) error {
	return nil
}

type MySQLRunner struct{}

func NewMySQLRunner() MySQLRunner {
	return MySQLRunner{}
}

func (MySQLRunner) Up(ctx context.Context, cfg *config.Config) error {
	db, err := mysqlinfra.Open(cfg)
	if err != nil {
		return err
	}
	if db == nil {
		return nil
	}
	defer db.Close()

	return ApplyMySQLSchema(ctx, db)
}

func RunOnStartup(ctx context.Context, cfg *config.Config, runner Runner) error {
	return runner.Up(ctx, cfg)
}

func ApplyMySQLSchema(ctx context.Context, exec SQLExecutor) error {
	entries, err := mysqlSchemaFS.ReadDir("mysql")
	if err != nil {
		return fmt.Errorf("read mysql migration dir: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := "mysql/" + entry.Name()
		raw, err := mysqlSchemaFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read mysql migration %s: %w", path, err)
		}
		for _, statement := range splitSQLStatements(string(raw)) {
			if _, err := exec.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("execute mysql migration %s: %w", path, err)
			}
		}
	}

	return nil
}

func splitSQLStatements(sqlText string) []string {
	parts := strings.Split(sqlText, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		statement := strings.TrimSpace(part)
		if statement == "" {
			continue
		}
		statements = append(statements, statement)
	}
	return statements
}
