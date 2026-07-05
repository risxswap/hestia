SET NAMES utf8mb4;
SET time_zone = '+00:00';

CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `wechat_openid` varchar(128) DEFAULT NULL,
  `wechat_unionid` varchar(128) DEFAULT NULL,
  `phone` varchar(32) DEFAULT NULL,
  `nickname` varchar(128) DEFAULT NULL,
  `avatar_url` varchar(1024) DEFAULT NULL,
  `onboarding_status` varchar(32) NOT NULL DEFAULT 'not_started',
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `last_active_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_users_public_id` (`public_id`),
  UNIQUE KEY `uk_users_wechat_openid` (`wechat_openid`),
  UNIQUE KEY `uk_users_phone` (`phone`),
  KEY `idx_users_status` (`status`),
  KEY `idx_users_last_active_at` (`last_active_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `profiles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `gender` varchar(32) DEFAULT NULL,
  `height_cm` smallint unsigned DEFAULT NULL,
  `body_notes` text,
  `skin_notes` text,
  `hair_notes` text,
  `lifestyle_scenarios` json DEFAULT NULL,
  `style_goal_summary` text,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_profiles_public_id` (`public_id`),
  UNIQUE KEY `uk_profiles_user_id` (`user_id`),
  KEY `idx_profiles_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `profile_facts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `profile_id` bigint unsigned NOT NULL,
  `fact_key` varchar(128) NOT NULL,
  `fact_value` json NOT NULL,
  `source` varchar(32) NOT NULL,
  `confirmed_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_profile_facts_user_key` (`user_id`, `fact_key`),
  KEY `idx_profile_facts_profile` (`profile_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `profile_inferences` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `profile_id` bigint unsigned NOT NULL,
  `inference_key` varchar(128) NOT NULL,
  `inference_value` json NOT NULL,
  `confidence` decimal(5,4) DEFAULT NULL,
  `source_asset_id` bigint unsigned DEFAULT NULL,
  `source_job_id` bigint unsigned DEFAULT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `confirmed_by_user` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_profile_inferences_user_key` (`user_id`, `inference_key`),
  KEY `idx_profile_inferences_status` (`status`),
  KEY `idx_profile_inferences_source_asset` (`source_asset_id`),
  KEY `idx_profile_inferences_source_job` (`source_job_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `profile_prefs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `profile_id` bigint unsigned NOT NULL,
  `pref_type` varchar(64) NOT NULL,
  `pref_key` varchar(128) NOT NULL,
  `pref_value` json NOT NULL,
  `polarity` varchar(32) NOT NULL,
  `source` varchar(32) NOT NULL,
  `confidence` decimal(5,4) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_profile_prefs_user_type` (`user_id`, `pref_type`),
  KEY `idx_profile_prefs_key` (`pref_key`),
  KEY `idx_profile_prefs_polarity` (`polarity`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `files` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `owner_user_id` bigint unsigned DEFAULT NULL,
  `bucket` varchar(128) NOT NULL,
  `object_key` varchar(512) NOT NULL,
  `mime_type` varchar(128) NOT NULL,
  `file_size` bigint unsigned NOT NULL,
  `width` int unsigned DEFAULT NULL,
  `height` int unsigned DEFAULT NULL,
  `asset_type` varchar(64) NOT NULL,
  `source` varchar(32) NOT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `review_status` varchar(32) NOT NULL DEFAULT 'pending',
  `metadata_json` json DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_files_public_id` (`public_id`),
  UNIQUE KEY `uk_files_bucket_object_key` (`bucket`, `object_key`),
  KEY `idx_files_owner_type` (`owner_user_id`, `asset_type`),
  KEY `idx_files_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `clothes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `name` varchar(128) NOT NULL,
  `category` varchar(64) NOT NULL,
  `color` varchar(128) DEFAULT NULL,
  `silhouette` varchar(128) DEFAULT NULL,
  `material` varchar(128) DEFAULT NULL,
  `thickness` varchar(64) DEFAULT NULL,
  `pattern` varchar(64) DEFAULT NULL,
  `season` varchar(64) DEFAULT NULL,
  `formality` varchar(64) DEFAULT NULL,
  `scene_tags` json DEFAULT NULL,
  `ai_attrs` json DEFAULT NULL,
  `user_notes` text,
  `is_core` tinyint(1) NOT NULL DEFAULT 0,
  `recognition_status` varchar(32) NOT NULL DEFAULT 'succeeded',
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_clothes_public_id` (`public_id`),
  KEY `idx_clothes_user_category` (`user_id`, `category`),
  KEY `idx_clothes_user_status` (`user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `clothes_assets` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `clothes_id` bigint unsigned NOT NULL,
  `asset_id` bigint unsigned NOT NULL,
  `is_primary` tinyint(1) NOT NULL DEFAULT 0,
  `sort_order` int NOT NULL DEFAULT 0,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_clothes_assets_item` (`clothes_id`),
  KEY `idx_clothes_assets_asset` (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `clothes_gaps` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `gap_type` varchar(64) NOT NULL,
  `title` varchar(160) NOT NULL,
  `description` text,
  `reason` text,
  `priority` int NOT NULL DEFAULT 0,
  `source_report_id` bigint unsigned DEFAULT NULL,
  `source_advice_id` bigint unsigned DEFAULT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_clothes_gaps_public_id` (`public_id`),
  KEY `idx_clothes_gaps_user_status` (`user_id`, `status`),
  KEY `idx_clothes_gaps_source_report` (`source_report_id`),
  KEY `idx_clothes_gaps_source_advice` (`source_advice_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `hair` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `name` varchar(128) NOT NULL,
  `length` varchar(64) DEFAULT NULL,
  `shape` varchar(128) DEFAULT NULL,
  `bangs` varchar(128) DEFAULT NULL,
  `color` varchar(128) DEFAULT NULL,
  `care_time` varchar(64) DEFAULT NULL,
  `suitability_notes` text,
  `avoidance_notes` text,
  `user_notes` text,
  `recommendation_status` varchar(32) NOT NULL DEFAULT 'normal',
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_hair_public_id` (`public_id`),
  KEY `idx_hair_user_status` (`user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `hair_assets` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `hair_id` bigint unsigned NOT NULL,
  `asset_id` bigint unsigned NOT NULL,
  `is_primary` tinyint(1) NOT NULL DEFAULT 0,
  `sort_order` int NOT NULL DEFAULT 0,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_hair_assets_hair` (`hair_id`),
  KEY `idx_hair_assets_asset` (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `hair_scene_tags` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `hair_id` bigint unsigned NOT NULL,
  `tag` varchar(64) NOT NULL,
  `sort_order` int NOT NULL DEFAULT 0,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_hair_scene_tags_hair` (`hair_id`),
  KEY `idx_hair_scene_tags_tag` (`tag`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `makeup` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `name` varchar(128) NOT NULL,
  `makeup_type` varchar(64) DEFAULT NULL,
  `focus` varchar(128) DEFAULT NULL,
  `color_palette` varchar(128) DEFAULT NULL,
  `finish` varchar(128) DEFAULT NULL,
  `suitability_notes` text,
  `avoidance_notes` text,
  `user_notes` text,
  `recommendation_status` varchar(32) NOT NULL DEFAULT 'normal',
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_makeup_public_id` (`public_id`),
  KEY `idx_makeup_user_status` (`user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `makeup_assets` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `makeup_id` bigint unsigned NOT NULL,
  `asset_id` bigint unsigned NOT NULL,
  `is_primary` tinyint(1) NOT NULL DEFAULT 0,
  `sort_order` int NOT NULL DEFAULT 0,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_makeup_assets_makeup` (`makeup_id`),
  KEY `idx_makeup_assets_asset` (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `makeup_scene_tags` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `makeup_id` bigint unsigned NOT NULL,
  `tag` varchar(64) NOT NULL,
  `sort_order` int NOT NULL DEFAULT 0,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_makeup_scene_tags_makeup` (`makeup_id`),
  KEY `idx_makeup_scene_tags_tag` (`tag`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `style_subjects` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `name` varchar(128) NOT NULL,
  `subject_type` varchar(64) NOT NULL,
  `gender` varchar(32) DEFAULT NULL,
  `description` text,
  `status` varchar(32) NOT NULL DEFAULT 'draft',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_style_subjects_public_id` (`public_id`),
  KEY `idx_style_subjects_type_status` (`subject_type`, `status`),
  KEY `idx_style_subjects_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `style_samples` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `subject_id` bigint unsigned NOT NULL,
  `name` varchar(160) NOT NULL,
  `style_keywords` json DEFAULT NULL,
  `suitable_scenes` json DEFAULT NULL,
  `clothing_structure` json DEFAULT NULL,
  `color_logic` text,
  `hair_makeup_points` text,
  `transferable_elements` json NOT NULL,
  `non_transferable_risks` json NOT NULL,
  `suitable_profile_notes` text,
  `status` varchar(32) NOT NULL DEFAULT 'draft',
  `published_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_style_samples_public_id` (`public_id`),
  KEY `idx_style_samples_subject_status` (`subject_id`, `status`),
  KEY `idx_style_samples_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `style_tags` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `name` varchar(64) NOT NULL,
  `tag_type` varchar(64) NOT NULL,
  `description` text,
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_style_tags_public_id` (`public_id`),
  UNIQUE KEY `uk_style_tags_type_name` (`tag_type`, `name`),
  KEY `idx_style_tags_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `style_sample_tags` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `style_sample_id` bigint unsigned NOT NULL,
  `style_tag_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_style_sample_tags_sample_tag` (`style_sample_id`, `style_tag_id`),
  KEY `idx_style_sample_tags_tag` (`style_tag_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `style_sample_assets` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `style_sample_id` bigint unsigned NOT NULL,
  `asset_id` bigint unsigned NOT NULL,
  `usage_type` varchar(32) NOT NULL DEFAULT 'reference',
  `sort_order` int NOT NULL DEFAULT 0,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_style_sample_assets_sample` (`style_sample_id`),
  KEY `idx_style_sample_assets_asset` (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `image_routes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `profile_id` bigint unsigned NOT NULL,
  `name` varchar(128) NOT NULL,
  `route_role` varchar(32) NOT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'candidate',
  `weight` decimal(8,4) NOT NULL DEFAULT 0,
  `target_impression` json DEFAULT NULL,
  `suitable_scenes` json DEFAULT NULL,
  `hair_strategy` json DEFAULT NULL,
  `makeup_strategy` json DEFAULT NULL,
  `outfit_strategy` json DEFAULT NULL,
  `avoid_points` json DEFAULT NULL,
  `reason` json DEFAULT NULL,
  `source` varchar(32) NOT NULL,
  `created_from_report_id` bigint unsigned DEFAULT NULL,
  `created_from_job_id` bigint unsigned DEFAULT NULL,
  `activated_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_image_routes_public_id` (`public_id`),
  KEY `idx_image_routes_user_status` (`user_id`, `status`),
  KEY `idx_image_routes_user_role` (`user_id`, `route_role`),
  KEY `idx_image_routes_profile` (`profile_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `image_route_events` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `image_route_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `event_type` varchar(64) NOT NULL,
  `event_value` json DEFAULT NULL,
  `source` varchar(32) NOT NULL,
  `source_feedback_id` bigint unsigned DEFAULT NULL,
  `source_advice_id` bigint unsigned DEFAULT NULL,
  `source_report_id` bigint unsigned DEFAULT NULL,
  `created_by` varchar(32) NOT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_image_route_events_route` (`image_route_id`),
  KEY `idx_image_route_events_user_created` (`user_id`, `created_at`),
  KEY `idx_image_route_events_type` (`event_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `image_route_styles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `image_route_id` bigint unsigned NOT NULL,
  `style_sample_id` bigint unsigned NOT NULL,
  `reason` text,
  `transfer_snapshot` json DEFAULT NULL,
  `risk_snapshot` json DEFAULT NULL,
  `weight` decimal(8,4) NOT NULL DEFAULT 0,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_image_route_styles_route` (`image_route_id`),
  KEY `idx_image_route_styles_sample` (`style_sample_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `reports` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `profile_id` bigint unsigned NOT NULL,
  `report_type` varchar(64) NOT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'generating',
  `title` varchar(180) NOT NULL,
  `summary` text,
  `content_json` json DEFAULT NULL,
  `context_snapshot` json DEFAULT NULL,
  `style_refs_json` json DEFAULT NULL,
  `job_id` bigint unsigned DEFAULT NULL,
  `generated_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_reports_public_id` (`public_id`),
  KEY `idx_reports_user_status` (`user_id`, `status`),
  KEY `idx_reports_profile` (`profile_id`),
  KEY `idx_reports_generated` (`generated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `report_image_routes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `report_id` bigint unsigned NOT NULL,
  `image_route_id` bigint unsigned NOT NULL,
  `route_role` varchar(32) NOT NULL,
  `sort_order` int NOT NULL DEFAULT 0,
  `route_snapshot` json DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_report_image_routes_report` (`report_id`),
  KEY `idx_report_image_routes_route` (`image_route_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `chat_msgs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `role` varchar(32) NOT NULL,
  `msg_type` varchar(32) NOT NULL,
  `content_text` mediumtext,
  `content_json` json DEFAULT NULL,
  `asset_refs` json DEFAULT NULL,
  `related_type` varchar(64) DEFAULT NULL,
  `related_id` bigint unsigned DEFAULT NULL,
  `related_public_id` varchar(32) DEFAULT NULL,
  `job_id` bigint unsigned DEFAULT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'sent',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chat_msgs_public_id` (`public_id`),
  KEY `idx_chat_msgs_user_created` (`user_id`, `created_at`),
  KEY `idx_chat_msgs_related` (`related_type`, `related_id`),
  KEY `idx_chat_msgs_job` (`job_id`),
  KEY `idx_chat_msgs_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `advice_requests` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `source` varchar(64) NOT NULL,
  `source_msg_id` bigint unsigned DEFAULT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'pending',
  `input_text` mediumtext,
  `input_assets` json DEFAULT NULL,
  `scenario` json DEFAULT NULL,
  `trigger_context` json DEFAULT NULL,
  `parent_request_id` bigint unsigned DEFAULT NULL,
  `parent_advice_id` bigint unsigned DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_advice_requests_public_id` (`public_id`),
  KEY `idx_advice_requests_user_status` (`user_id`, `status`),
  KEY `idx_advice_requests_user_created` (`user_id`, `created_at`),
  KEY `idx_advice_requests_source_msg` (`source_msg_id`),
  KEY `idx_advice_requests_parent_advice` (`parent_advice_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `advices` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `advice_request_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `adopted_route_id` bigint unsigned DEFAULT NULL,
  `report_id` bigint unsigned DEFAULT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'generating',
  `advice_date` date NOT NULL,
  `scene_key` varchar(64) NOT NULL,
  `title` varchar(180) NOT NULL,
  `summary` text,
  `outfit_advice` json DEFAULT NULL,
  `hair_advice` json DEFAULT NULL,
  `makeup_advice` json DEFAULT NULL,
  `avoid_notes` json DEFAULT NULL,
  `alternatives` json DEFAULT NULL,
  `wardrobe_item_refs` json DEFAULT NULL,
  `wardrobe_gap_refs` json DEFAULT NULL,
  `context_snapshot` json DEFAULT NULL,
  `job_id` bigint unsigned DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_advices_public_id` (`public_id`),
  KEY `idx_advices_advice_request` (`advice_request_id`),
  KEY `idx_advices_user_status` (`user_id`, `status`),
  KEY `idx_advices_user_date_scene` (`user_id`, `advice_date`, `scene_key`, `status`),
  KEY `idx_advices_adopted_route` (`adopted_route_id`),
  KEY `idx_advices_report` (`report_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `feedbacks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `target_type` varchar(64) NOT NULL,
  `target_id` bigint unsigned NOT NULL,
  `target_public_id` varchar(32) DEFAULT NULL,
  `feedback_type` varchar(64) NOT NULL,
  `feedback_text` mediumtext,
  `feedback_value` json DEFAULT NULL,
  `source` varchar(64) NOT NULL,
  `source_msg_id` bigint unsigned DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_feedbacks_public_id` (`public_id`),
  KEY `idx_feedbacks_user_created` (`user_id`, `created_at`),
  KEY `idx_feedbacks_target` (`target_type`, `target_id`),
  KEY `idx_feedbacks_type` (`feedback_type`),
  KEY `idx_feedbacks_source_msg` (`source_msg_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `onboarding_drafts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'draft',
  `current_step` varchar(64) DEFAULT NULL,
  `draft_data` json DEFAULT NULL,
  `content_hash` varchar(64) NOT NULL,
  `version` int unsigned NOT NULL DEFAULT 1,
  `submitted_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  `active_user_id` bigint unsigned GENERATED ALWAYS AS (IF(`deleted_at` IS NULL, `user_id`, NULL)) STORED,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_onboarding_drafts_public_id` (`public_id`),
  UNIQUE KEY `uk_onboarding_drafts_active_user` (`active_user_id`),
  KEY `idx_onboarding_drafts_user_status` (`user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `memories` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `memory_type` varchar(64) NOT NULL,
  `memory_key` varchar(128) NOT NULL,
  `memory_value` json NOT NULL,
  `polarity` varchar(32) NOT NULL DEFAULT 'neutral',
  `confidence` decimal(5,4) DEFAULT NULL,
  `visibility` varchar(32) NOT NULL DEFAULT 'visible',
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `last_reinforced_at` datetime(3) DEFAULT NULL,
  `user_corrected_at` datetime(3) DEFAULT NULL,
  `correction_note` text,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_memories_public_id` (`public_id`),
  KEY `idx_memories_user_type_status` (`user_id`, `memory_type`, `status`),
  KEY `idx_memories_user_status` (`user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `memory_sources` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `memory_id` bigint unsigned NOT NULL,
  `source_type` varchar(64) NOT NULL,
  `source_id` bigint unsigned DEFAULT NULL,
  `source_public_id` varchar(32) DEFAULT NULL,
  `weight` decimal(8,4) NOT NULL DEFAULT 0,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_memory_sources_memory` (`memory_id`),
  KEY `idx_memory_sources_source` (`source_type`, `source_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `plans` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `plan_code` varchar(64) NOT NULL,
  `name` varchar(128) NOT NULL,
  `billing_period` varchar(32) NOT NULL,
  `price_amount` bigint unsigned NOT NULL DEFAULT 0,
  `currency` char(3) NOT NULL DEFAULT 'CNY',
  `grant_credits` bigint unsigned NOT NULL DEFAULT 0,
  `credit_valid_days` int unsigned DEFAULT NULL,
  `grant_storage_bytes` bigint unsigned NOT NULL DEFAULT 0,
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_plans_public_id` (`public_id`),
  UNIQUE KEY `uk_plans_plan_code` (`plan_code`),
  KEY `idx_plans_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `subs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `plan_id` bigint unsigned NOT NULL,
  `order_id` bigint unsigned DEFAULT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `current_period_start_at` datetime(3) NOT NULL,
  `current_period_end_at` datetime(3) NOT NULL,
  `cancel_at_period_end` tinyint(1) NOT NULL DEFAULT 0,
  `cancelled_at` datetime(3) DEFAULT NULL,
  `grant_snapshot` json DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_subs_public_id` (`public_id`),
  KEY `idx_subs_user_status` (`user_id`, `status`),
  KEY `idx_subs_user_period` (`user_id`, `current_period_end_at`),
  KEY `idx_subs_order` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `orders` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `order_no` varchar(64) NOT NULL,
  `order_type` varchar(64) NOT NULL,
  `amount` bigint unsigned NOT NULL,
  `currency` char(3) NOT NULL DEFAULT 'CNY',
  `pay_channel` varchar(64) NOT NULL,
  `pay_status` varchar(32) NOT NULL DEFAULT 'pending',
  `paid_at` datetime(3) DEFAULT NULL,
  `closed_at` datetime(3) DEFAULT NULL,
  `refund_status` varchar(32) NOT NULL DEFAULT 'none',
  `item_snapshot` json DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_orders_public_id` (`public_id`),
  UNIQUE KEY `uk_orders_order_no` (`order_no`),
  KEY `idx_orders_user_created` (`user_id`, `created_at`),
  KEY `idx_orders_type_status` (`order_type`, `pay_status`),
  KEY `idx_orders_pay_status` (`pay_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `benefits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `benefit_type` varchar(32) NOT NULL,
  `benefit_kind` varchar(32) NOT NULL,
  `source` varchar(64) NOT NULL,
  `source_type` varchar(64) DEFAULT NULL,
  `source_id` bigint unsigned DEFAULT NULL,
  `total_amount` bigint NOT NULL,
  `remaining_amount` bigint NOT NULL,
  `unit` varchar(32) NOT NULL,
  `starts_at` datetime(3) NOT NULL,
  `expires_at` datetime(3) DEFAULT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_benefits_public_id` (`public_id`),
  KEY `idx_benefits_user_type_kind` (`user_id`, `benefit_type`, `benefit_kind`, `status`),
  KEY `idx_benefits_user_expires` (`user_id`, `expires_at`),
  KEY `idx_benefits_source` (`source_type`, `source_id`),
  KEY `idx_benefits_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `benefit_txns` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `benefit_id` bigint unsigned NOT NULL,
  `txn_type` varchar(64) NOT NULL,
  `amount_delta` bigint NOT NULL,
  `balance_after` bigint NOT NULL,
  `target_type` varchar(64) DEFAULT NULL,
  `target_id` bigint unsigned DEFAULT NULL,
  `order_id` bigint unsigned DEFAULT NULL,
  `job_id` bigint unsigned DEFAULT NULL,
  `related_benefit_id` bigint unsigned DEFAULT NULL,
  `reason` varchar(255) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_benefit_txns_public_id` (`public_id`),
  KEY `idx_benefit_txns_user_created` (`user_id`, `created_at`),
  KEY `idx_benefit_txns_benefit` (`benefit_id`),
  KEY `idx_benefit_txns_type` (`txn_type`),
  KEY `idx_benefit_txns_order` (`order_id`),
  KEY `idx_benefit_txns_job` (`job_id`),
  KEY `idx_benefit_txns_related_benefit` (`related_benefit_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `system_configs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `group` varchar(64) NOT NULL,
  `key` varchar(128) NOT NULL,
  `value` json NOT NULL,
  `value_type` varchar(32) NOT NULL,
  `description` text,
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_system_configs_group_key` (`group`, `key`),
  KEY `idx_system_configs_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

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

CREATE TABLE IF NOT EXISTS `jobs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `job_type` varchar(64) NOT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'pending',
  `queue_name` varchar(64) NOT NULL,
  `related_type` varchar(64) DEFAULT NULL,
  `related_id` bigint unsigned DEFAULT NULL,
  `user_id` bigint unsigned DEFAULT NULL,
  `input_summary` json DEFAULT NULL,
  `output_summary` json DEFAULT NULL,
  `error_message` text,
  `retry_count` int unsigned NOT NULL DEFAULT 0,
  `next_retry_at` datetime(3) DEFAULT NULL,
  `started_at` datetime(3) DEFAULT NULL,
  `finished_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_jobs_public_id` (`public_id`),
  KEY `idx_jobs_status_next_retry` (`status`, `next_retry_at`),
  KEY `idx_jobs_type_status` (`job_type`, `status`),
  KEY `idx_jobs_related` (`related_type`, `related_id`),
  KEY `idx_jobs_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
