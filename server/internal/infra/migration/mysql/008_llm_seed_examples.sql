INSERT INTO `llm_providers`
  (`code`, `name`, `api_base_url`, `token`, `auth_type`, `status`)
VALUES
  ('qwen', '通义千问', 'https://dashscope.aliyuncs.com/compatible-mode/v1', '', 'bearer', 'active')
ON DUPLICATE KEY UPDATE
  `name` = VALUES(`name`),
  `auth_type` = VALUES(`auth_type`),
  `status` = VALUES(`status`);

INSERT INTO `llm_models`
  (`provider_code`, `model_code`, `name`, `caps_json`, `max_input_tokens`, `max_output_tokens`, `status`)
VALUES
  ('qwen', 'qwen-plus', 'Qwen Plus', CAST('["text","json"]' AS JSON), 131072, 8192, 'active'),
  ('qwen', 'qwen-vl-plus', 'Qwen VL Plus', CAST('["text","vision","json"]' AS JSON), 129024, 8192, 'active')
ON DUPLICATE KEY UPDATE
  `name` = VALUES(`name`),
  `caps_json` = VALUES(`caps_json`),
  `max_input_tokens` = VALUES(`max_input_tokens`),
  `max_output_tokens` = VALUES(`max_output_tokens`),
  `status` = VALUES(`status`);
