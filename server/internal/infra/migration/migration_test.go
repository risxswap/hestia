package migration_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

func TestInitialMySQLSchemaMatchesLogicalDesign(t *testing.T) {
	schemaPath := filepath.Join("mysql", "001_init_schema.sql")
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read initial schema %s: %v", schemaPath, err)
	}
	sql := string(raw)

	expectedTables := []string{
		"users",
		"admin_users",
		"profiles",
		"profile_facts",
		"profile_inferences",
		"profile_prefs",
		"assets",
		"wardrobe_items",
		"wardrobe_item_assets",
		"wardrobe_gaps",
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
		"jobs",
	}
	for _, table := range expectedTables {
		if !regexp.MustCompile("(?i)CREATE TABLE IF NOT EXISTS `" + table + "`").MatchString(sql) {
			t.Fatalf("schema should create table %s", table)
		}
	}

	for _, token := range []string{
		"`public_id` varchar(32) NOT NULL",
		"UNIQUE KEY `uk_users_public_id` (`public_id`)",
		"UNIQUE KEY `uk_system_configs_group_key` (`group`, `key`)",
		"KEY `idx_advices_user_date_scene` (`user_id`, `advice_date`, `scene_key`, `status`)",
		"KEY `idx_jobs_status_next_retry` (`status`, `next_retry_at`)",
		"`content_json` json DEFAULT NULL",
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

	if strings.Contains(strings.ToLower(sql), "foreign key") {
		t.Fatal("initial schema should keep relations as indexed ids without database foreign keys")
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
