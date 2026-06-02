# 数据库逻辑设计

日期：2026-06-03

## 目标

本文档定义个人 AI 形象顾问 Agent 第一版 MySQL 数据库逻辑设计。

本设计覆盖：

- 账号与管理。
- 用户画像。
- 资产与衣橱。
- 风格库。
- 报告与建议。
- 反馈与记忆。
- AI 与任务。
- 分享与指标。

本文档是逻辑表设计，不是最终 DDL。后续进入实现阶段时，再基于本文档编写迁移文件。

## 总体策略

### 建模策略

数据库采用：

```text
结构化核心实体 + JSON 快照
```

结构化存储：

- 用户。
- 资产。
- 衣橱。
- 风格库。
- 报告和建议主关系。
- 反馈。
- 长期记忆。
- AI 配置。
- 任务。
- 管理端操作日志。

JSON 快照存储：

- 报告正文。
- 推荐生成上下文。
- AI 输入输出摘要。
- 画像和衣橱生成快照。
- 风格库引用摘要。

这样可以兼顾运营查询、隐私删除、关系约束和 AI 输出灵活性。

### ID 策略

核心表使用：

- 内部主键：`id BIGINT`。
- 外部暴露 ID：`public_id`。

原则：

- 表之间内部关联优先使用 `*_id BIGINT`。
- 前端、分享链接、外部 API 暴露 `public_id`。
- `public_id` 可使用 ULID、NanoID 或类似短 ID。
- 分享 token、上传 token、会话 token 使用安全随机值，不使用自增 ID。

### 会话策略

用户会话不存 MySQL。

- 用户 session 存 Redis。
- 管理端 session 也优先存 Redis。
- MySQL 只存用户账号、管理员账号和必要审计记录。

### 软删除策略

敏感和核心业务表使用 `deleted_at` 软删除。

适合软删除：

- `users`
- `user_profiles`
- `profile_facts`
- `profile_inferences`
- `user_preferences`
- `assets`
- `wardrobe_items`
- `wardrobe_gaps`
- `reports`
- `recommendation_requests`
- `recommendations`
- `recommendation_options`
- `feedbacks`
- `memory_entries`

不做普通软删除：

- `jobs`
- `ai_call_logs`
- `admin_operation_logs`
- `metrics_events`

审计和排障表不普通软删，但隐私删除后不能保留敏感原文、完整图片地址或可逆的个人敏感内容。

## 账号与管理

### users

用途：终端用户主账号。

关键字段：

- `id`
- `public_id`
- `wechat_openid`
- `wechat_unionid`
- `phone`
- `nickname`
- `avatar_asset_id`
- `onboarding_status`
- `status`
- `last_active_at`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_users_public_id`
- `uk_users_wechat_openid`
- `uk_users_phone`
- `idx_users_status`
- `idx_users_last_active_at`

说明：

- 小程序微信登录直接落到 `users`。
- 第一版不建 `user_sessions`，会话放 Redis。
- 第一版不建 `user_auth_bindings`，微信 openid、unionid 和可选手机号直接放 `users`。
- `phone` 允许为空；如果启用手机号登录，需要唯一索引。
- `avatar_asset_id` 关联 `assets.id`，可为空。

### admin_users

用途：管理端账号。

关键字段：

- `id`
- `public_id`
- `username`
- `password_hash`
- `display_name`
- `is_super_admin`
- `status`
- `last_login_at`
- `created_at`
- `updated_at`

索引建议：

- `uk_admin_users_public_id`
- `uk_admin_users_username`
- `idx_admin_users_status`

说明：

- 第一版不建 `admin_roles`。
- 用 `is_super_admin` 和 `status` 控制最小管理能力。
- 如后续需要 Operator/Viewer 等角色，再扩展字段或拆权限表。

### admin_operation_logs

用途：管理端操作审计。

关键字段：

- `id`
- `admin_user_id`
- `module`
- `action`
- `target_type`
- `target_id`
- `target_public_id`
- `summary`
- `ip`
- `user_agent`
- `created_at`

索引建议：

- `idx_admin_logs_admin_created`
- `idx_admin_logs_target`
- `idx_admin_logs_module_action`

说明：

- 不普通软删。
- 不保存敏感原文和完整图片地址。
- 风格库发布、AI 配置发布/回滚、任务重试、用户数据查看都要记录。

## 用户画像

### user_profiles

用途：用户画像主表，存相对稳定的个人形象档案摘要。

关键字段：

- `id`
- `user_id`
- `profile_status`
- `gender`
- `height_cm`
- `body_notes`
- `skin_tone_notes`
- `hair_notes`
- `lifestyle_scenarios`
- `style_goal_summary`
- `profile_summary`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_user_profiles_user_id`
- `idx_user_profiles_status`

