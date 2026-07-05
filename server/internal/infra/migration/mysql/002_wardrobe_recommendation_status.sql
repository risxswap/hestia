ALTER TABLE clothes ADD COLUMN recommendation_status varchar(32) NOT NULL DEFAULT 'normal' AFTER is_core;
