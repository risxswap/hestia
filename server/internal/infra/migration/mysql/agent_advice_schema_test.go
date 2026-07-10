package mysql_test

import (
	"os"
	"strings"
	"testing"
)

func TestAgentAdviceSectionsSchemaUsesSectionTables(t *testing.T) {
	raw := readInitSchema(t)

	mustContain(t, raw, "CREATE TABLE IF NOT EXISTS `agent_run_steps`")
	mustContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_drafts`")
	mustContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_draft_sections`")
	mustContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_draft_section_versions`")
	mustContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_sections`")
	mustContain(t, raw, "`current_revision_no` int unsigned NOT NULL DEFAULT 1")
	mustContain(t, raw, "`section_type` varchar(32) NOT NULL")
	mustNotContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_requests`")
	mustNotContain(t, raw, "`advice_request_id`")
}

func TestAgentRunStepsSchemaRecordsObservableDecisions(t *testing.T) {
	raw := readInitSchema(t)
	chatMsgs := tableSQL(t, raw, "chat_msgs")
	steps := tableSQL(t, raw, "agent_run_steps")

	mustContain(t, chatMsgs, "`source_msg_id` bigint unsigned DEFAULT NULL")
	mustContain(t, chatMsgs, "KEY `idx_chat_msgs_source_msg` (`source_msg_id`)")
	for _, token := range []string{
		"`assistant_msg_id` bigint unsigned NOT NULL",
		"`step_no` int unsigned NOT NULL",
		"`step_type` varchar(32) NOT NULL",
		"`tool_call_id` varchar(128) DEFAULT NULL",
		"`decision_label` varchar(64) DEFAULT NULL",
		"`input_summary` text",
		"`output_summary` text",
		"UNIQUE KEY `uk_agent_run_steps_assistant_step` (`assistant_msg_id`, `step_no`)",
		"KEY `idx_agent_run_steps_source_msg` (`source_msg_id`)",
		"KEY `idx_agent_run_steps_tool_call` (`assistant_msg_id`, `tool_call_id`)",
	} {
		mustContain(t, steps, token)
	}
}

func TestAgentAdviceSectionsSchemaDefinesDraftSectionKeys(t *testing.T) {
	raw := readInitSchema(t)

	mustContain(t, raw, "UNIQUE KEY `uk_advice_draft_sections_draft_type` (`draft_id`, `section_type`)")
	mustContain(t, raw, "UNIQUE KEY `uk_advice_draft_section_versions_section_version` (`draft_id`, `section_type`, `section_version_no`)")
	mustContain(t, raw, "UNIQUE KEY `uk_advice_draft_section_versions_revision_type` (`draft_id`, `draft_revision_no`, `section_type`)")
	mustContain(t, raw, "UNIQUE KEY `uk_advice_sections_advice_type` (`advice_id`, `section_type`)")
	mustContain(t, raw, "KEY `idx_advice_drafts_user_status_updated` (`user_id`, `status`, `updated_at`)")
	mustContain(t, raw, "KEY `idx_advices_user_target_scene` (`user_id`, `target_date`, `scene_key`, `status`)")
}

func TestAgentAdviceSchemaDefinesConfirmedReferenceTables(t *testing.T) {
	raw := readInitSchema(t)

	mustContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_clothes_refs`")
	mustContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_gap_refs`")
	mustContain(t, raw, "UNIQUE KEY `uk_advice_clothes_refs_public_id` (`public_id`)")
	mustContain(t, raw, "KEY `idx_advice_clothes_refs_advice` (`advice_id`)")
	mustContain(t, raw, "KEY `idx_advice_clothes_refs_clothes` (`user_id`, `clothes_id`, `created_at`)")
	mustContain(t, raw, "UNIQUE KEY `uk_advice_gap_refs_public_id` (`public_id`)")
	mustContain(t, raw, "KEY `idx_advice_gap_refs_advice` (`advice_id`)")
	clothes := tableSQL(t, raw, "clothes")
	mustContain(t, clothes, "`recommendation_status` varchar(32) NOT NULL DEFAULT 'normal'")
}

func TestAgentAdviceContainerDoesNotStoreLargeAdviceJSON(t *testing.T) {
	raw := readInitSchema(t)
	advices := tableSQL(t, raw, "advices")

	mustContain(t, advices, "`source_msg_id` bigint unsigned DEFAULT NULL")
	mustContain(t, advices, "`source_draft_id` bigint unsigned DEFAULT NULL")
	mustContain(t, advices, "`source_draft_revision_no` int unsigned DEFAULT NULL")
	mustContain(t, advices, "`scene_label` varchar(180) DEFAULT NULL")
	mustContain(t, advices, "`target_date` date DEFAULT NULL")
	mustContain(t, advices, "`occasion` varchar(180) DEFAULT NULL")
	mustContain(t, advices, "`weather_text` varchar(255) DEFAULT NULL")
	mustContain(t, advices, "`mood_text` varchar(255) DEFAULT NULL")
	mustContain(t, advices, "`style_goal` text")
	mustContain(t, advices, "`avoid_goal` text")
	mustNotContain(t, advices, "`outfit_advice`")
	mustNotContain(t, advices, "`hair_advice`")
	mustNotContain(t, advices, "`makeup_advice`")
	mustNotContain(t, advices, "`avoid_notes`")
	mustNotContain(t, advices, "`alternatives`")
	mustNotContain(t, advices, "`context_snapshot`")
}

func readInitSchema(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile("001_init_schema.sql")
	if err != nil {
		t.Fatalf("read init schema: %v", err)
	}
	return string(raw)
}

func mustContain(t *testing.T, raw, want string) {
	t.Helper()

	if !strings.Contains(raw, want) {
		t.Fatalf("schema should contain %q", want)
	}
}

func mustNotContain(t *testing.T, raw, forbidden string) {
	t.Helper()

	if strings.Contains(raw, forbidden) {
		t.Fatalf("schema should not contain %q", forbidden)
	}
}

func tableSQL(t *testing.T, raw, tableName string) string {
	t.Helper()

	startMarker := "CREATE TABLE IF NOT EXISTS `" + tableName + "`"
	start := strings.Index(raw, startMarker)
	if start < 0 {
		t.Fatalf("schema should contain table %q", tableName)
	}

	rest := raw[start:]
	endMarker := ") ENGINE=InnoDB"
	end := strings.Index(rest, endMarker)
	if end < 0 {
		t.Fatalf("schema table %q should end with %q", tableName, endMarker)
	}
	return rest[:end]
}
