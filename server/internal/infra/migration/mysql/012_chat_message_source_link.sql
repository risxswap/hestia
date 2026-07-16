SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'chat_msgs'
      AND column_name = 'source_msg_id'
  ),
  'ALTER TABLE `chat_msgs` ADD COLUMN `source_msg_id` bigint unsigned DEFAULT NULL AFTER `user_id`',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql := IF(
  NOT EXISTS (
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'chat_msgs'
      AND index_name = 'idx_chat_msgs_source_msg'
  ),
  'ALTER TABLE `chat_msgs` ADD KEY `idx_chat_msgs_source_msg` (`source_msg_id`)',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
