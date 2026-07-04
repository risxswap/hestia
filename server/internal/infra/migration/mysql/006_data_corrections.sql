CREATE TABLE IF NOT EXISTS `data_corrections` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `correction_key` varchar(128) NOT NULL,
  `correction_type` varchar(32) NOT NULL,
  `description` varchar(512) NOT NULL DEFAULT '',
  `checksum` varchar(64) NOT NULL DEFAULT '',
  `status` varchar(32) NOT NULL DEFAULT 'succeeded',
  `error_message` text,
  `executed_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_data_corrections_key` (`correction_key`),
  KEY `idx_data_corrections_status` (`status`, `executed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