JSON 字段：

- `lifestyle_scenarios`
- `profile_summary`

说明：

- 第一版一个用户只保留一份当前画像。
- 不把 AI 推断和用户事实混在一起，详细内容放 `profile_facts` 和 `profile_inferences`。

### profile_facts

用途：用户明确提供或确认过的事实。

关键字段：

- `id`
- `user_id`
- `profile_id`
- `fact_key`
- `fact_value`
- `source`
- `confirmed_at`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `idx_profile_facts_user_key`
- `idx_profile_facts_profile`

JSON 字段：

- `fact_value`

说明：

- 存用户明确说的内容，例如身高、常见场景、不喜欢太甜。
- `source` 可为 `onboarding`、`chat`、`feedback`、`admin_note`。

### profile_inferences

用途：AI 推断的画像信息，带置信度和可修正状态。

关键字段：

- `id`
- `user_id`
- `profile_id`
- `inference_key`
- `inference_value`
- `confidence`
- `source_asset_id`
- `source_job_id`
- `status`
- `confirmed_by_user`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `idx_profile_inferences_user_key`
- `idx_profile_inferences_status`
- `idx_profile_inferences_source_asset`
- `idx_profile_inferences_source_job`

JSON 字段：

- `inference_value`

说明：

- 存脸型倾向、肤色倾向、体型比例线索、发量发质等 AI 推断。
- `status` 可为 `active`、`rejected`、`superseded`。
- 用户否定后标记 `rejected`，避免反复推同样结论。

### user_preferences

用途：用户偏好和禁忌的结构化记录。

关键字段：

- `id`
- `user_id`
- `preference_type`
- `preference_key`
- `preference_value`
- `polarity`
- `source`
- `confidence`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `idx_user_preferences_user_type`
- `idx_user_preferences_key`
- `idx_user_preferences_polarity`

JSON 字段：

- `preference_value`

说明：

- `polarity` 可为 `like`、`dislike`、`avoid`、`prefer`。
- 用于记录风格、颜色、版型、成熟度、露肤度、场景偏好等。
- 真实反馈沉淀出的偏好置信度应高于初始 onboarding。

## 资产与衣橱

### assets

用途：统一存储七牛云资源元数据。

关键字段：

- `id`
- `public_id`
- `owner_user_id`
- `bucket`
- `object_key`
- `mime_type`
- `file_size`
- `width`
- `height`
- `asset_type`
- `source`
- `status`
- `review_status`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_assets_public_id`
- `uk_assets_bucket_object_key`
- `idx_assets_owner_type`
- `idx_assets_status`

说明：

- `asset_type` 可为 `selfie`、`body_photo`、`wardrobe_item`、`reference_image`、`style_sample`、`avatar`。
- 用户资产用 `owner_user_id`，管理端风格样本图可以为空或用系统归属。
- 删除资产时先软删，再创建七牛云资源清理任务。
- 业务表不要直接存七牛云完整 URL。

### asset_links

用途：通用资源关联表。

关键字段：

- `id`
- `asset_id`
- `target_type`
- `target_id`
- `usage_type`
- `sort_order`
- `created_at`

索引建议：

- `idx_asset_links_asset`
- `idx_asset_links_target`

说明：

- 适合自拍关联画像、参考图关联风格解析等通用关系。
- 强关系可以用专表，例如 `wardrobe_item_assets`、`style_sample_assets`。

### wardrobe_items

用途：用户核心衣橱单品。

关键字段：

- `id`
- `public_id`
- `user_id`
- `name`
- `category`
- `color`
- `silhouette`
- `material`
- `thickness`
- `pattern`
- `season`
- `formality`
- `scene_tags`
- `ai_attributes`
- `user_notes`
- `is_core`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_wardrobe_items_public_id`
- `idx_wardrobe_items_user_category`
- `idx_wardrobe_items_user_status`

JSON 字段：

- `scene_tags`
- `ai_attributes`

说明：

- 第一版只管理核心单品，不做完整库存。
- AI 识别结果和用户修正先合并在该表字段中。

### wardrobe_item_assets

用途：衣橱单品和图片的明确关联。

关键字段：

- `id`
- `wardrobe_item_id`
- `asset_id`
- `is_primary`
- `sort_order`
- `created_at`

索引建议：

- `idx_wardrobe_item_assets_item`
- `idx_wardrobe_item_assets_asset`

说明：

- 一个单品可有多张图。
- 主图用 `is_primary` 标记。

