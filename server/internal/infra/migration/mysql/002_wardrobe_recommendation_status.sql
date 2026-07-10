SET @hestia_add_clothes_recommendation_status_sql = (
  SELECT IF(
    EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'clothes'
        AND column_name = 'recommendation_status'
    ),
    'SELECT 1',
    'ALTER TABLE `clothes` ADD COLUMN `recommendation_status` varchar(32) NOT NULL DEFAULT ''normal'' AFTER `is_core`'
  )
);

PREPARE hestia_add_clothes_recommendation_status_stmt FROM @hestia_add_clothes_recommendation_status_sql;
EXECUTE hestia_add_clothes_recommendation_status_stmt;
DEALLOCATE PREPARE hestia_add_clothes_recommendation_status_stmt;
