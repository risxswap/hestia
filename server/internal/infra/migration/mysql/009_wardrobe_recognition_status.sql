SET @hestia_add_clothes_recognition_status_sql = (
  SELECT IF(
    EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'clothes'
        AND column_name = 'recognition_status'
    ),
    'SELECT 1',
    'ALTER TABLE `clothes` ADD COLUMN `recognition_status` varchar(32) NOT NULL DEFAULT ''succeeded'' AFTER `recommendation_status`'
  )
);

PREPARE hestia_add_clothes_recognition_status_stmt FROM @hestia_add_clothes_recognition_status_sql;
EXECUTE hestia_add_clothes_recognition_status_stmt;
DEALLOCATE PREPARE hestia_add_clothes_recognition_status_stmt;
