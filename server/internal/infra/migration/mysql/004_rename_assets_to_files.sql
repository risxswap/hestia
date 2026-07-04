SET @hestia_copy_assets_to_files_sql = (
  SELECT IF(
    EXISTS (
      SELECT 1
      FROM information_schema.tables
      WHERE table_schema = DATABASE()
        AND table_name = 'assets'
    ),
    'INSERT IGNORE INTO `files`
      (id, public_id, owner_user_id, bucket, object_key, mime_type, file_size, width, height, asset_type, source, status, review_status, metadata_json, created_at, updated_at, deleted_at)
     SELECT id, public_id, owner_user_id, bucket, object_key, mime_type, file_size, width, height, asset_type, source, status, review_status, metadata_json, created_at, updated_at, deleted_at
     FROM `assets`',
    'DO 0'
  )
);

PREPARE hestia_copy_assets_to_files_stmt FROM @hestia_copy_assets_to_files_sql;

EXECUTE hestia_copy_assets_to_files_stmt;

DEALLOCATE PREPARE hestia_copy_assets_to_files_stmt;

SET @hestia_drop_assets_sql = (
  SELECT IF(
    EXISTS (
    SELECT 1
    FROM information_schema.tables
    WHERE table_schema = DATABASE()
        AND table_name = 'assets'
    ),
    'DROP TABLE `assets`',
    'DO 0'
  )
);

PREPARE hestia_drop_assets_stmt FROM @hestia_drop_assets_sql;

EXECUTE hestia_drop_assets_stmt;

DEALLOCATE PREPARE hestia_drop_assets_stmt;
