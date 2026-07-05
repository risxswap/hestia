package migration

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"

	"hestia/server/internal/infra/config"
)

func TestRunOnStartupCallsRunnerUp(t *testing.T) {
	runner := &fakeRunner{}
	cfg := &config.Config{DatabaseDSN: "mysql://example"}

	if err := RunOnStartup(context.Background(), cfg, runner); err != nil {
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

	err := RunOnStartup(context.Background(), &config.Config{}, runner)
	if !errors.Is(err, expected) {
		t.Fatalf("expected runner error, got %v", err)
	}
}

func TestApplyMySQLSchemaExecutesInitialSchemaStatements(t *testing.T) {
	exec := &fakeSQLExecutor{}

	if err := ApplyMySQLSchema(context.Background(), exec); err != nil {
		t.Fatalf("apply mysql schema: %v", err)
	}

	if len(exec.queries) < 30 {
		t.Fatalf("expected initial schema statements to be executed, got %d", len(exec.queries))
	}
	if exec.queries[0] != "SET NAMES utf8mb4" {
		t.Fatalf("expected first statement to set charset, got %q", exec.queries[0])
	}
	if !containsStatement(exec.queries, "CREATE TABLE IF NOT EXISTS `users`") {
		t.Fatalf("expected users table creation statement")
	}
	if !containsStatement(exec.queries, "CREATE TABLE IF NOT EXISTS `jobs`") {
		t.Fatalf("expected jobs table creation statement")
	}
	if !containsStatement(exec.queries, "ALTER TABLE clothes ADD COLUMN recommendation_status") {
		t.Fatalf("expected wardrobe recommendation status migration statement")
	}
	if !containsStatement(exec.queries, "clothes.item_options") {
		t.Fatalf("expected wardrobe item options system config migration statement")
	}
	if !containsStatement(exec.queries, "INSERT IGNORE INTO `system_configs`") {
		t.Fatalf("expected system config initialization to preserve existing rows")
	}
	if containsStatement(exec.queries, "ON DUPLICATE KEY UPDATE\n  `value` = VALUES(`value`)") {
		t.Fatalf("system config migrations must not overwrite initialized config rows")
	}
	if !containsStatement(exec.queries, "INSERT IGNORE INTO `files`") {
		t.Fatalf("expected assets-to-files data copy migration statement")
	}
	if !containsStatement(exec.queries, "DROP TABLE `assets`") {
		t.Fatalf("expected old assets table drop migration statement")
	}
	if !containsStatement(exec.queries, "CREATE TABLE IF NOT EXISTS `llm_providers`") {
		t.Fatalf("expected llm providers table migration statement")
	}
	if !containsStatement(exec.queries, "CREATE TABLE IF NOT EXISTS `llm_models`") {
		t.Fatalf("expected llm models table migration statement")
	}
	if !containsStatement(exec.queries, "CREATE TABLE IF NOT EXISTS `data_corrections`") {
		t.Fatalf("expected data corrections table migration statement")
	}
	if !containsStatement(exec.queries, "caps_json") {
		t.Fatalf("expected llm model caps_json field")
	}
	if !containsStatement(exec.queries, "`provider_code` varchar(64) NOT NULL") {
		t.Fatalf("expected llm models to use provider_code field")
	}
	if !containsStatement(exec.queries, "DROP COLUMN `provider_id`") {
		t.Fatalf("expected llm provider_id to provider_code migration statement")
	}
	if !containsStatement(exec.queries, "INSERT INTO `llm_providers`") || !containsStatement(exec.queries, "'qwen'") {
		t.Fatalf("expected llm provider example config migration statement")
	}
	if !containsStatement(exec.queries, "INSERT INTO `llm_models`") ||
		!containsStatement(exec.queries, "'qwen-plus'") ||
		!containsStatement(exec.queries, "'qwen-vl-plus'") {
		t.Fatalf("expected llm model example config migration statement")
	}
	if containsStatement(exec.queries, "INSERT INTO `system_configs`\n  (`group`, `key`, `value`, `value_type`, `description`, `status`)\nVALUES\n  ('llm.usages'") {
		t.Fatalf("llm usage config must be read from database, not seeded with defaults")
	}
	if containsStatement(exec.queries, "`api_base_url` = VALUES(`api_base_url`)") ||
		containsStatement(exec.queries, "`token` = VALUES(`token`)") {
		t.Fatalf("llm provider seed must not overwrite configured endpoint or api key")
	}
	if statementIndex(exec.queries, "INSERT INTO `llm_models`") < statementIndex(exec.queries, "DROP COLUMN `provider_id`") {
		t.Fatalf("expected llm model examples to run after provider_code schema migration")
	}
	for _, key := range []string{"categories", "colors", "materials", "seasons", "silhouettes"} {
		if !containsStatement(exec.queries, "`key`, `value`, `value_type`, `description`, `status`)") || !containsStatement(exec.queries, key) {
			t.Fatalf("expected wardrobe item option config key %s", key)
		}
	}
	for _, query := range exec.queries {
		if strings.TrimSpace(query) == "" {
			t.Fatal("expected no empty SQL statements to be executed")
		}
	}
}

func TestApplyMySQLSchemaIgnoresDuplicateColumnForIncrementalAddColumn(t *testing.T) {
	exec := &duplicateColumnSQLExecutor{}

	if err := ApplyMySQLSchema(context.Background(), exec); err != nil {
		t.Fatalf("expected duplicate column migration to be ignored, got %v", err)
	}

	if !exec.sawRecommendationStatusMigration {
		t.Fatalf("expected wardrobe recommendation status migration to be executed")
	}
}

func TestDuplicateColumnIgnoreOnlyAppliesToWardrobeRecommendationStatusMigration(t *testing.T) {
	duplicateErr := &mysql.MySQLError{Number: 1060, Message: "Duplicate column name 'recommendation_status'"}

	if !isIgnorableDuplicateAddColumn("ALTER TABLE clothes ADD COLUMN recommendation_status varchar(32) NOT NULL DEFAULT 'normal' AFTER is_core", duplicateErr) {
		t.Fatal("expected wardrobe recommendation_status duplicate add column to be ignored")
	}
	if isIgnorableDuplicateAddColumn("ALTER TABLE users ADD COLUMN recommendation_status varchar(32) NOT NULL DEFAULT 'normal'", duplicateErr) {
		t.Fatal("expected unrelated duplicate add column to return an error")
	}
}

func TestInitialMySQLSchemaMatchesLogicalDesign(t *testing.T) {
	schemaPath := filepath.Join("mysql", "001_init_schema.sql")
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read initial schema %s: %v", schemaPath, err)
	}
	sql := string(raw)

	expectedTables := []string{
		"users",
		"profiles",
		"profile_facts",
		"profile_inferences",
		"profile_prefs",
		"files",
		"clothes",
		"clothes_assets",
		"clothes_gaps",
		"style_subjects",
		"style_samples",
		"style_tags",
		"style_sample_tags",
		"style_sample_assets",
		"image_routes",
		"image_route_events",
		"image_route_styles",
		"reports",
		"report_image_routes",
		"chat_msgs",
		"advice_requests",
		"advices",
		"feedbacks",
		"onboarding_drafts",
		"memories",
		"memory_sources",
		"plans",
		"subs",
		"orders",
		"benefits",
		"benefit_txns",
		"system_configs",
		"llm_providers",
		"llm_models",
		"data_corrections",
		"jobs",
	}
	for _, table := range expectedTables {
		if !regexp.MustCompile("(?i)CREATE TABLE IF NOT EXISTS `" + table + "`").MatchString(sql) {
			t.Fatalf("schema should create table %s", table)
		}
	}
	if regexp.MustCompile("(?i)CREATE TABLE IF NOT EXISTS `assets`").MatchString(sql) {
		t.Fatal("schema should create files table instead of assets table")
	}

	for _, token := range []string{
		"`public_id` varchar(32) NOT NULL",
		"UNIQUE KEY `uk_users_public_id` (`public_id`)",
		"UNIQUE KEY `uk_files_public_id` (`public_id`)",
		"UNIQUE KEY `uk_files_bucket_object_key` (`bucket`, `object_key`)",
		"UNIQUE KEY `uk_system_configs_group_key` (`group`, `key`)",
		"KEY `idx_advices_user_date_scene` (`user_id`, `advice_date`, `scene_key`, `status`)",
		"KEY `idx_jobs_status_next_retry` (`status`, `next_retry_at`)",
		"`content_json` json DEFAULT NULL",
		"`metadata_json` json DEFAULT NULL",
		"`content_hash` varchar(64) NOT NULL",
		"`version` int unsigned NOT NULL DEFAULT 1",
		"`draft_data` json DEFAULT NULL",
		"`active_user_id` bigint unsigned GENERATED ALWAYS AS (IF(`deleted_at` IS NULL, `user_id`, NULL)) STORED",
		"UNIQUE KEY `uk_onboarding_drafts_active_user` (`active_user_id`)",
		"`memory_value` json NOT NULL",
		"ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
	} {
		if !strings.Contains(sql, token) {
			t.Fatalf("schema missing required token: %s", token)
		}
	}
	if strings.Contains(sql, "`provider_id` bigint unsigned NOT NULL") {
		t.Fatal("llm_models should use provider_code instead of provider_id")
	}

	for _, token := range []string{
		"ad" + "min_users",
		"created_by_" + "ad" + "min_id",
		"updated_by_" + "ad" + "min_id",
	} {
		if strings.Contains(sql, token) {
			t.Fatalf("schema should not contain removed schema token: %s", token)
		}
	}

	if strings.Contains(strings.ToLower(sql), "foreign key") {
		t.Fatal("initial schema should keep relations as indexed ids without database foreign keys")
	}
}