### wardrobe_gaps

用途：记录系统识别出的衣橱缺口。

关键字段：

- `id`
- `public_id`
- `user_id`
- `gap_type`
- `title`
- `description`
- `reason`
- `priority`
- `source_report_id`
- `source_recommendation_id`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_wardrobe_gaps_public_id`
- `idx_wardrobe_gaps_user_status`
- `idx_wardrobe_gaps_source_report`
- `idx_wardrobe_gaps_source_recommendation`

说明：

- 只记录品类和特征，不记录商品链接。
- `status` 可为 `active`、`dismissed`、`resolved`。

## 风格库

### style_reference_subjects

用途：明星、博主、内部参考对象或用户参考对象主表。

关键字段：

- `id`
- `public_id`
- `name`
- `subject_type`
- `gender`
- `description`
- `status`
- `created_by_admin_id`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_style_subjects_public_id`
- `idx_style_subjects_type_status`
- `idx_style_subjects_name`

说明：

- `subject_type` 可为 `celebrity`、`blogger`、`internal`、`user_reference`。
- 第一版不自动抓取公开图片。
- 这里是“参考对象”，不是“相似明星”。

### style_samples

用途：参考对象下的一条结构化风格样本。

关键字段：

- `id`
- `public_id`
- `subject_id`
- `name`
- `style_keywords`
- `suitable_scenarios`
- `clothing_structure`
- `color_logic`
- `hair_makeup_points`
- `transferable_elements`
- `non_transferable_risks`
- `suitable_profile_notes`
- `status`
- `published_at`
- `created_by_admin_id`
- `updated_by_admin_id`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_style_samples_public_id`
- `idx_style_samples_subject_status`
- `idx_style_samples_status`

JSON 字段：

- `style_keywords`
- `suitable_scenarios`
- `clothing_structure`
- `transferable_elements`
- `non_transferable_risks`

说明：

- `status` 可为 `draft`、`pending_publish`、`published`、`offline`。
- 发布前必须有可迁移元素和不可迁移风险。
- 不存“用户像某明星”字段。

### style_tags

用途：风格标签字典。

关键字段：

- `id`
- `public_id`
- `name`
- `tag_type`
- `description`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_style_tags_public_id`
- `uk_style_tags_type_name`
- `idx_style_tags_status`

说明：

- `tag_type` 可为 `style`、`scene`、`color`、`silhouette`、`hair`。

### style_sample_tags

用途：风格样本和标签的多对多关系。

关键字段：

- `id`
- `style_sample_id`
- `style_tag_id`
- `created_at`

索引建议：

- `uk_style_sample_tags_sample_tag`
- `idx_style_sample_tags_tag`

### style_sample_assets

用途：风格样本和图片资源关联。

关键字段：

- `id`
- `style_sample_id`
- `asset_id`
- `usage_type`
- `sort_order`
- `created_at`

索引建议：

- `idx_style_sample_assets_sample`
- `idx_style_sample_assets_asset`

说明：

- 可关联管理端上传的样本图或参考图。
- 若图片版权或授权边界不清，前端不应直接展示给用户端。

## 报告与建议

### reports

用途：用户个人形象报告主表。

关键字段：

- `id`
- `public_id`
- `user_id`
- `profile_id`
- `current_version_id`
- `report_type`
- `status`
- `title`
- `summary`
- `generated_at`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_reports_public_id`
- `idx_reports_user_status`
- `idx_reports_profile`

说明：

- `status` 可为 `generating`、`ready`、`failed`、`expired`。
- `current_version_id` 指向当前展示版本。
- 报告正文放 `report_versions`。

### report_versions

用途：报告版本和正文快照。

关键字段：

- `id`
- `public_id`
- `report_id`
- `version_no`
- `content_json`
- `context_snapshot`
- `style_sample_refs`
- `ai_call_id`
- `job_id`
- `created_at`

索引建议：

- `uk_report_versions_public_id`
- `uk_report_versions_report_version`
- `idx_report_versions_report`

JSON 字段：

- `content_json`
- `context_snapshot`
- `style_sample_refs`

说明：

- `content_json` 存行动清单、风格路线、发型方向、穿搭原则、缺口建议、避雷项等。
- `context_snapshot` 存生成时使用的画像、衣橱、风格库摘要。
- `style_sample_refs` 存引用的风格样本和引用理由。

### recommendation_requests

用途：用户发起的建议请求。

关键字段：

- `id`
- `public_id`
- `user_id`
- `source`
- `scenario`
- `user_goal`
- `input_text`
- `input_assets`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_recommendation_requests_public_id`
- `idx_recommendation_requests_user_status`
- `idx_recommendation_requests_created`

