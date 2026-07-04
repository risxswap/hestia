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
  `provider_id` bigint unsigned NOT NULL,
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
  UNIQUE KEY `uk_llm_models_provider_model` (`provider_id`, `model_code`),
  KEY `idx_llm_models_provider_status` (`provider_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO `system_configs`
  (`group`, `key`, `value`, `value_type`, `description`, `status`)
VALUES
  ('llm.usages', 'wardrobe_image_recognition', CAST('{"provider_code":"qwen","model_code":"qwen-vl-plus","params":{"temperature":0.2,"max_tokens":1200,"response_format":"json_object"},"prompt_version":"v1"}' AS JSON), 'json', '衣服图片识别大模型配置', 'active'),
  ('llm.usages', 'agent_chat', CAST('{"provider_code":"qwen","model_code":"qwen-plus","params":{"temperature":0.7,"max_tokens":2000},"prompt_version":"v1"}' AS JSON), 'json', '聊天大模型配置', 'active'),
  ('llm.usages', 'onboarding_summary', CAST('{"provider_code":"qwen","model_code":"qwen-plus","params":{"temperature":0.3,"max_tokens":1600,"response_format":"json_object"},"prompt_version":"v1"}' AS JSON), 'json', 'onboarding 总结大模型配置', 'active')
ON DUPLICATE KEY UPDATE
  `value` = VALUES(`value`),
  `value_type` = VALUES(`value_type`),
  `description` = VALUES(`description`),
  `status` = VALUES(`status`);
