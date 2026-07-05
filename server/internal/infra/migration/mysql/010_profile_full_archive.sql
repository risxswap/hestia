SET @hestia_add_profiles_weight_kg_sql = (
  SELECT IF(
    NOT EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'profiles'
        AND column_name = 'weight_kg'
    ),
    'ALTER TABLE `profiles` ADD COLUMN `weight_kg` decimal(5,2) NULL AFTER `height_cm`',
    'DO 0'
  )
);

PREPARE hestia_add_profiles_weight_kg_stmt FROM @hestia_add_profiles_weight_kg_sql;
EXECUTE hestia_add_profiles_weight_kg_stmt;
DEALLOCATE PREPARE hestia_add_profiles_weight_kg_stmt;

SET @hestia_add_profiles_face_shape_sql = (
  SELECT IF(
    NOT EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'profiles'
        AND column_name = 'face_shape'
    ),
    'ALTER TABLE `profiles` ADD COLUMN `face_shape` varchar(80) NULL AFTER `hair_notes`',
    'DO 0'
  )
);

PREPARE hestia_add_profiles_face_shape_stmt FROM @hestia_add_profiles_face_shape_sql;
EXECUTE hestia_add_profiles_face_shape_stmt;
DEALLOCATE PREPARE hestia_add_profiles_face_shape_stmt;

SET @hestia_add_profiles_upper_body_notes_sql = (
  SELECT IF(
    NOT EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'profiles'
        AND column_name = 'upper_body_notes'
    ),
    'ALTER TABLE `profiles` ADD COLUMN `upper_body_notes` varchar(220) NULL AFTER `face_shape`',
    'DO 0'
  )
);

PREPARE hestia_add_profiles_upper_body_notes_stmt FROM @hestia_add_profiles_upper_body_notes_sql;
EXECUTE hestia_add_profiles_upper_body_notes_stmt;
DEALLOCATE PREPARE hestia_add_profiles_upper_body_notes_stmt;

SET @hestia_add_profiles_lower_body_notes_sql = (
  SELECT IF(
    NOT EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'profiles'
        AND column_name = 'lower_body_notes'
    ),
    'ALTER TABLE `profiles` ADD COLUMN `lower_body_notes` varchar(220) NULL AFTER `upper_body_notes`',
    'DO 0'
  )
);

PREPARE hestia_add_profiles_lower_body_notes_stmt FROM @hestia_add_profiles_lower_body_notes_sql;
EXECUTE hestia_add_profiles_lower_body_notes_stmt;
DEALLOCATE PREPARE hestia_add_profiles_lower_body_notes_stmt;

SET @hestia_add_profiles_size_notes_sql = (
  SELECT IF(
    NOT EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = DATABASE()
        AND table_name = 'profiles'
        AND column_name = 'size_notes'
    ),
    'ALTER TABLE `profiles` ADD COLUMN `size_notes` varchar(220) NULL AFTER `lower_body_notes`',
    'DO 0'
  )
);

PREPARE hestia_add_profiles_size_notes_stmt FROM @hestia_add_profiles_size_notes_sql;
EXECUTE hestia_add_profiles_size_notes_stmt;
DEALLOCATE PREPARE hestia_add_profiles_size_notes_stmt;

CREATE TABLE IF NOT EXISTS profile_photos (
  id bigint PRIMARY KEY AUTO_INCREMENT,
  public_id varchar(64) NOT NULL UNIQUE,
  user_id bigint NOT NULL,
  profile_id bigint NOT NULL,
  asset_public_id varchar(64) NOT NULL,
  photo_type varchar(32) NOT NULL,
  angle varchar(32) NOT NULL,
  note varchar(220) NULL,
  sort_order int NOT NULL DEFAULT 0,
  status varchar(32) NOT NULL DEFAULT 'active',
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at datetime(3) NULL,
  KEY idx_profile_photos_user_profile (user_id, profile_id, deleted_at),
  KEY idx_profile_photos_asset (asset_public_id),
  KEY idx_profile_photos_type_angle (user_id, photo_type, angle, deleted_at)
);