JSON 字段：

- `input_assets`

说明：

- `source` 可为 `miniapp`、`user_web`。第一版 `user_web` 不发起新建议，但保留枚举空间。
- `status` 可为 `pending`、`processing`、`completed`、`failed`。

### recommendations

用途：一次建议生成结果主表。

关键字段：

- `id`
- `public_id`
- `request_id`
- `user_id`
- `report_id`
- `status`
- `scenario_summary`
- `context_snapshot`
- `ai_call_id`
- `job_id`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_recommendations_public_id`
- `idx_recommendations_request`
- `idx_recommendations_user_status`
- `idx_recommendations_report`

JSON 字段：

- `context_snapshot`

说明：

- 一次 request 对应一条 recommendation。
- 具体方案放 `recommendation_options`。

### recommendation_options

用途：建议结果中的单个方案。

关键字段：

- `id`
- `public_id`
- `recommendation_id`
- `option_label`
- `outfit_items`
- `hairstyle_advice`
- `reason`
- `avoid_notes`
- `alternative_items`
- `wardrobe_gap_refs`
- `sort_order`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_recommendation_options_public_id`
- `idx_recommendation_options_recommendation`

JSON 字段：

- `outfit_items`
- `alternative_items`
- `wardrobe_gap_refs`

说明：

- 每个 option 都可以被反馈。
- 不直接存商品链接。

## 反馈与记忆

### feedbacks

用途：用户对报告、建议、单品、发型、风格路线等对象的反馈。

关键字段：

