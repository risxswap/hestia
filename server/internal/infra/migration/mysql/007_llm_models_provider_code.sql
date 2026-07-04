SET @hestia_add_llm_models_provider_code_sql = (
  SELECT IF(
    EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'llm_models'
        AND column_name = 'provider_id'
    )
    AND NOT EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'llm_models'
        AND column_name = 'provider_code'
    ),
    'ALTER TABLE `llm_models` ADD COLUMN `provider_code` varchar(64) NOT NULL DEFAULT '''' AFTER `id`',
    'DO 0'
  )
);

PREPARE hestia_add_llm_models_provider_code_stmt FROM @hestia_add_llm_models_provider_code_sql;

EXECUTE hestia_add_llm_models_provider_code_stmt;

DEALLOCATE PREPARE hestia_add_llm_models_provider_code_stmt;

SET @hestia_backfill_llm_models_provider_code_sql = (
  SELECT IF(
    EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'llm_models'
        AND column_name = 'provider_id'
    )
    AND EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'llm_models'
        AND column_name = 'provider_code'
    ),
    'UPDATE `llm_models` m
     JOIN `llm_providers` p ON p.id = m.provider_id
     SET m.provider_code = p.code
     WHERE m.provider_code = ''''',
    'DO 0'
  )
);

PREPARE hestia_backfill_llm_models_provider_code_stmt FROM @hestia_backfill_llm_models_provider_code_sql;

EXECUTE hestia_backfill_llm_models_provider_code_stmt;

DEALLOCATE PREPARE hestia_backfill_llm_models_provider_code_stmt;

SET @hestia_drop_llm_models_provider_index_sql = (
  SELECT IF(
    EXISTS (
      SELECT 1
      FROM information_schema.statistics
      WHERE table_schema = DATABASE()
        AND table_name = 'llm_models'
        AND index_name = 'uk_llm_models_provider_model'
        AND column_name = 'provider_id'
    ),
    'ALTER TABLE `llm_models` DROP INDEX `uk_llm_models_provider_model`',
    'DO 0'
  )
);

PREPARE hestia_drop_llm_models_provider_index_stmt FROM @hestia_drop_llm_models_provider_index_sql;

EXECUTE hestia_drop_llm_models_provider_index_stmt;

DEALLOCATE PREPARE hestia_drop_llm_models_provider_index_stmt;

SET @hestia_add_llm_models_provider_code_unique_sql = (
  SELECT IF(
    NOT EXISTS (
      SELECT 1
      FROM information_schema.statistics
      WHERE table_schema = DATABASE()
        AND table_name = 'llm_models'
        AND index_name = 'uk_llm_models_provider_model'
    ),
    'ALTER TABLE `llm_models` ADD UNIQUE KEY `uk_llm_models_provider_model` (`provider_code`, `model_code`)',
    'DO 0'
  )
);

PREPARE hestia_add_llm_models_provider_code_unique_stmt FROM @hestia_add_llm_models_provider_code_unique_sql;

EXECUTE hestia_add_llm_models_provider_code_unique_stmt;

DEALLOCATE PREPARE hestia_add_llm_models_provider_code_unique_stmt;

SET @hestia_drop_llm_models_provider_status_index_sql = (
  SELECT IF(
    EXISTS (
      SELECT 1
      FROM information_schema.statistics
      WHERE table_schema = DATABASE()
        AND table_name = 'llm_models'
        AND index_name = 'idx_llm_models_provider_status'
        AND column_name = 'provider_id'
    ),
    'ALTER TABLE `llm_models` DROP INDEX `idx_llm_models_provider_status`',
    'DO 0'
  )
);

PREPARE hestia_drop_llm_models_provider_status_index_stmt FROM @hestia_drop_llm_models_provider_status_index_sql;

EXECUTE hestia_drop_llm_models_provider_status_index_stmt;

DEALLOCATE PREPARE hestia_drop_llm_models_provider_status_index_stmt;

SET @hestia_add_llm_models_provider_code_status_index_sql = (
  SELECT IF(
    NOT EXISTS (
      SELECT 1
      FROM information_schema.statistics
      WHERE table_schema = DATABASE()
        AND table_name = 'llm_models'
        AND index_name = 'idx_llm_models_provider_status'
    ),
    'ALTER TABLE `llm_models` ADD KEY `idx_llm_models_provider_status` (`provider_code`, `status`)',
    'DO 0'
  )
);

PREPARE hestia_add_llm_models_provider_code_status_index_stmt FROM @hestia_add_llm_models_provider_code_status_index_sql;

EXECUTE hestia_add_llm_models_provider_code_status_index_stmt;

DEALLOCATE PREPARE hestia_add_llm_models_provider_code_status_index_stmt;

SET @hestia_drop_llm_models_provider_id_sql = (
  SELECT IF(
    EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'llm_models'
        AND column_name = 'provider_id'
    ),
    'ALTER TABLE `llm_models` DROP COLUMN `provider_id`',
    'DO 0'
  )
);

PREPARE hestia_drop_llm_models_provider_id_stmt FROM @hestia_drop_llm_models_provider_id_sql;

EXECUTE hestia_drop_llm_models_provider_id_stmt;

DEALLOCATE PREPARE hestia_drop_llm_models_provider_id_stmt;
