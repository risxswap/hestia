CREATE TABLE IF NOT EXISTS `llm_providers` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(64) NOT NULL,
  `name` varchar(128) NOT NULL,
  `api_base_url` varchar(512) NOT NULL,
  `token` varchar(2048) NOT NULL,
  `auth_type` varchar(32) NOT NULL DEFAULT 'bearer',
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_llm_providers_code` (`code`),
  KEY `idx_llm_providers_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `llm_models` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `provider_code` varchar(64) NOT NULL,
  `model_code` varchar(128) NOT NULL,
  `name` varchar(128) NOT NULL,
  `caps_json` json NOT NULL,
  `max_input_tokens` int unsigned DEFAULT NULL,
  `max_output_tokens` int unsigned DEFAULT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_llm_models_provider_model` (`provider_code`, `model_code`),
  KEY `idx_llm_models_provider_status` (`provider_code`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