func TestMySQLMigrationsAvoidProcedureBodies(t *testing.T) {
	err := filepath.WalkDir("mysql", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(string(raw)), "create procedure") {
			t.Fatalf("%s uses CREATE PROCEDURE, but the migration runner splits SQL by semicolon", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan mysql migrations: %v", err)
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

type fakeSQLExecutor struct {
	queries []string
}

func (f *fakeSQLExecutor) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	f.queries = append(f.queries, query)
	return nil, nil
}

func (f *fakeSQLExecutor) GetContext(_ context.Context, _ any, query string, _ ...any) error {
	f.queries = append(f.queries, query)
	return sql.ErrNoRows
}

type duplicateColumnSQLExecutor struct {
	sawRecommendationStatusMigration bool
}

func (f *duplicateColumnSQLExecutor) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	if strings.Contains(query, "ALTER TABLE clothes ADD COLUMN recommendation_status") {
		f.sawRecommendationStatusMigration = true
		return nil, &mysql.MySQLError{Number: 1060, Message: "Duplicate column name 'recommendation_status'"}
	}
	return nil, nil
}

func (f *duplicateColumnSQLExecutor) GetContext(_ context.Context, _ any, _ string, _ ...any) error {
	return sql.ErrNoRows
}

func containsStatement(queries []string, token string) bool {
	for _, query := range queries {
		if strings.Contains(query, token) {
			return true
		}
	}
	return false
}

func statementIndex(queries []string, token string) int {
	for i, query := range queries {
		if strings.Contains(query, token) {
			return i
		}
	}
	return -1
}
