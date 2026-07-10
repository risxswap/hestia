SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'chat_msgs'
      AND column_name = 'asset_refs'
  ),
  'ALTER TABLE `chat_msgs` ADD COLUMN `asset_refs` json DEFAULT NULL AFTER `content_json`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'agent_run_steps'
      AND column_name = 'usage_key'
  ),
  'ALTER TABLE `agent_run_steps` ADD COLUMN `usage_key` varchar(64) DEFAULT NULL AFTER `status`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'agent_run_steps'
      AND column_name = 'provider_code'
  ),
  'ALTER TABLE `agent_run_steps` ADD COLUMN `provider_code` varchar(64) DEFAULT NULL AFTER `usage_key`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'agent_run_steps'
      AND column_name = 'model_code'
  ),
  'ALTER TABLE `agent_run_steps` ADD COLUMN `model_code` varchar(128) DEFAULT NULL AFTER `provider_code`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'agent_run_steps'
      AND column_name = 'prompt_version'
  ),
  'ALTER TABLE `agent_run_steps` ADD COLUMN `prompt_version` varchar(64) DEFAULT NULL AFTER `model_code`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'agent_run_steps'
      AND column_name = 'max_iterations'
  ),
  'ALTER TABLE `agent_run_steps` ADD COLUMN `max_iterations` int unsigned DEFAULT NULL AFTER `prompt_version`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'agent_run_steps'
      AND column_name = 'tool_name'
  ),
  'ALTER TABLE `agent_run_steps` ADD COLUMN `tool_name` varchar(128) DEFAULT NULL AFTER `max_iterations`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'agent_run_steps'
      AND column_name = 'tool_call_id'
  ),
  'ALTER TABLE `agent_run_steps` ADD COLUMN `tool_call_id` varchar(128) DEFAULT NULL AFTER `tool_name`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'agent_run_steps'
      AND column_name = 'duration_ms'
  ),
  'ALTER TABLE `agent_run_steps` ADD COLUMN `duration_ms` int unsigned DEFAULT NULL AFTER `finished_at`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'agent_run_steps'
      AND index_name = 'idx_agent_run_steps_tool_call'
  ),
  'ALTER TABLE `agent_run_steps` ADD KEY `idx_agent_run_steps_tool_call` (`assistant_msg_id`, `tool_call_id`)',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