- `id`
- `public_id`
- `user_id`
- `target_type`
- `target_id`
- `target_public_id`
- `feedback_type`
- `feedback_text`
- `feedback_value`
- `source`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_feedbacks_public_id`
- `idx_feedbacks_user_created`
- `idx_feedbacks_target`
- `idx_feedbacks_type`

JSON 字段：

- `feedback_value`

说明：

- `target_type` 可为 `report`、`report_action`、`recommendation`、`recommendation_option`、`wardrobe_item`、`style_route`、`hairstyle`。
- `feedback_type` 可为 `accepted`、`rejected`、`modified`、`worn_good`、`worn_bad`、`external_positive`、`external_negative`。
- 真实反馈是后续记忆更新的高优先级来源。

### memory_entries

用途：系统沉淀出的长期记忆条目。

关键字段：

- `id`
- `public_id`
- `user_id`
- `memory_type`
- `memory_key`
- `memory_value`
- `polarity`
- `confidence`
- `status`
- `last_reinforced_at`
- `created_at`
- `updated_at`
- `deleted_at`

索引建议：

- `uk_memory_entries_public_id`
- `idx_memory_entries_user_type`
- `idx_memory_entries_status`

JSON 字段：

- `memory_value`

说明：

- `memory_type` 可为 `style_preference`、`scene_rule`、`color_preference`、`silhouette_preference`、`hair_preference`、`avoidance`。
- `polarity` 可为 `like`、`dislike`、`prefer`、`avoid`。
- `status` 可为 `active`、`rejected`、`superseded`。

### memory_sources

用途：记忆条目和来源反馈、建议、报告之间的关系。

关键字段：

- `id`
- `memory_entry_id`
- `source_type`
- `source_id`
- `source_public_id`
- `weight`
- `created_at`

索引建议：

- `idx_memory_sources_entry`
- `idx_memory_sources_source`

说明：

- 一个记忆可以来自多次反馈。
- 真实穿着反馈权重大于初始 onboarding。
- 用于解释“为什么系统记住了这件事”。

## AI 与任务

### ai_configs

用途：AI 配置主表。

关键字段：

- `id`
- `public_id`
- `config_type`
- `name`
- `description`
- `current_version_id`
- `status`
- `created_by_admin_id`
- `created_at`
- `updated_at`

索引建议：

- `uk_ai_configs_public_id`
- `idx_ai_configs_type_status`

说明：

- `config_type` 可为 `prompt`、`report_template`、`recommendation_rule`、`model_config`。
- 当前线上版本通过 `current_version_id` 指向 `ai_config_versions`。

### ai_config_versions

用途：AI 配置版本。

关键字段：

- `id`
- `public_id`
- `config_id`
- `version_no`
- `content`
- `test_cases`
- `status`
- `published_at`
- `published_by_admin_id`
- `created_by_admin_id`
- `created_at`
- `updated_at`

索引建议：

- `uk_ai_config_versions_public_id`
- `uk_ai_config_versions_config_version`
- `idx_ai_config_versions_status`

JSON 字段：

- `content`
- `test_cases`

说明：

- `status` 可为 `draft`、`testing`、`published`、`offline`。
- 已发布版本不可直接编辑，只能复制为新草稿。

### ai_call_logs

用途：AI 调用审计和成本记录。

关键字段：

- `id`
- `public_id`
- `user_id`
- `job_id`
- `config_version_id`
- `provider`
- `model`
- `call_type`
- `input_summary`
- `output_summary`
- `token_usage`
- `cost_amount`
- `latency_ms`
- `status`
- `error_message`
- `created_at`

索引建议：

- `uk_ai_call_logs_public_id`
- `idx_ai_call_logs_user_created`
- `idx_ai_call_logs_job`
- `idx_ai_call_logs_config_version`
- `idx_ai_call_logs_status`

JSON 字段：

- `input_summary`
- `output_summary`
- `token_usage`

说明：

- 不保存完整敏感原文和图片地址。
- 不普通软删，作为审计和排障记录。

### jobs

用途：异步任务持久化记录。

关键字段：

- `id`
- `public_id`
- `job_type`
- `status`
- `queue_name`
- `related_type`
- `related_id`
- `user_id`
- `input_summary`
- `output_summary`
- `error_message`
- `retry_count`
- `next_retry_at`
- `started_at`
- `finished_at`
- `created_at`
- `updated_at`

索引建议：

- `uk_jobs_public_id`
- `idx_jobs_status_next_retry`
- `idx_jobs_type_status`
- `idx_jobs_related`
- `idx_jobs_user`

JSON 字段：

- `input_summary`
- `output_summary`

说明：

- Redis 负责队列和锁，MySQL 记录状态。
- 不普通软删，作为排障记录。
- 隐私删除后，摘要字段不能保留敏感原文。

## 分享与指标

### share_tokens

用途：报告或建议分享页的访问 token。

关键字段：

- `id`
- `public_id`
- `token`
- `owner_user_id`
- `target_type`
- `target_id`
- `target_public_id`
- `scope`
- `expires_at`
- `revoked_at`
- `created_at`
- `updated_at`

索引建议：

- `uk_share_tokens_public_id`
- `uk_share_tokens_token`
- `idx_share_tokens_owner`
- `idx_share_tokens_target`
- `idx_share_tokens_expires`

JSON 字段：

- `scope`

说明：

- `token` 必须是安全随机值。
- `target_type` 可为 `report`、`recommendation`、`recommendation_option`。
- 第一版分享页只展示脱敏摘要。

### metrics_events

用途：基础埋点事件。

关键字段：

- `id`
- `user_id`
- `event_name`
- `event_source`
- `target_type`
- `target_id`
- `properties`
- `occurred_at`
- `created_at`

索引建议：

- `idx_metrics_events_user_time`
- `idx_metrics_events_name_time`
- `idx_metrics_events_target`

JSON 字段：

- `properties`

说明：

- 第一版只存必要产品事件，不做复杂 BI。
- 高敏感内容不要写入 `properties`。

## 明确不建的表

第一版不建：

- `user_sessions`
- `user_auth_bindings`
- `admin_roles`
- `user_delete_requests`

原因：

- 用户会话放 Redis。
- 第一版身份来源简单，微信身份和手机号直接放 `users`。
- 管理端权限保持极简。
- 第一版不做删除请求流程表。

## 关键约束

- 不在业务表中直接保存七牛云完整 URL。
- 不在 AI 调用日志中保存完整敏感输入输出。
- 不把 AI 推断和用户确认事实混存在同一张明细表。
- 不存“用户像某明星”这类字段。
- 用户反馈和长期记忆必须可追溯来源。
- 报告和推荐必须保存生成上下文快照，便于复盘。
- Redis 不能作为关键业务数据的唯一存储。

## MVP 验收标准

数据库逻辑设计满足第一版开发条件，当且仅当：

- 核心实体使用 BIGINT 内部 ID 和 `public_id`。
- 用户 session 不进入 MySQL。
- 账号域不包含 `user_auth_bindings` 和 `admin_roles`。
- 敏感和核心业务表支持软删除。
- 任务、AI 调用、管理端操作日志用于审计和排障，不普通软删。
- 用户画像区分事实、AI 推断和偏好。
- 资产统一由 `assets` 管理七牛云元数据。
- 衣橱只建核心单品和缺口，不做完整库存。
- 风格库支持可迁移元素和不可迁移风险。
- 报告与建议保存 JSON 快照。
- 反馈和记忆支持来源追溯。
- 分享 token 只暴露脱敏范围。
