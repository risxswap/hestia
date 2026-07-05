ALTER TABLE clothes ADD COLUMN recognition_status varchar(32) NOT NULL DEFAULT 'succeeded' AFTER recommendation_status;
